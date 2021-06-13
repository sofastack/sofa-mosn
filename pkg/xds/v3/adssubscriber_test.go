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

	envoy_config_bootstrap_v3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	envoy_config_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_config_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"
	"mosn.io/mosn/pkg/upstream/cluster"
	"mosn.io/mosn/pkg/xds/v3/conv"
)

func Test_Client(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("TestxDSClient error: %v \n %s", r, string(debug.Stack()))
		}
	}()

	xdsConfig := XDSConfig{}
	clusterName := "xds-cluster"
	xdsAddr := "127.0.0.1"
	xdsPort := 15010
	dynamicResources := &envoy_config_bootstrap_v3.Bootstrap_DynamicResources{
		LdsConfig: configSource(clusterName),
		CdsConfig: configSource(clusterName),
		AdsConfig: configApiSource(clusterName),
	}
	staticResources := &envoy_config_bootstrap_v3.Bootstrap_StaticResources{
		Clusters: []*envoy_config_cluster_v3.Cluster{{
			Name:                 clusterName,
			ConnectTimeout:       &duration.Duration{Seconds: 5},
			ClusterDiscoveryType: &envoy_config_cluster_v3.Cluster_Type{Type: envoy_config_cluster_v3.Cluster_STRICT_DNS},
			LbPolicy:             envoy_config_cluster_v3.Cluster_ROUND_ROBIN,
			LoadAssignment: &envoy_config_endpoint_v3.ClusterLoadAssignment{
				ClusterName: clusterName,
				Endpoints: []*envoy_config_endpoint_v3.LocalityLbEndpoints{
					localityLbEndpoints(xdsAddr, xdsPort),
				},
			},
			UpstreamConnectionOptions: &envoy_config_cluster_v3.UpstreamConnectionOptions{
				TcpKeepalive: &envoy_config_core_v3.TcpKeepalive{
					KeepaliveProbes:   &wrappers.UInt32Value{Value: 3},
					KeepaliveTime:     &wrappers.UInt32Value{Value: 60},
					KeepaliveInterval: &wrappers.UInt32Value{Value: 6},
				},
			},
			CircuitBreakers: &envoy_config_cluster_v3.CircuitBreakers{
				Thresholds: []*envoy_config_cluster_v3.CircuitBreakers_Thresholds{{
					Priority:           envoy_config_core_v3.RoutingPriority_HIGH,
					MaxConnections:     &wrappers.UInt32Value{Value: 10000},
					MaxPendingRequests: &wrappers.UInt32Value{Value: 30000},
					MaxRequests:        &wrappers.UInt32Value{Value: 300000},
					MaxRetries:         &wrappers.UInt32Value{Value: 10},
				}, {
					Priority:           envoy_config_core_v3.RoutingPriority_DEFAULT,
					MaxConnections:     &wrappers.UInt32Value{Value: 30000},
					MaxPendingRequests: &wrappers.UInt32Value{Value: 30000},
					MaxRequests:        &wrappers.UInt32Value{Value: 300000},
					MaxRetries:         &wrappers.UInt32Value{Value: 300},
				}},
			},
		},
		}}

	conv.InitStats()
	cluster.NewClusterManagerSingleton(nil, nil, nil)
	err := xdsConfig.Init(dynamicResources, staticResources)
	if err != nil {
		t.Errorf("xDS init failed: %v", err)
	}

	adsClient := &ADSClient{
		AdsConfig:         xdsConfig.ADSConfig,
		StreamClientMutex: sync.RWMutex{},
		StreamClient:      nil,
		SendControlChan:   make(chan int),
		RecvControlChan:   make(chan int),
		StopChan:          make(chan int),
	}
	adsClient.Start()
	go adsClient.Stop()
}

// configSource returns a *envoy_config_core_v3.ConfigSource for cluster.
func configSource(cluster string) *envoy_config_core_v3.ConfigSource {
	return &envoy_config_core_v3.ConfigSource{
		ConfigSourceSpecifier: &envoy_config_core_v3.ConfigSource_ApiConfigSource{
			ApiConfigSource: &envoy_config_core_v3.ApiConfigSource{
				ApiType: envoy_config_core_v3.ApiConfigSource_GRPC,
				GrpcServices: []*envoy_config_core_v3.GrpcService{{
					TargetSpecifier: &envoy_config_core_v3.GrpcService_EnvoyGrpc_{
						EnvoyGrpc: &envoy_config_core_v3.GrpcService_EnvoyGrpc{
							ClusterName: cluster,
						},
					},
				}},
			},
		},
	}
}

// configSource returns a *envoy_config_core_v3.ApiConfigSource for cluster.
func configApiSource(cluster string) *envoy_config_core_v3.ApiConfigSource {
	return &envoy_config_core_v3.ApiConfigSource{
		ApiType: envoy_config_core_v3.ApiConfigSource_GRPC,
		GrpcServices: []*envoy_config_core_v3.GrpcService{{
			TargetSpecifier: &envoy_config_core_v3.GrpcService_EnvoyGrpc_{
				EnvoyGrpc: &envoy_config_core_v3.GrpcService_EnvoyGrpc{
					ClusterName: cluster,
				},
			},
		}},
	}

}

// localityLbEndpoints creates a new TCP envoy_config_endpoint_v3.LocalityLbEndpoints.
func localityLbEndpoints(address string, port int) *envoy_config_endpoint_v3.LocalityLbEndpoints {
	return &envoy_config_endpoint_v3.LocalityLbEndpoints{
		LbEndpoints: []*envoy_config_endpoint_v3.LbEndpoint{
			&envoy_config_endpoint_v3.LbEndpoint{
				HostIdentifier: &envoy_config_endpoint_v3.LbEndpoint_Endpoint{
					Endpoint: &envoy_config_endpoint_v3.Endpoint{
						Address: socketAddress(address, port),
					},
				},
			},
		},
	}
}

func socketAddress(address string, port int) *envoy_config_core_v3.Address {
	return &envoy_config_core_v3.Address{
		Address: &envoy_config_core_v3.Address_SocketAddress{
			SocketAddress: &envoy_config_core_v3.SocketAddress{
				Address: address,
				PortSpecifier: &envoy_config_core_v3.SocketAddress_PortValue{
					PortValue: uint32(port),
				},
			},
		},
	}
}

// endpoints returns a slice of LocalityLbEndpoints.
// The slice contains one entry, with one LbEndpoint per
// *envoy_config_core_v3.Address supplied.
func endpoints(addrs ...*envoy_config_core_v3.Address) []*envoy_config_endpoint_v3.LocalityLbEndpoints {
	lbendpoints := make([]*envoy_config_endpoint_v3.LbEndpoint, 0, len(addrs))
	for _, addr := range addrs {
		lbendpoints = append(lbendpoints, &envoy_config_endpoint_v3.LbEndpoint{
			HostIdentifier: &envoy_config_endpoint_v3.LbEndpoint_Endpoint{
				Endpoint: &envoy_config_endpoint_v3.Endpoint{
					Address: addr,
				},
			},
		})
	}
	return []*envoy_config_endpoint_v3.LocalityLbEndpoints{{
		LbEndpoints: lbendpoints,
	}}
}
