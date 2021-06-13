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

package v3

import (
	"runtime/debug"
	"sync"
	"testing"

	envoy_config_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_extensions_filters_network_http_connection_manager_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_service_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	wellknown "github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
)

func Test_LdsHandler(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("TestLdsHandler error: %v \n %s", r, string(debug.Stack()))
		}
	}()

	xdsConfig := XDSConfig{}
	adsClient := &ADSClient{
		AdsConfig:         xdsConfig.ADSConfig,
		StreamClientMutex: sync.RWMutex{},
		StreamClient:      nil,
		SendControlChan:   make(chan int),
		RecvControlChan:   make(chan int),
		StopChan:          make(chan int),
	}

	httpManager := &envoy_extensions_filters_network_http_connection_manager_v3.HttpConnectionManager{
		RouteSpecifier: &envoy_extensions_filters_network_http_connection_manager_v3.HttpConnectionManager_Rds{
			Rds: &envoy_extensions_filters_network_http_connection_manager_v3.Rds{
				RouteConfigName: "test_route",
			},
		},
	}
	httpManagerAny, _ := ptypes.MarshalAny(httpManager)
	filter := envoy_config_listener_v3.Filter{
		Name: wellknown.HTTPConnectionManager,
	}
	filter.ConfigType = &envoy_config_listener_v3.Filter_TypedConfig{TypedConfig: httpManagerAny}
	listener := &envoy_config_listener_v3.Listener{
		Name: "test_listener",
		FilterChains: []*envoy_config_listener_v3.FilterChain{
			{
				Filters: []*envoy_config_listener_v3.Filter{
					&filter,
				},
			},
		},
	}

	listenerRes, _ := ptypes.MarshalAny(listener)
	resp := &envoy_service_discovery_v3.DiscoveryResponse{
		TypeUrl:   EnvoyListener,
		Resources: []*any.Any{listenerRes},
	}

	if lds := adsClient.handleListenersResp(resp); lds == nil || len(lds) != 1 {
		t.Error("handleListenersResp failed.")
	}
}
