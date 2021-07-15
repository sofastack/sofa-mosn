/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package proxy

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"mosn.io/api"
	v2 "mosn.io/mosn/pkg/config/v2"
	mosnctx "mosn.io/mosn/pkg/context"
	"mosn.io/mosn/pkg/mock"
	"mosn.io/mosn/pkg/network"
	"mosn.io/mosn/pkg/protocol"
	"mosn.io/mosn/pkg/router"
	"mosn.io/mosn/pkg/trace"
	"mosn.io/mosn/pkg/types"
	"mosn.io/mosn/pkg/variable"
	"mosn.io/pkg/buffer"
)

func TestDownstream_FinishTracing_NotEnable(t *testing.T) {
	ds := downStream{context: context.Background()}
	ds.finishTracing()
	span := trace.SpanFromContext(context.Background())
	if span != nil {
		t.Error("Span is not nil")
	}
}

func TestDownstream_FinishTracing_Enable(t *testing.T) {
	trace.Enable()
	ds := downStream{context: context.Background()}
	ds.finishTracing()
	span := trace.SpanFromContext(context.Background())
	if span != nil {
		t.Error("Span is not nil")
	}
}

func TestDownstream_FinishTracing_Enable_SpanIsNotNil(t *testing.T) {
	trace.Enable()
	err := trace.Init("SOFATracer", nil)
	if err != nil {
		t.Error("init tracing driver failed: ", err)
	}

	span := trace.Tracer(mockProtocol).Start(context.Background(), nil, time.Now())
	ctx := mosnctx.WithValue(context.Background(), types.ContextKeyActiveSpan, span)
	requestInfo := &network.RequestInfo{}
	ds := downStream{context: ctx, requestInfo: requestInfo}
	header := protocol.CommonHeader{}
	span.InjectContext(header, requestInfo)
	ds.finishTracing()

	span = trace.SpanFromContext(ctx)
	if span == nil {
		t.Error("Span is nil")
	}
	mockSpan := span.(*mockSpan)
	if v, _ := header.Get("test-inject"); v != "mock" {
		t.Error("Span is not inject")
	}
	if !mockSpan.inject {
		t.Error("Span is not inject")
	}
	if !mockSpan.finished {
		t.Error("Span is not finish")
	}
}

func TestDirectResponse(t *testing.T) {
	testCases := []struct {
		client *mockResponseSender
		route  *mockRoute
		check  func(t *testing.T, ctx context.Context, sender *mockResponseSender)
	}{
		// without body
		{
			client: &mockResponseSender{},
			route: &mockRoute{
				direct: &mockDirectRule{
					status: 500,
				},
			},
			check: func(t *testing.T, ctx context.Context, client *mockResponseSender) {
				if client.headers == nil {
					t.Fatal("want to receive a header response")
				}
				if code, err := variable.GetString(ctx, types.VarHeaderStatus); err != nil || code != "500" {
					t.Error("response status code not expected")
				}
			},
		},
		// with body
		{
			client: &mockResponseSender{},
			route: &mockRoute{
				direct: &mockDirectRule{
					status: 400,
					body:   "mock 400 response",
				},
			},
			check: func(t *testing.T, ctx context.Context, client *mockResponseSender) {
				if client.headers == nil {
					t.Fatal("want to receive a header response")
				}
				if code, err := variable.GetString(ctx, types.VarHeaderStatus); err != nil || code != "400" {
					t.Error("response status code not expected")
				}
				if client.data == nil {
					t.Fatal("want to receive a body response")
				}
				if client.data.String() != "mock 400 response" {
					t.Error("response  data not expected")
				}
			},
		},
	}
	for _, tc := range testCases {
		ctx := variable.NewVariableContext(context.Background())
		s := &downStream{
			proxy: &proxy{
				config: &v2.Proxy{},
				routersWrapper: &mockRouterWrapper{
					routers: &mockRouters{
						route: tc.route,
					},
				},
				clusterManager:      &mockClusterManager{},
				readCallbacks:       &mockReadFilterCallbacks{},
				stats:               globalStats,
				listenerStats:       newListenerStats("test"),
				serverStreamConn:    &mockServerConn{},
				routeHandlerFactory: router.DefaultMakeHandler,
			},
			responseSender: tc.client,
			requestInfo:    &network.RequestInfo{},
			context:        ctx,
		}
		s.initStreamFilterChain()
		// event call Receive Headers
		// trigger direct response
		s.OnReceive(ctx, protocol.CommonHeader{}, buffer.NewIoBuffer(1), nil)
		// check
		time.Sleep(100 * time.Millisecond)
		tc.check(t, ctx, tc.client)
	}
}

func TestSetDownstreamRouter(t *testing.T) {
	s := &downStream{
		context: context.Background(),
		proxy: &proxy{
			config: &v2.Proxy{},
			routersWrapper: &mockRouterWrapper{
				routers: &mockRouters{
					route: &mockRoute{},
				},
			},
			clusterManager:      &mockClusterManager{},
			readCallbacks:       &mockReadFilterCallbacks{},
			stats:               globalStats,
			listenerStats:       newListenerStats("test"),
			serverStreamConn:    &mockServerConn{},
			routeHandlerFactory: router.DefaultMakeHandler,
		},
		responseSender: &mockResponseSender{},
		requestInfo:    &network.RequestInfo{},
		snapshot:       &mockClusterSnapshot{},
	}
	s.initStreamFilterChain()
	s.matchRoute()
	assert.NotNilf(t, s.DownstreamRoute(),
		"downstream router in context should not be nil")
}

func TestOnewayHijack(t *testing.T) {
	initGlobalStats()
	proxy := &proxy{
		config:              &v2.Proxy{},
		routersWrapper:      nil,
		clusterManager:      &mockClusterManager{},
		readCallbacks:       &mockReadFilterCallbacks{},
		stats:               globalStats,
		listenerStats:       newListenerStats("test"),
		serverStreamConn:    &mockServerConn{},
		routeHandlerFactory: router.DefaultMakeHandler,
	}
	s := newActiveStream(context.Background(), proxy, nil, nil)

	// not routes, sendHijack
	s.OnReceive(context.Background(), protocol.CommonHeader{}, buffer.NewIoBuffer(1), nil)
	// check
	time.Sleep(100 * time.Millisecond)
	if s.downstreamCleaned != 1 {
		t.Errorf("downStream should be cleaned")
	}
}

func TestIsRequestFailed(t *testing.T) {
	testCases := []struct {
		Flags    []api.ResponseFlag
		Expected bool
	}{
		{
			Flags:    []api.ResponseFlag{api.NoHealthyUpstream},
			Expected: true,
		},
		{
			Flags:    []api.ResponseFlag{api.UpstreamRequestTimeout},
			Expected: false,
		},
		{
			Flags:    []api.ResponseFlag{api.UpstreamRemoteReset},
			Expected: false,
		},
		{
			Flags:    []api.ResponseFlag{api.NoRouteFound},
			Expected: true,
		},
		{
			Flags:    []api.ResponseFlag{api.DelayInjected},
			Expected: false,
		},
		{
			Flags:    []api.ResponseFlag{api.FaultInjected},
			Expected: true,
		},
		{
			Flags:    []api.ResponseFlag{api.RateLimited},
			Expected: true,
		},
		{
			Flags:    []api.ResponseFlag{api.DelayInjected, api.FaultInjected},
			Expected: true,
		},
		{
			Flags:    []api.ResponseFlag{api.UpstreamRequestTimeout, api.NoHealthyUpstream},
			Expected: true,
		},
		{
			Flags:    []api.ResponseFlag{api.UpstreamConnectionTermination, api.UpstreamRemoteReset},
			Expected: false,
		},
	}
	for idx, tc := range testCases {
		s := &downStream{
			requestInfo: network.NewRequestInfo(),
		}
		for _, f := range tc.Flags {
			s.requestInfo.SetResponseFlag(f)
		}
		if s.isRequestFailed() != tc.Expected {
			t.Errorf("case no.%d is not expected, flag: %v, expected: %v", idx, tc.Flags, tc.Expected)
		}
	}
}

func TestProcessError(t *testing.T) {
	var s *downStream
	var p types.Phase
	var e error

	s = &downStream{}
	p, e = s.processError(0)
	if p != types.End || e != nil {
		t.Errorf("TestprocessError Error")
	}

	s = &downStream{}
	s.oneway = true
	p, e = s.processError(0)
	if p != types.End || e != nil {
		t.Errorf("TestprocessError Error")
	}

	s = &downStream{}
	p, e = s.processError(1)
	if p != types.End || e != types.ErrExit {
		t.Errorf("TestprocessError Error")
	}

	s = &downStream{}
	s.downstreamCleaned = 1
	p, e = s.processError(0)
	if p != types.End || e != types.ErrExit {
		t.Errorf("TestprocessError Error")
	}

	s = &downStream{}
	s.upstreamReset = 1
	s.oneway = true
	p, e = s.processError(0)
	if p != types.Oneway || e != types.ErrExit {
		t.Errorf("TestprocessError Error")
	}

	s = &downStream{}
	s.directResponse = true
	p, e = s.processError(0)
	if p != types.UpFilter || e != types.ErrExit {
		t.Errorf("TestprocessError Error")
	}

	s = &downStream{}
	s.directResponse = true
	s.oneway = true
	p, e = s.processError(0)
	if p != types.Oneway || e != types.ErrExit {
		t.Errorf("TestprocessError Error")
	}

	s = &downStream{}
	s.upstreamRequest = &upstreamRequest{}
	s.upstreamRequest.setupRetry = true
	p, e = s.processError(0)
	if p != types.Retry || e != types.ErrExit {
		t.Errorf("TestprocessError Error")
	}

	s = &downStream{}
	s.receiverFiltersAgainPhase = types.MatchRoute
	p, e = s.processError(0)
	if p != types.MatchRoute || e != types.ErrExit {
		t.Errorf("TestprocessError Error")
	}

	s = &downStream{}
	s.receiverFiltersAgainPhase = types.ChooseHost
	p, e = s.processError(0)
	if p != types.ChooseHost || e != types.ErrExit {
		t.Errorf("TestprocessError Error")
	}
}

func TestMetadataMatchCriteria(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := variable.NewVariableContext(context.Background())

	cluster := mock.NewMockClusterInfo(ctrl)
	cluster.EXPECT().Name().AnyTimes()

	routeEntry := mock.NewMockRouteRule(ctrl)
	routeEntryMeta := router.NewMetadataMatchCriteriaImpl(map[string]string{
		"a": "b",
		"b": "bb",
	})
	routeEntry.EXPECT().MetadataMatchCriteria(gomock.Any()).AnyTimes().Return(routeEntryMeta)

	requestInfo := &network.RequestInfo{}
	requestInfo.SetRouteEntry(routeEntry)

	s := &downStream{
		context:     ctx,
		requestInfo: requestInfo,
		cluster:     cluster,
	}

	assert.Equal(t, s.MetadataMatchCriteria(), routeEntryMeta)

	variable.Set(ctx, types.VarRouterMeta, map[string]string{"a": "aa"})
	assert.Equal(t, len(s.MetadataMatchCriteria().MetadataMatchCriteria()), 2)
	assert.Equal(t, s.MetadataMatchCriteria().MetadataMatchCriteria()[0].MetadataValue(), "aa")
}
