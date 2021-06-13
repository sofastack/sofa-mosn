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

package mixer

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogo/protobuf/jsonpb"
	"mosn.io/api"
	"mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/istio/control/http"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/mixer/v1/config/client"
	"mosn.io/pkg/buffer"
)

func init() {
	// static mixer stream filter factory
	api.RegisterStream(v2.MIXER, CreateMixerFilterFactory)
}

// FilterConfigFactory filter config factory
type FilterConfigFactory struct {
	MixerConfig *v2.Mixer
}

type mixerFilter struct {
	context               context.Context
	config                *v2.Mixer
	serviceContext        *http.ServiceContext
	clientContext         *http.ClientContext
	requestHandler        http.RequestHandler
	receiverFilterHandler api.StreamReceiverFilterHandler
	requestTotalSize      uint64
}

// newMixerFilter used to create new mixer filter
func newMixerFilter(context context.Context, config *v2.Mixer) *mixerFilter {
	filter := &mixerFilter{
		context:       context,
		config:        config,
		clientContext: http.NewClientContext(config),
	}
	filter.serviceContext = http.NewServiceContext(filter.clientContext)
	return filter
}

func (f *mixerFilter) ReadPerRouteConfig(perFilterConfig map[string]interface{}) {
	mixerConfig, exist := perFilterConfig[v2.MIXER]
	if !exist {
		return
	}

	serviceConfig, ok := mixerConfig.(client.ServiceConfig)
	if !ok {
		return
	}

	f.serviceContext.SetServiceConfig(&serviceConfig)
}

func (f *mixerFilter) createRequestHandler() {
	if f.requestHandler != nil {
		log.DefaultLogger.Tracef("requestHandler not nil, return")
		return
	}

	route := f.receiverFilterHandler.Route()
	if route == nil {
		log.DefaultLogger.Tracef("no route, return")
		return
	}
	rule := route.RouteRule()
	if rule == nil {
		log.DefaultLogger.Tracef("no route rule, return")
		return
	}

	perFilterConfig := rule.PerFilterConfig()

	if perFilterConfig != nil {
		f.ReadPerRouteConfig(perFilterConfig)
	}

	f.requestHandler = http.NewRequestHandler(f.serviceContext)
}

func (f *mixerFilter) OnReceive(ctx context.Context, headers api.HeaderMap, buf buffer.IoBuffer, trailers api.HeaderMap) api.StreamFilterStatus {
	if headers != nil {
		f.requestTotalSize += headers.ByteSize()
		f.createRequestHandler()
	}
	if buf != nil {
		f.requestTotalSize += uint64(buf.Len())
	}
	if trailers != nil {
		f.requestTotalSize += trailers.ByteSize()
	}

	return api.StreamFilterContinue
}

func (f *mixerFilter) SetReceiveFilterHandler(handler api.StreamReceiverFilterHandler) {
	f.receiverFilterHandler = handler
}

func (f *mixerFilter) OnDestroy() {}

func (f *mixerFilter) Log(ctx context.Context, reqHeaders api.HeaderMap, respHeaders api.HeaderMap, requestInfo api.RequestInfo) {
	if reqHeaders == nil || respHeaders == nil || requestInfo == nil {
		return
	}

	f.createRequestHandler()

	// TODO: use f.receiverFilterHandler.Connection() to get address instead of requestInfo
	checkData := http.NewCheckData(reqHeaders, requestInfo)

	reportData := http.NewReportData(respHeaders, requestInfo, f.requestTotalSize)

	f.requestHandler.Report(checkData, reportData)
}

// CreateFilterChain for create mixer filter
func (f *FilterConfigFactory) CreateFilterChain(context context.Context, callbacks api.StreamFilterChainFactoryCallbacks) {
	filter := newMixerFilter(context, f.MixerConfig)
	callbacks.AddStreamReceiverFilter(filter, api.AfterRoute)
	callbacks.AddStreamAccessLog(filter)
}

// CreateMixerFilterFactory for create mixer filter factory
func CreateMixerFilterFactory(conf map[string]interface{}) (api.StreamFilterChainFactory, error) {
	m, err := ParseMixerFilter(conf)
	if err != nil {
		return nil, err
	}
	return &FilterConfigFactory{
		MixerConfig: m,
	}, nil
}

// ParseMixerFilter
func ParseMixerFilter(cfg map[string]interface{}) (*v2.Mixer, error) {
	mixerFilter := &v2.Mixer{}

	data, err := json.Marshal(cfg)
	if err != nil {
		log.DefaultLogger.Errorf("[mixer] parsing mixer filter error, err: %v, cfg: %v", err, cfg)
		return nil, err
	}

	var un jsonpb.Unmarshaler
	err = un.Unmarshal(strings.NewReader(string(data)), &mixerFilter.HttpClientConfig)
	if err != nil {
		log.DefaultLogger.Errorf("[mixer] parsing mixer filter error, err: %v, cfg: %v", err, cfg)
		return nil, err
	}

	// default configuration
	if mixerFilter.Transport == nil {
		mixerFilter.Transport = &client.TransportConfig{
			ReportCluster: "mixer_server",
		}
	}

	return mixerFilter, nil
}
