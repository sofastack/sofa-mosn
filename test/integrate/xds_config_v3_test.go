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

package integrate

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	envoy_config_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_config_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_config_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_config_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_extensions_filters_network_http_connection_manager_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_service_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	conversion "github.com/envoyproxy/go-control-plane/pkg/conversion"
	resource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/proto"
	ptypes "github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	_struct "github.com/golang/protobuf/ptypes/struct"
	"github.com/golang/protobuf/ptypes/wrappers"
	jsoniter "github.com/json-iterator/go"
	v2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/configmanager"
	_ "mosn.io/mosn/pkg/filter/stream/faultinject"
	"mosn.io/mosn/pkg/mosn"
	"mosn.io/mosn/pkg/xds/v3/conv"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type effectiveConfig struct {
	MOSNConfig interface{}                       `json:"mosn_config,omitempty"`
	Listener   map[string]v2.Listener            `json:"listener,omitempty"`
	Cluster    map[string]v2.Cluster             `json:"cluster,omitempty"`
	Routers    map[string]v2.RouterConfiguration `josn:"routers,omitempty"`
}

func handleListenersResp(msg *envoy_service_discovery_v3.DiscoveryResponse) []*envoy_config_listener_v3.Listener {
	listeners := make([]*envoy_config_listener_v3.Listener, 0)
	for _, res := range msg.Resources {
		listener := &envoy_config_listener_v3.Listener{}
		ptypes.UnmarshalAny(res, listener)
		listeners = append(listeners, listener)
	}
	return listeners
}

func handleEndpointsResp(msg *envoy_service_discovery_v3.DiscoveryResponse) []*envoy_config_endpoint_v3.ClusterLoadAssignment {
	lbAssignments := make([]*envoy_config_endpoint_v3.ClusterLoadAssignment, 0)
	for _, res := range msg.Resources {
		lbAssignment := &envoy_config_endpoint_v3.ClusterLoadAssignment{}
		ptypes.UnmarshalAny(res, lbAssignment)
		lbAssignments = append(lbAssignments, lbAssignment)
	}
	return lbAssignments
}

func handleClustersResp(msg *envoy_service_discovery_v3.DiscoveryResponse) []*envoy_config_cluster_v3.Cluster {
	clusters := make([]*envoy_config_cluster_v3.Cluster, 0)
	for _, res := range msg.Resources {
		cluster := &envoy_config_cluster_v3.Cluster{}
		ptypes.UnmarshalAny(res, cluster)
		clusters = append(clusters, cluster)
	}
	return clusters
}

func handleXdsData(mosnConfig *v2.MOSNConfig, xdsFiles []string) error {
	for _, fileName := range xdsFiles {
		file := filepath.Join("testdata", fileName)
		msg := &envoy_service_discovery_v3.DiscoveryResponse{}

		if data, err := ioutil.ReadFile(file); err == nil {
			proto.Unmarshal(data, msg)
		} else {
			return err
		}

		switch msg.TypeUrl {
		case resource.ListenerType:
			listeners := handleListenersResp(msg)
			fmt.Printf("get %d listeners from LDS\n", len(listeners))
			conv.ConvertAddOrUpdateListeners(listeners)
		case resource.EndpointType:
			endpoints := handleEndpointsResp(msg)
			fmt.Printf("get %d endpoints from EDS\n", len(endpoints))
			conv.ConvertUpdateEndpoints(endpoints)
		case resource.ClusterType:
			clusters := handleClustersResp(msg)
			fmt.Printf("get %d clusters from CDS\n", len(clusters))
			conv.ConvertUpdateClusters(clusters)
		default:
			return fmt.Errorf("unkown type: %s", msg.TypeUrl)
		}
	}
	return nil
}

func TestConfigAddAndUpdate(t *testing.T) {
	mosnConfig := configmanager.Load(filepath.Join("testdata", "envoy.json"))
	configmanager.Reset()
	configmanager.SetMosnConfig(mosnConfig)
	Mosn := mosn.NewMosn(mosnConfig)
	Mosn.Start()

	buf, err := configmanager.DumpJSON()
	if err != nil {
		t.Fatal(err)
	}
	var m effectiveConfig
	err = json.Unmarshal(buf, &m)
	if err != nil {
		t.Fatal(err)
	}

	if m.MOSNConfig == nil {
		t.Fatalf("mosn_config missing")
	}
	if len(m.Listener) > 0 {
		t.Fatalf("should not have listners")
	}
	if len(m.Cluster) > 0 {
		t.Fatalf("should not have clusters")
	}

	loadXdsData()

	buf, err = configmanager.DumpJSON()
	if err != nil {
		t.Fatal(err)
	}
	err = json.Unmarshal(buf, &m)
	if err != nil {
		t.Fatal(err)
	}

	if m.MOSNConfig == nil {
		t.Fatalf("mosn_config missing")
	}
	if len(m.Listener) != 1 {
		t.Fatalf("should have 1 listeners, but got %d", len(m.Listener))
	}

	if listener, ok := m.Listener["0.0.0.0_9080"]; !ok {
		t.Fatalf("listener[0.0.0.0_9080] is missing")
	} else {
		if listener.Name != "0.0.0.0_9080" || listener.BindToPort || len(listener.FilterChains) != 1 {
			t.Fatalf("error listener[0.0.0.0_9080] config: %v", listener)
		}

		if len(listener.FilterChains[0].Filters) != 1 {
			t.Fatalf("error listener[0.0.0.0_9080] config: %v", listener)
		}

		if len(m.Routers) != 1 {
			t.Fatalf("listener[0.0.0.0_9080] router config is wrong")
		}
		for _, rcfg := range m.Routers {
			vhs := rcfg.VirtualHosts
			if len(vhs) != 4 {
				t.Fatalf("listener[0.0.0.0_9080] virtual hosts is not 3, got %d", len(vhs))
			}
			vh := vhs[3]
			routers := vh.Routers
			// 第一次 reviews 没有按照版本和权重来路由（v1,v2,v3 轮训）
			clusterName := routers[0].Route.ClusterName
			if clusterName != "outbound|9080||reviews.default.svc.cluster.local" {
				t.Fatalf("reviews.default.svc.cluster.local:9080 should route to [outbound|9080||reviews.default.svc.cluster.local], but got %s", clusterName)
			}
		}
	}

	if len(m.Cluster) != 1 {
		t.Fatalf("should have 1 clusters, but got %d", len(m.Cluster))
	}

	if cluster, ok := m.Cluster["outbound|9080||productpage.default.svc.cluster.local"]; !ok {
		t.Fatalf("cluster[outbound|9080||productpage.default.svc.cluster.local] is missing")
	} else {
		if cluster.Name != "outbound|9080||productpage.default.svc.cluster.local" ||
			cluster.LbType != v2.LB_ROUNDROBIN || len(cluster.Hosts) != 1 {
			t.Fatalf("error cluster config: %v", cluster)
		}

		if cluster.Hosts[0].Address != "172.16.1.171:9080" {
			t.Fatalf("error host: %v", cluster.Hosts[0])
		}
	}

	loadXdsData2()

	buf, err = configmanager.DumpJSON()
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(buf, &m)

	if m.MOSNConfig == nil {
		t.Fatalf("mosn_config missing")
	}
	if len(m.Listener) != 1 {
		t.Fatalf("should have 1 listeners, but got %d", len(m.Listener))
	}

	if listener, ok := m.Listener["0.0.0.0_9080"]; !ok {
		t.Fatalf("listener[0.0.0.0_9080] is missing")
	} else {
		if listener.Name != "0.0.0.0_9080" || listener.BindToPort || len(listener.FilterChains) != 1 {
			t.Fatalf("error listener config: %v", listener)
		}
		if len(m.Routers) != 1 {
			t.Fatalf("listener[0.0.0.0_9080] router config is wrong")
		}
		for _, rcfg := range m.Routers {
			vhs := rcfg.VirtualHosts
			if len(vhs) != 4 {
				t.Fatalf("listener[0.0.0.0_9080] virtual hosts is not 3, got %d", len(vhs))
			}
			vh := vhs[3]
			router := vh.Routers[0].Route
			if router.ClusterName != "" {
				t.Fatalf("cluster_name is not omitempty: %s", router.ClusterName)
			}
			if len(router.WeightedClusters) != 2 {
				t.Fatalf("reviews.default.svc.cluster.local:9080 should route to weighted_clusters")
			}
			clusterName1 := router.WeightedClusters[0].Cluster.Name
			clusterName2 := router.WeightedClusters[1].Cluster.Name
			weight1 := router.WeightedClusters[0].Cluster.Weight
			weight2 := router.WeightedClusters[1].Cluster.Weight
			// 第二次 review，按照 v1 和 v3 版本各 50% 的权重路由
			if clusterName1 != "outbound|9080|v1|reviews.default.svc.cluster.local" || weight1 != 50 ||
				clusterName2 != "outbound|9080|v3|reviews.default.svc.cluster.local" || weight2 != 50 {
				t.Fatalf("reviews.default.svc.cluster.local:9080 should route to v1(50) & v3(50)")
			}
		}
	}

	if len(m.Cluster) != 1 {
		t.Fatalf("should have 1 clusters, but got %d", len(m.Cluster))
	}

	if cluster, ok := m.Cluster["outbound|9080||productpage.default.svc.cluster.local"]; !ok {
		t.Fatalf("cluster[outbound|9080||productpage.default.svc.cluster.local] is missing")
	} else {
		if cluster.Name != "outbound|9080||productpage.default.svc.cluster.local" ||
			cluster.LbType != v2.LB_ROUNDROBIN || len(cluster.Hosts) != 1 {
			t.Fatalf("error cluster config: %v", cluster)
		}

		if cluster.Hosts[0].Address != "172.16.1.171:9080" {
			t.Fatalf("error host: %v", cluster.Hosts[0])
		}
	}

	Mosn.Close()
	configmanager.Reset()
}

func loadXdsData2() {
	// Listeners
	listener := &envoy_config_listener_v3.Listener{
		Name: "0.0.0.0_9080",
		Address: &envoy_config_core_v3.Address{
			Address: &envoy_config_core_v3.Address_SocketAddress{
				SocketAddress: &envoy_config_core_v3.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &envoy_config_core_v3.SocketAddress_PortValue{
						PortValue: 9080,
					},
				},
			},
		},
		FilterChains: []*envoy_config_listener_v3.FilterChain{
			{
				FilterChainMatch: nil,
				Filters: []*envoy_config_listener_v3.Filter{
					{
						Name: wellknown.HTTPConnectionManager,
						ConfigType: &envoy_config_listener_v3.Filter_TypedConfig{
							TypedConfig: MessageToAny(&envoy_extensions_filters_network_http_connection_manager_v3.HttpConnectionManager{
								RouteSpecifier: &envoy_extensions_filters_network_http_connection_manager_v3.HttpConnectionManager_RouteConfig{
									RouteConfig: &envoy_config_route_v3.RouteConfiguration{
										Name: "test_router_name",
										VirtualHosts: []*envoy_config_route_v3.VirtualHost{
											{},
											{},
											{},
											{
												Routes: []*envoy_config_route_v3.Route{
													{
														Match: &envoy_config_route_v3.RouteMatch{
															PathSpecifier: &envoy_config_route_v3.RouteMatch_Prefix{
																Prefix: "/",
															},
														},
														Action: &envoy_config_route_v3.Route_Route{
															Route: &envoy_config_route_v3.RouteAction{
																ClusterSpecifier: &envoy_config_route_v3.RouteAction_WeightedClusters{
																	WeightedClusters: &envoy_config_route_v3.WeightedCluster{
																		Clusters: []*envoy_config_route_v3.WeightedCluster_ClusterWeight{
																			{
																				Name:   "outbound|9080|v1|reviews.default.svc.cluster.local",
																				Weight: &wrappers.UInt32Value{Value: 50},
																			},
																			{
																				Name:   "outbound|9080|v3|reviews.default.svc.cluster.local",
																				Weight: &wrappers.UInt32Value{Value: 50},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							}),
						},
					},
				},
			},
		},
	}
	// Clusters
	cluster := &envoy_config_cluster_v3.Cluster{
		Name:     "outbound|9080||productpage.default.svc.cluster.local",
		LbPolicy: envoy_config_cluster_v3.Cluster_ROUND_ROBIN,
		LoadAssignment: &envoy_config_endpoint_v3.ClusterLoadAssignment{
			Endpoints: []*envoy_config_endpoint_v3.LocalityLbEndpoints{
				{
					LbEndpoints: []*envoy_config_endpoint_v3.LbEndpoint{
						{
							HostIdentifier: &envoy_config_endpoint_v3.LbEndpoint_Endpoint{
								Endpoint: &envoy_config_endpoint_v3.Endpoint{
									Address: &envoy_config_core_v3.Address{
										Address: &envoy_config_core_v3.Address_SocketAddress{
											SocketAddress: &envoy_config_core_v3.SocketAddress{
												Address: "172.16.1.171",
												PortSpecifier: &envoy_config_core_v3.SocketAddress_PortValue{
													PortValue: 9080,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	listeners := []*envoy_config_listener_v3.Listener{listener}
	clusters := []*envoy_config_cluster_v3.Cluster{cluster}
	conv.ConvertAddOrUpdateListeners(listeners)
	conv.ConvertUpdateClusters(clusters)
}

func loadXdsData() {
	// Listeners
	listener := &envoy_config_listener_v3.Listener{
		Name: "0.0.0.0_9080",
		Address: &envoy_config_core_v3.Address{
			Address: &envoy_config_core_v3.Address_SocketAddress{
				SocketAddress: &envoy_config_core_v3.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &envoy_config_core_v3.SocketAddress_PortValue{
						PortValue: 9080,
					},
				},
			},
		},
		DeprecatedV1: &envoy_config_listener_v3.Listener_DeprecatedV1{
			BindToPort: &wrappers.BoolValue{Value: false},
		},
		FilterChains: []*envoy_config_listener_v3.FilterChain{
			{
				FilterChainMatch: nil,
				Filters: []*envoy_config_listener_v3.Filter{
					{
						Name: wellknown.HTTPConnectionManager,
						ConfigType: &envoy_config_listener_v3.Filter_TypedConfig{
							TypedConfig: MessageToAny(&envoy_extensions_filters_network_http_connection_manager_v3.HttpConnectionManager{
								RouteSpecifier: &envoy_extensions_filters_network_http_connection_manager_v3.HttpConnectionManager_RouteConfig{
									RouteConfig: &envoy_config_route_v3.RouteConfiguration{
										Name: "test_router_name",
										VirtualHosts: []*envoy_config_route_v3.VirtualHost{
											{},
											{},
											{},
											{
												Routes: []*envoy_config_route_v3.Route{
													{
														Match: &envoy_config_route_v3.RouteMatch{
															PathSpecifier: &envoy_config_route_v3.RouteMatch_Prefix{
																Prefix: "/",
															},
														},
														Action: &envoy_config_route_v3.Route_Route{
															Route: &envoy_config_route_v3.RouteAction{
																ClusterSpecifier: &envoy_config_route_v3.RouteAction_Cluster{
																	Cluster: "outbound|9080||reviews.default.svc.cluster.local",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							}),
						},
					},
				},
			},
		},
	}
	// Clusters
	cluster := &envoy_config_cluster_v3.Cluster{
		Name:     "outbound|9080||productpage.default.svc.cluster.local",
		LbPolicy: envoy_config_cluster_v3.Cluster_ROUND_ROBIN,
		LoadAssignment: &envoy_config_endpoint_v3.ClusterLoadAssignment{
			Endpoints: []*envoy_config_endpoint_v3.LocalityLbEndpoints{
				{
					LbEndpoints: []*envoy_config_endpoint_v3.LbEndpoint{
						{
							HostIdentifier: &envoy_config_endpoint_v3.LbEndpoint_Endpoint{
								Endpoint: &envoy_config_endpoint_v3.Endpoint{
									Address: &envoy_config_core_v3.Address{
										Address: &envoy_config_core_v3.Address_SocketAddress{
											SocketAddress: &envoy_config_core_v3.SocketAddress{
												Address: "172.16.1.171",
												PortSpecifier: &envoy_config_core_v3.SocketAddress_PortValue{
													PortValue: 9080,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	listeners := []*envoy_config_listener_v3.Listener{listener}
	clusters := []*envoy_config_cluster_v3.Cluster{cluster}
	conv.ConvertAddOrUpdateListeners(listeners)
	conv.ConvertUpdateClusters(clusters)
}

// MessageToStruct converts from proto message to proto Struct
func MessageToStruct(msg proto.Message) *_struct.Struct {
	s, err := conversion.MessageToStruct(msg)
	if err != nil {
		return &_struct.Struct{}
	}
	return s
}

// MessageToAny converts from proto message to proto Any
func MessageToAny(msg proto.Message) *any.Any {
	s, err := ptypes.MarshalAny(msg)
	if err != nil {
		return nil
	}
	return s
}
