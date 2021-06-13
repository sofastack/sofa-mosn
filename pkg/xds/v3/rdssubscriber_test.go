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

	envoy_config_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_service_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
)

func Test_RdsHandler(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("TestRdsHandler error: %v \n %s", r, string(debug.Stack()))
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
	route := &envoy_config_route_v3.RouteConfiguration{
		Name: "testroute",
	}

	routeAny, _ := ptypes.MarshalAny(route)
	resp := &envoy_service_discovery_v3.DiscoveryResponse{
		TypeUrl:   EnvoyRoute,
		Resources: []*any.Any{routeAny},
	}

	if rds := adsClient.handleRoutesResp(resp); rds == nil || len(rds) != 1 {
		t.Error("handleRoutesResp failed.")
	}
}
