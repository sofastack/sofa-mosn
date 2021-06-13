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

	envoy_api_v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	xdslistener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	xdsroute "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	xdshttpfault "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/fault/v2"
	xdshttp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	xdstcp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	xdsutil "github.com/envoyproxy/go-control-plane/pkg/conversion"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	xdswellknown "github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/duration"
	pstruct "github.com/golang/protobuf/ptypes/struct"
	"mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/router"
	"mosn.io/mosn/pkg/server"
)

func messageToStruct(t *testing.T, msg proto.Message) *pstruct.Struct {
	s, err := xdsutil.MessageToStruct(msg)
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
	filterConfig := &xdshttp.HttpConnectionManager{
		CodecType:  xdshttp.HttpConnectionManager_AUTO,
		StatPrefix: "0.0.0.0_80",
		RouteSpecifier: &xdshttp.HttpConnectionManager_RouteConfig{
			RouteConfig: &envoy_api_v2.RouteConfiguration{
				Name: "80",
				VirtualHosts: []*xdsroute.VirtualHost{
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
						Routes: []*xdsroute.Route{
							{
								Match: &xdsroute.RouteMatch{
									PathSpecifier: &xdsroute.RouteMatch_Prefix{
										Prefix: "/",
									},
								},
								Action: &xdsroute.Route_Route{
									Route: &xdsroute.RouteAction{
										ClusterSpecifier: &xdsroute.RouteAction_Cluster{
											Cluster: "outbound|80||istio-egressgateway.istio-system.svc.cluster.local",
										},
										ClusterNotFoundResponseCode: xdsroute.RouteAction_SERVICE_UNAVAILABLE,
										MetadataMatch:               nil,
										PrefixRewrite:               "",
										HostRewriteSpecifier:        nil,
										Timeout:                     zeroSecond,
										RetryPolicy:                 nil,
										RequestMirrorPolicy:         nil,
										Priority:                    core.RoutingPriority_DEFAULT,
										MaxGrpcTimeout:              new(duration.Duration),
									},
								},
								Metadata: nil,
								Decorator: &xdsroute.Decorator{
									Operation: "istio-egressgateway.istio-system.svc.cluster.local:80/*",
								},
								PerFilterConfig: nil,
							},
						},
						RequireTls: xdsroute.VirtualHost_NONE,
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
						Routes: []*xdsroute.Route{
							{
								Match: &xdsroute.RouteMatch{
									PathSpecifier: &xdsroute.RouteMatch_Prefix{
										Prefix: "/",
									},
								},
								Action: &xdsroute.Route_Route{
									Route: &xdsroute.RouteAction{
										ClusterSpecifier: &xdsroute.RouteAction_Cluster{
											Cluster: "outbound|80||istio-ingressgateway.istio-system.svc.cluster.local",
										},
										ClusterNotFoundResponseCode: xdsroute.RouteAction_SERVICE_UNAVAILABLE,
										PrefixRewrite:               "",
										Timeout:                     zeroSecond,
										Priority:                    core.RoutingPriority_DEFAULT,
										MaxGrpcTimeout:              new(duration.Duration),
									},
								},
								Decorator: &xdsroute.Decorator{
									Operation: "istio-ingressgateway.istio-system.svc.cluster.local:80/*",
								},
								PerFilterConfig: nil,
							},
						},
						RequireTls: xdsroute.VirtualHost_NONE,
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
						Routes: []*xdsroute.Route{
							{
								Match: &xdsroute.RouteMatch{
									PathSpecifier: &xdsroute.RouteMatch_Prefix{
										Prefix: "/",
									},
								},
								Action: &xdsroute.Route_Route{
									Route: &xdsroute.RouteAction{
										ClusterSpecifier: &xdsroute.RouteAction_Cluster{
											Cluster: "outbound|80||nginx-ingress-lb.kube-system.svc.cluster.local",
										},
										ClusterNotFoundResponseCode: xdsroute.RouteAction_SERVICE_UNAVAILABLE,
										PrefixRewrite:               "",
										Timeout:                     zeroSecond,
										Priority:                    core.RoutingPriority_DEFAULT,
										MaxGrpcTimeout:              new(duration.Duration),
									},
								},
								Decorator: &xdsroute.Decorator{
									Operation: "nginx-ingress-lb.kube-system.svc.cluster.local:80/*",
								},
								PerFilterConfig: nil,
							},
						},
						RequireTls: xdsroute.VirtualHost_NONE,
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
		ForwardClientCertDetails:    xdshttp.HttpConnectionManager_SANITIZE,
		SetCurrentClientCertDetails: nil,
		Proxy_100Continue:           false,
		RepresentIpv4RemoteAddressAsIpv4MappedIpv6: false,
	}
	filterName := wellknown.HTTPConnectionManager
	address := core.Address{
		Address: &core.Address_SocketAddress{
			SocketAddress: &core.SocketAddress{
				Protocol: core.SocketAddress_TCP,
				Address:  "0.0.0.0",
				PortSpecifier: &core.SocketAddress_PortValue{
					PortValue: 80,
				},
				ResolverName: "",
				Ipv4Compat:   false,
			},
		},
	}
	//
	listenerConfig := &envoy_api_v2.Listener{
		Name:    "0.0.0.0_80",
		Address: &address,
		FilterChains: []*xdslistener.FilterChain{
			{
				Filters: []*xdslistener.Filter{
					{
						Name: filterName,
						ConfigType: &xdslistener.Filter_Config{
							Config: messageToStruct(t, filterConfig),
						},
					},
				},
			},
		},
		DeprecatedV1: &envoy_api_v2.Listener_DeprecatedV1{
			BindToPort: NewBoolValue(false),
		},
		DrainType: envoy_api_v2.Listener_DEFAULT,
	}
	ConvertAddOrUpdateListeners([]*envoy_api_v2.Listener{listenerConfig})
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
	streamFilterConfig := &xdshttpfault.HTTPFault{}
	filterConfig.HttpFilters = []*xdshttp.HttpFilter{
		&xdshttp.HttpFilter{
			Name: v2.FaultStream,
			ConfigType: &xdshttp.HttpFilter_Config{
				Config: messageToStruct(t, streamFilterConfig),
			},
		},
	}
	listenerConfig.FilterChains[0].Filters[0] = &xdslistener.Filter{
		Name: filterName,
		ConfigType: &xdslistener.Filter_Config{
			Config: messageToStruct(t, filterConfig),
		},
	}
	ConvertAddOrUpdateListeners([]*envoy_api_v2.Listener{listenerConfig})
	ln := adapter.FindListenerByName("", "0.0.0.0_80")
	if ln == nil {
		t.Fatal("no listener found")
	}
	lnCfg := ln.Config()
	if len(lnCfg.StreamFilters) != 1 || lnCfg.StreamFilters[0].Type != v2.FaultStream {
		t.Fatalf("listener stream filters config is : %v", lnCfg.StreamFilters)
	}
	// tcp proxy networker
	filterName = xdswellknown.TCPProxy
	tcpConfig := &xdstcp.TcpProxy{
		StatPrefix: "tcp",
		ClusterSpecifier: &xdstcp.TcpProxy_Cluster{
			Cluster: "tcpCluster",
		},
	}

	listenerConfig = &envoy_api_v2.Listener{
		Name:    "0.0.0.0_80",
		Address: &address,
		FilterChains: []*xdslistener.FilterChain{
			{
				Filters: []*xdslistener.Filter{
					{
						Name: filterName,
						ConfigType: &xdslistener.Filter_Config{
							Config: messageToStruct(t, tcpConfig),
						},
					},
				},
			},
		},
		DeprecatedV1: &envoy_api_v2.Listener_DeprecatedV1{
			BindToPort: NewBoolValue(false),
		},
		DrainType: envoy_api_v2.Listener_DEFAULT,
	}
	ConvertAddOrUpdateListeners([]*envoy_api_v2.Listener{listenerConfig})

	ConvertDeleteListeners([]*envoy_api_v2.Listener{listenerConfig})
}

func Test_updateCluster(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("TestUpdateCluster error: %v", r)
		}
	}()

	addrsConfig := &core.Address{
		Address: &core.Address_SocketAddress{
			SocketAddress: &core.SocketAddress{
				Protocol: core.SocketAddress_TCP,
				Address:  "localhost",
				PortSpecifier: &core.SocketAddress_PortValue{
					PortValue: 80,
				},
			},
		},
	}

	ClusterConfig := &envoy_api_v2.Cluster{
		Name:                 "addCluser",
		ClusterDiscoveryType: &envoy_api_v2.Cluster_Type{Type: envoy_api_v2.Cluster_EDS},
		EdsClusterConfig: &envoy_api_v2.Cluster_EdsClusterConfig{
			EdsConfig: &core.ConfigSource{},
		},
		LbPolicy:       envoy_api_v2.Cluster_ROUND_ROBIN,
		Hosts:          []*core.Address{addrsConfig},
		ConnectTimeout: &duration.Duration{Seconds: 1},
	}

	if mc := ConvertClustersConfig([]*envoy_api_v2.Cluster{ClusterConfig}); mc == nil {
		t.Error("ConvertClustersConfig failed!")
	}

	ConvertUpdateClusters([]*envoy_api_v2.Cluster{ClusterConfig})
	ConvertDeleteClusters([]*envoy_api_v2.Cluster{ClusterConfig})
}

func Test_ConvertAddOrUpdateRouters(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("TestConvertAddOrUpdateRouters error: %v", r)
		}
	}()
	want := "addRoute"
	routeConfig := &envoy_api_v2.RouteConfiguration{
		Name: want,
		VirtualHosts: []*xdsroute.VirtualHost{
			{
				Name: "istio-egressgateway.istio-system.svc.cluster.local:80",
				Domains: []string{
					"istio-egressgateway.istio-system.svc.cluster.local",
					"istio-egressgateway.istio-system.svc:80",
					"172.19.3.204",
					"172.19.3.204:80",
				},
				Routes: []*xdsroute.Route{
					{
						Match: &xdsroute.RouteMatch{
							PathSpecifier: &xdsroute.RouteMatch_Prefix{
								Prefix: "/",
							},
						},
						Action: &xdsroute.Route_Route{
							Route: &xdsroute.RouteAction{
								ClusterSpecifier: &xdsroute.RouteAction_Cluster{
									Cluster: "outbound|80||istio-egressgateway.istio-system.svc.cluster.local",
								},
								ClusterNotFoundResponseCode: xdsroute.RouteAction_SERVICE_UNAVAILABLE,
								MetadataMatch:               nil,
								PrefixRewrite:               "",
								HostRewriteSpecifier:        nil,
								RetryPolicy:                 nil,
								RequestMirrorPolicy:         nil,
								Priority:                    core.RoutingPriority_DEFAULT,
								MaxGrpcTimeout:              new(duration.Duration),
							},
						},
						Metadata: nil,
						Decorator: &xdsroute.Decorator{
							Operation: "istio-egressgateway.istio-system.svc.cluster.local:80/*",
						},
						PerFilterConfig: nil,
					},
				},
				RequireTls: xdsroute.VirtualHost_NONE,
			},
		},
		ValidateClusters: NewBoolValue(false),
	}
	ConvertAddOrUpdateRouters([]*envoy_api_v2.RouteConfiguration{routeConfig})
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
	ClusterConfig := &envoy_api_v2.Cluster{
		Name:                 cluster,
		ClusterDiscoveryType: &envoy_api_v2.Cluster_Type{Type: envoy_api_v2.Cluster_EDS},
		EdsClusterConfig: &envoy_api_v2.Cluster_EdsClusterConfig{
			EdsConfig: &core.ConfigSource{},
		},
		LbPolicy: envoy_api_v2.Cluster_ROUND_ROBIN,
	}

	if mc := ConvertClustersConfig([]*envoy_api_v2.Cluster{ClusterConfig}); mc == nil {
		t.Error("ConvertClustersConfig failed!")
	}

	ConvertUpdateClusters([]*envoy_api_v2.Cluster{ClusterConfig})

	CLAConfig := &envoy_api_v2.ClusterLoadAssignment{
		ClusterName: cluster,
		Endpoints: []*endpoint.LocalityLbEndpoints{{
			LbEndpoints: []*endpoint.LbEndpoint{{
				HostIdentifier: &endpoint.LbEndpoint_Endpoint{
					Endpoint: &endpoint.Endpoint{
						Address: &core.Address{
							Address: &core.Address_SocketAddress{
								SocketAddress: &core.SocketAddress{
									Protocol: core.SocketAddress_TCP,
									Address:  "127.0.0.1",
									PortSpecifier: &core.SocketAddress_PortValue{
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

	if err := ConvertUpdateEndpoints([]*envoy_api_v2.ClusterLoadAssignment{CLAConfig}); err != nil {
		t.Errorf("ConvertUpdateEndpoints failed: %v", err)
	}

}
