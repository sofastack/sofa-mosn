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
	"time"

	"github.com/golang/mock/gomock"
	"mosn.io/api"
	"mosn.io/mosn/pkg/metrics"
	"mosn.io/mosn/pkg/mock"
	"mosn.io/mosn/pkg/types"
	"mosn.io/pkg/buffer"
)

// gomock func wrapper
func gomockClusterInfo(ctrl *gomock.Controller) types.ClusterInfo {
	info := mock.NewMockClusterInfo(ctrl)
	info.EXPECT().Stats().DoAndReturn(func() types.ClusterStats {
		s := metrics.NewClusterStats("mockcluster")
		return types.ClusterStats{
			UpstreamRequestDuration:      s.Histogram(metrics.UpstreamRequestDuration),
			UpstreamRequestDurationTotal: s.Counter(metrics.UpstreamRequestDurationTotal),
			UpstreamResponseSuccess:      s.Counter(metrics.UpstreamResponseSuccess),
			UpstreamResponseFailed:       s.Counter(metrics.UpstreamResponseFailed),
		}
	}).AnyTimes()
	info.EXPECT().ResourceManager().DoAndReturn(func() types.ResourceManager {
		mng := mock.NewMockResourceManager(ctrl)
		mng.EXPECT().Retries().DoAndReturn(func() types.Resource {
			r := mock.NewMockResource(ctrl)
			r.EXPECT().Increase().AnyTimes()
			r.EXPECT().Decrease().AnyTimes()
			return r
		}).AnyTimes()
		return mng
	}).AnyTimes()
	return info
}

type gomockReceiverFilterWrapper struct {
	*mock.MockStreamReceiverFilter
	handler api.StreamReceiverFilterHandler
}

func (f *gomockReceiverFilterWrapper) SetReceiveFilterHandler(handler api.StreamReceiverFilterHandler) {
	f.handler = handler
}

func gomockReceiverFilter(ctrl *gomock.Controller, receive func(api.StreamReceiverFilterHandler, context.Context, api.HeaderMap, buffer.IoBuffer, api.HeaderMap)) api.StreamReceiverFilter {
	filter := mock.NewMockStreamReceiverFilter(ctrl)
	filter.EXPECT().OnDestroy().AnyTimes()
	fw := &gomockReceiverFilterWrapper{
		MockStreamReceiverFilter: filter,
	}
	filter.EXPECT().
		OnReceive(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, headers api.HeaderMap, data buffer.IoBuffer, trailers api.HeaderMap) api.StreamFilterStatus {
			if receive != nil {
				receive(fw.handler, ctx, headers, data, trailers)
			}
			return api.StreamFilterContinue
		}).AnyTimes()
	return fw
}

func gomockStreamSender(ctrl *gomock.Controller) types.StreamSender {
	encoder := mock.NewMockStreamSender(ctrl)
	encoder.EXPECT().GetStream().DoAndReturn(func() types.Stream {
		s := mock.NewMockStream(ctrl)
		s.EXPECT().AddEventListener(gomock.Any()).AnyTimes()
		s.EXPECT().ResetStream(gomock.Any()).AnyTimes()
		return s
	})
	encoder.EXPECT().AppendHeaders(gomock.Any(), gomock.Any(), gomock.Any())
	encoder.EXPECT().AppendData(gomock.Any(), gomock.Any(), gomock.Any())
	encoder.EXPECT().AppendTrailers(gomock.Any(), gomock.Any())
	return encoder

}

//  mock a simple route rule that route a request to a cluster
func gomockRouteMatchCluster(ctrl *gomock.Controller, cluster_name string) api.Route {
	r := mock.NewMockRoute(ctrl)
	// mock route rule returns cluster name : mock_cluster
	r.EXPECT().RouteRule().DoAndReturn(func() api.RouteRule {
		rule := mock.NewMockRouteRule(ctrl)
		rule.EXPECT().ClusterName(gomock.Any()).Return(cluster_name).AnyTimes()
		rule.EXPECT().UpstreamProtocol().Return("").AnyTimes()
		rule.EXPECT().GlobalTimeout().Return(300 * time.Second).AnyTimes()
		rule.EXPECT().Policy().DoAndReturn(func() api.Policy {
			p := mock.NewMockPolicy(ctrl)
			p.EXPECT().RetryPolicy().DoAndReturn(func() api.RetryPolicy {
				rp := mock.NewMockRetryPolicy(ctrl)
				rp.EXPECT().RetryOn().Return(false).AnyTimes()
				rp.EXPECT().TryTimeout().Return(time.Duration(0)).AnyTimes()
				rp.EXPECT().NumRetries().Return(uint32(3)).AnyTimes()
				return rp
			}).AnyTimes()
			return p
		}).AnyTimes()
		rule.EXPECT().FinalizeRequestHeaders(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		rule.EXPECT().FinalizeResponseHeaders(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		return rule
	}).AnyTimes()
	r.EXPECT().DirectResponseRule().Return(nil)
	r.EXPECT().RedirectRule().Return(nil)
	return r
}
