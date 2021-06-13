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

package conv

import (
	"testing"

	envoy_config_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_config_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_config_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_config_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_extensions_filters_http_fault_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/fault/v3"
	envoy_extensions_filters_network_http_connection_manager_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_extensions_filters_network_tcp_proxy_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	conversion "github.com/envoyproxy/go-control-plane/pkg/conversion"
	wellknown "github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/duration"
	pstruct "github.com/golang/protobuf/ptypes/struct"
	"google.golang.org/protobuf/types/known/durationpb"
	v2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/router"
	"mosn.io/mosn/pkg/server"
)

func messageToStruct(t *testing.T, msg proto.Message) *pstruct.Struct {
	s, err := conversion.MessageToStruct(msg)
	if err != nil {
		t.Fatalf("transfer failed: %v", err)
	}
	return s
}

func Test_updateListener(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("TestUpdateListener error: %v", r)
		}
	}()
	InitStats()

	// no support access log register
	//	accessLogFilterConfig := messageToStruct(t, &xdsaccesslog.FileAccessLog{
	//		Path: "/dev/stdout",
	//	})
	zeroSecond := new(duration.Duration)
	zeroSecond.Seconds = 0
	filterConfig := &envoy_extensions_filters_network_http_connection_manager_v3.HttpConnectionManager{
		CodecType:  envoy_extensions_filters_network_http_connection_manager_v3.HttpConnectionManager_AUTO,
		StatPrefix: "0.0.0.0_80",
		RouteSpecifier: &envoy_extensions_filters_network_http_connection_manager_v3.HttpConnectionManager_RouteConfig{
			RouteConfig: &envoy_config_route_v3.RouteConfiguration{
				Name: "80",
				VirtualHosts: []*envoy_config_route_v3.VirtualHost{
					{
						Name: "istio-egressgateway.istio-system.svc.cluster.local:80",
						Domains: []string{
							"istio-egressgateway.istio-system.svc.cluster.local",
							"istio-egressgateway.istio-system.svc.cluster.local:80",
							"istio-egressgateway.istio-system",
							"istio-egressgateway.istio-system:80",
							"istio-egressgateway.istio-system.svc.cluster",
							"istio-egressgateway.istio-system.svc.cluster:80",
							"istio-egressgateway.istio-system.svc",
							"istio-egressgateway.istio-system.svc:80",
							"172.19.3.204",
							"172.19.3.204:80",
						},
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
											Cluster: "outbound|80||istio-egressgateway.istio-system.svc.cluster.local",
										},
										ClusterNotFoundResponseCode:              envoy_config_route_v3.RouteAction_SERVICE_UNAVAILABLE,
										MetadataMatch:                            nil,
										PrefixRewrite:                            "",
										HostRewriteSpecifier:                     nil,
										Timeout:                                  zeroSecond,
										RetryPolicy:                              nil,
										HiddenEnvoyDeprecatedRequestMirrorPolicy: nil,
										Priority:                                 envoy_config_core_v3.RoutingPriority_DEFAULT,
										MaxGrpcTimeout:                           new(duration.Duration),
									},
								},
								Metadata: nil,
								Decorator: &envoy_config_route_v3.Decorator{
									Operation: "istio-egressgateway.istio-system.svc.cluster.local:80/*",
								},
								HiddenEnvoyDeprecatedPerFilterConfig: nil,
							},
						},
						RequireTls: envoy_config_route_v3.VirtualHost_NONE,
					},
					{
						Name: "istio-ingressgateway.istio-system.svc.cluster.local:80",
						Domains: []string{
							"istio-ingressgateway.istio-system.svc.cluster.local",
							"istio-ingressgateway.istio-system.svc.cluster.local:80",
							"istio-ingressgateway.istio-system",
							"istio-ingressgateway.istio-system:80",
							"istio-ingressgateway.istio-system.svc.cluster",
							"istio-ingressgateway.istio-system.svc.cluster:80",
							"istio-ingressgateway.istio-system.svc",
							"istio-ingressgateway.istio-system.svc:80",
							"172.19.8.101",
							"172.19.8.101:80",
						},
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
											Cluster: "outbound|80||istio-ingressgateway.istio-system.svc.cluster.local",
										},
										ClusterNotFoundResponseCode: envoy_config_route_v3.RouteAction_SERVICE_UNAVAILABLE,
										PrefixRewrite:               "",
										Timeout:                     zeroSecond,
										Priority:                    envoy_config_core_v3.RoutingPriority_DEFAULT,
										MaxGrpcTimeout:              new(duration.Duration),
									},
								},
								Decorator: &envoy_config_route_v3.Decorator{
									Operation: "istio-ingressgateway.istio-system.svc.cluster.local:80/*",
								},
								HiddenEnvoyDeprecatedPerFilterConfig: nil,
							},
						},
						RequireTls: envoy_config_route_v3.VirtualHost_NONE,
					},
					{
						Name: "nginx-ingress-lb.kube-system.svc.cluster.local:80",
						Domains: []string{
							"nginx-ingress-lb.kube-system.svc.cluster.local",
							"nginx-ingress-lb.kube-system.svc.cluster.local:80",
							"nginx-ingress-lb.kube-system",
							"nginx-ingress-lb.kube-system:80",
							"nginx-ingress-lb.kube-system.svc.cluster",
							"nginx-ingress-lb.kube-system.svc.cluster:80",
							"nginx-ingress-lb.kube-system.svc",
							"nginx-ingress-lb.kube-system.svc:80",
							"172.19.6.192:80",
						},
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
											Cluster: "outbound|80||nginx-ingress-lb.kube-system.svc.cluster.local",
										},
										ClusterNotFoundResponseCode: envoy_config_route_v3.RouteAction_SERVICE_UNAVAILABLE,
										PrefixRewrite:               "",
										Timeout:                     zeroSecond,
										Priority:                    envoy_config_core_v3.RoutingPriority_DEFAULT,
										MaxGrpcTimeout:              new(duration.Duration),
									},
								},
								Decorator: &envoy_config_route_v3.Decorator{
									Operation: "nginx-ingress-lb.kube-system.svc.cluster.local:80/*",
								},
								HiddenEnvoyDeprecatedPerFilterConfig: nil,
							},
						},
						RequireTls: envoy_config_route_v3.VirtualHost_NONE,
					},
				},
				ValidateClusters: NewBoolValue(false),
			},
		},
		//AccessLog: []*xdsfal.AccessLog{{
		//	Name:   "envoy.file_access_log",
		//	Filter: nil,
		//	ConfigType: &xdsfal.AccessLog_Config{
		//		Config: accessLogFilterConfig,
		//	},
		//}},
		UseRemoteAddress:            NewBoolValue(false),
		XffNumTrustedHops:           0,
		SkipXffAppend:               false,
		Via:                         "",
		GenerateRequestId:           NewBoolValue(true),
		ForwardClientCertDetails:    envoy_extensions_filters_network_http_connection_manager_v3.HttpConnectionManager_SANITIZE,
		SetCurrentClientCertDetails: nil,
		Proxy_100Continue:           false,
		RepresentIpv4RemoteAddressAsIpv4MappedIpv6: false,
	}
	filterName := wellknown.HTTPConnectionManager
	address := envoy_config_core_v3.Address{
		Address: &envoy_config_core_v3.Address_SocketAddress{
			SocketAddress: &envoy_config_core_v3.SocketAddress{
				Protocol: envoy_config_core_v3.SocketAddress_TCP,
				Address:  "0.0.0.0",
				PortSpecifier: &envoy_config_core_v3.SocketAddress_PortValue{
					PortValue: 80,
				},
				ResolverName: "",
				Ipv4Compat:   false,
			},
		},
	}
	//
	listenerConfig := &envoy_config_listener_v3.Listener{
		Name:    "0.0.0.0_80",
		Address: &address,
		FilterChains: []*envoy_config_listener_v3.FilterChain{
			{
				Filters: []*envoy_config_listener_v3.Filter{
					{
						Name: filterName,
						ConfigType: &envoy_config_listener_v3.Filter_TypedConfig{
							TypedConfig: messageToAny(t, filterConfig),
						},
					},
				},
			},
		},
		DeprecatedV1: &envoy_config_listener_v3.Listener_DeprecatedV1{
			BindToPort: NewBoolValue(false),
		},
		DrainType: envoy_config_listener_v3.Listener_DEFAULT,
	}
	ConvertAddOrUpdateListeners([]*envoy_config_listener_v3.Listener{listenerConfig})
	// Verify
	adapter := server.GetListenerAdapterInstance()
	if ln := adapter.FindListenerByName("", "0.0.0.0_80"); ln == nil {
		t.Fatal("no listener found")
	}
	routerMng := router.GetRoutersMangerInstance()
	if rw := routerMng.GetRouterWrapperByName("80"); rw == nil {
		t.Fatal("no router found")
	}
	// Test Update Stream Filter
	streamFilterConfig := &envoy_extensions_filters_http_fault_v3.HTTPFault{}
	filterConfig.HttpFilters = []*envoy_extensions_filters_network_http_connection_manager_v3.HttpFilter{
		&envoy_extensions_filters_network_http_connection_manager_v3.HttpFilter{
			Name: v2.FaultStream,
			ConfigType: &envoy_extensions_filters_network_http_connection_manager_v3.HttpFilter_TypedConfig{
				TypedConfig: messageToAny(t, streamFilterConfig),
			},
		},
	}
	listenerConfig.FilterChains[0].Filters[0] = &envoy_config_listener_v3.Filter{
		Name: filterName,
		ConfigType: &envoy_config_listener_v3.Filter_TypedConfig{
			TypedConfig: messageToAny(t, filterConfig),
		},
	}
	ConvertAddOrUpdateListeners([]*envoy_config_listener_v3.Listener{listenerConfig})
	ln := adapter.FindListenerByName("test_xds_server", "0.0.0.0_80")
	if ln == nil {
		t.Fatal("no listener found")
	}
	lnCfg := ln.Config()
	if len(lnCfg.StreamFilters) != 1 || lnCfg.StreamFilters[0].Type != v2.FaultStream {
		t.Fatalf("listener stream filters config is : %v", lnCfg.StreamFilters)
	}
	// tcp proxy networker
	filterName = wellknown.TCPProxy
	tcpConfig := &envoy_extensions_filters_network_tcp_proxy_v3.TcpProxy{
		StatPrefix: "tcp",
		ClusterSpecifier: &envoy_extensions_filters_network_tcp_proxy_v3.TcpProxy_Cluster{
			Cluster: "tcpCluster",
		},
	}

	listenerConfig = &envoy_config_listener_v3.Listener{
		Name:    "0.0.0.0_80",
		Address: &address,
		FilterChains: []*envoy_config_listener_v3.FilterChain{
			{
				Filters: []*envoy_config_listener_v3.Filter{
					{
						Name: filterName,
						ConfigType: &envoy_config_listener_v3.Filter_TypedConfig{
							TypedConfig: messageToAny(t, tcpConfig),
						},
					},
				},
			},
		},
		DeprecatedV1: &envoy_config_listener_v3.Listener_DeprecatedV1{
			BindToPort: NewBoolValue(false),
		},
		DrainType: envoy_config_listener_v3.Listener_DEFAULT,
	}
	ConvertAddOrUpdateListeners([]*envoy_config_listener_v3.Listener{listenerConfig})

	ConvertDeleteListeners([]*envoy_config_listener_v3.Listener{listenerConfig})
}

func Test_updateCluster(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("TestUpdateCluster error: %v", r)
		}
	}()

	addrsConfig := &envoy_config_core_v3.Address{
		Address: &envoy_config_core_v3.Address_SocketAddress{
			SocketAddress: &envoy_config_core_v3.SocketAddress{
				Protocol: envoy_config_core_v3.SocketAddress_TCP,
				Address:  "localhost",
				PortSpecifier: &envoy_config_core_v3.SocketAddress_PortValue{
					PortValue: 80,
				},
			},
		},
	}

	ClusterConfig := &envoy_config_cluster_v3.Cluster{
		Name:                 "addCluser",
		ClusterDiscoveryType: &envoy_config_cluster_v3.Cluster_Type{Type: envoy_config_cluster_v3.Cluster_EDS},
		EdsClusterConfig: &envoy_config_cluster_v3.Cluster_EdsClusterConfig{
			EdsConfig: &envoy_config_core_v3.ConfigSource{},
		},
		LbPolicy:                   envoy_config_cluster_v3.Cluster_ROUND_ROBIN,
		HiddenEnvoyDeprecatedHosts: []*envoy_config_core_v3.Address{addrsConfig},
		ConnectTimeout:             &durationpb.Duration{Seconds: 1},
	}

	if mc := ConvertClustersConfig([]*envoy_config_cluster_v3.Cluster{ClusterConfig}); mc == nil {
		t.Error("ConvertClustersConfig failed!")
	}

	ConvertUpdateClusters([]*envoy_config_cluster_v3.Cluster{ClusterConfig})
	ConvertDeleteClusters([]*envoy_config_cluster_v3.Cluster{ClusterConfig})
}

func Test_ConvertAddOrUpdateRouters(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("TestConvertAddOrUpdateRouters error: %v", r)
		}
	}()
	want := "addRoute"
	routeConfig := &envoy_config_route_v3.RouteConfiguration{
		Name: want,
		VirtualHosts: []*envoy_config_route_v3.VirtualHost{
			{
				Name: "istio-egressgateway.istio-system.svc.cluster.local:80",
				Domains: []string{
					"istio-egressgateway.istio-system.svc.cluster.local",
					"istio-egressgateway.istio-system.svc:80",
					"172.19.3.204",
					"172.19.3.204:80",
				},
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
									Cluster: "outbound|80||istio-egressgateway.istio-system.svc.cluster.local",
								},
								ClusterNotFoundResponseCode: envoy_config_route_v3.RouteAction_SERVICE_UNAVAILABLE,
								MetadataMatch:               nil,
								PrefixRewrite:               "",
								HostRewriteSpecifier:        nil,
								RetryPolicy:                 nil,
								RequestMirrorPolicies:       nil,
								Priority:                    envoy_config_core_v3.RoutingPriority_DEFAULT,
								MaxGrpcTimeout:              new(duration.Duration),
							},
						},
						Metadata: nil,
						Decorator: &envoy_config_route_v3.Decorator{
							Operation: "istio-egressgateway.istio-system.svc.cluster.local:80/*",
						},
						HiddenEnvoyDeprecatedPerFilterConfig: nil,
					},
				},
				RequireTls: envoy_config_route_v3.VirtualHost_NONE,
			},
		},
		ValidateClusters: NewBoolValue(false),
	}
	ConvertAddOrUpdateRouters([]*envoy_config_route_v3.RouteConfiguration{routeConfig})
	routersMngIns := router.GetRoutersMangerInstance()
	if routersMngIns == nil {
		t.Error("get routerMngIns failed!")
	}

	if rn := routersMngIns.GetRouterWrapperByName(want); rn == nil {
		t.Error("get routerMngIns failed!")
	}
}

func Test_ConvertUpdateEndpoints(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ConvertUpdateEndpoints error: %v", r)
		}
	}()
	cluster := "addCluser"
	ClusterConfig := &envoy_config_cluster_v3.Cluster{
		Name:                 cluster,
		ClusterDiscoveryType: &envoy_config_cluster_v3.Cluster_Type{Type: envoy_config_cluster_v3.Cluster_EDS},
		EdsClusterConfig: &envoy_config_cluster_v3.Cluster_EdsClusterConfig{
			EdsConfig: &envoy_config_core_v3.ConfigSource{},
		},
		LbPolicy: envoy_config_cluster_v3.Cluster_ROUND_ROBIN,
	}

	if mc := ConvertClustersConfig([]*envoy_config_cluster_v3.Cluster{ClusterConfig}); mc == nil {
		t.Error("ConvertClustersConfig failed!")
	}

	ConvertUpdateClusters([]*envoy_config_cluster_v3.Cluster{ClusterConfig})

	CLAConfig := &envoy_config_endpoint_v3.ClusterLoadAssignment{
		ClusterName: cluster,
		Endpoints: []*envoy_config_endpoint_v3.LocalityLbEndpoints{{
			LbEndpoints: []*envoy_config_endpoint_v3.LbEndpoint{{
				HostIdentifier: &envoy_config_endpoint_v3.LbEndpoint_Endpoint{
					Endpoint: &envoy_config_endpoint_v3.Endpoint{
						Address: &envoy_config_core_v3.Address{
							Address: &envoy_config_core_v3.Address_SocketAddress{
								SocketAddress: &envoy_config_core_v3.SocketAddress{
									Protocol: envoy_config_core_v3.SocketAddress_TCP,
									Address:  "127.0.0.1",
									PortSpecifier: &envoy_config_core_v3.SocketAddress_PortValue{
										PortValue: 80,
									},
								},
							},
						},
					},
				},
			}},
		}},
	}

	if err := ConvertUpdateEndpoints([]*envoy_config_endpoint_v3.ClusterLoadAssignment{CLAConfig}); err != nil {
		t.Errorf("ConvertUpdateEndpoints failed: %v", err)
	}

}
