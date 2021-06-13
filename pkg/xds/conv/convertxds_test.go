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
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"

	xdsapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoy_api_v2_auth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	xdscore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	xdsendpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	xdslistener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	xdsroute "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	xdsaccesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v2"
	xdsfal "github.com/envoyproxy/go-control-plane/envoy/config/filter/accesslog/v2"
	xdsfault "github.com/envoyproxy/go-control-plane/envoy/config/filter/fault/v2"
	xdshttpfault "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/fault/v2"
	xdshttpgzip "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/gzip/v2"
	xdshttp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	xdstcp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	xdstype "github.com/envoyproxy/go-control-plane/envoy/type"
	xdswellknown "github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/proto"
	ptypes "github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	v2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/configmanager"
	"mosn.io/mosn/pkg/filter/stream/faultinject"
	v1 "mosn.io/mosn/pkg/mixer/v1"
	"mosn.io/mosn/pkg/mixer/v1/config/client"
	"mosn.io/mosn/pkg/router"
	"mosn.io/mosn/pkg/server"
	"mosn.io/mosn/pkg/upstream/cluster"
)

func TestMain(m *testing.M) {
	// init
	router.NewRouterManager()
	cm := cluster.NewClusterManagerSingleton(nil, nil, nil)
	sc := server.NewConfig(&v2.ServerConfig{
		ServerName:      "test_xds_server",
		DefaultLogPath:  "stdout",
		DefaultLogLevel: "FATAL",
	})
	server.NewServer(sc, &mockCMF{}, cm)
	os.Exit(m.Run())
}

// messageToAny converts from proto message to proto Any
func messageToAny(t *testing.T, msg proto.Message) *any.Any {
	s, err := ptypes.MarshalAny(msg)
	if err != nil {
		t.Fatalf("transfer failed: %v", err)
		return nil
	}
	return s
}

type mockIdentifier struct {
}

// todo fill the unit test
func Test_convertEndpointsConfig(t *testing.T) {
	type args struct {
		xdsEndpoint *xdsendpoint.LocalityLbEndpoints
	}
	tests := []struct {
		name string
		args args
		want []v2.Host
	}{
		{
			name: "case1",
			args: args{
				xdsEndpoint: &xdsendpoint.LocalityLbEndpoints{
					Priority: 1,
				},
			},
			want: []v2.Host{},
		},
		{
			name: "case2",
			args: args{
				xdsEndpoint: &xdsendpoint.LocalityLbEndpoints{
					LbEndpoints: []*xdsendpoint.LbEndpoint{
						{
							HostIdentifier: &xdsendpoint.LbEndpoint_Endpoint{
								Endpoint: &xdsendpoint.Endpoint{
									Address: &core.Address{
										Address: &core.Address_SocketAddress{
											SocketAddress: &core.SocketAddress{
												Address:       "192.168.0.1",
												Protocol:      core.SocketAddress_TCP,
												PortSpecifier: &core.SocketAddress_PortValue{PortValue: 8080},
											},
										},
									},
								},
							},
							LoadBalancingWeight: &wrappers.UInt32Value{Value: 20},
						},
						{
							HostIdentifier: &xdsendpoint.LbEndpoint_Endpoint{
								Endpoint: &xdsendpoint.Endpoint{
									Address: &core.Address{
										Address: &core.Address_SocketAddress{
											SocketAddress: &core.SocketAddress{
												Address:       "192.168.0.2",
												Protocol:      core.SocketAddress_TCP,
												PortSpecifier: &core.SocketAddress_PortValue{PortValue: 8080},
											},
										},
									},
								},
							},
							LoadBalancingWeight: &wrappers.UInt32Value{Value: 0},
						},
						{
							HostIdentifier: &xdsendpoint.LbEndpoint_Endpoint{
								Endpoint: &xdsendpoint.Endpoint{
									Address: &core.Address{
										Address: &core.Address_SocketAddress{
											SocketAddress: &core.SocketAddress{
												Address:       "192.168.0.3",
												Protocol:      core.SocketAddress_TCP,
												PortSpecifier: &core.SocketAddress_PortValue{PortValue: 8080},
											},
										},
									},
								},
							},
							LoadBalancingWeight: &wrappers.UInt32Value{Value: 200},
						},
					},
				},
			},
			want: []v2.Host{
				{
					HostConfig: v2.HostConfig{
						Address: "192.168.0.1:8080",
						Weight:  20,
					},
				},
				{
					HostConfig: v2.HostConfig{
						Address: "192.168.0.2:8080",
						Weight:  configmanager.MinHostWeight,
					},
				},
				{
					HostConfig: v2.HostConfig{
						Address: "192.168.0.3:8080",
						Weight:  configmanager.MaxHostWeight,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConvertEndpointsConfig(tt.args.xdsEndpoint); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertEndpointsConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_convertHeaders(t *testing.T) {
	type args struct {
		xdsHeaders []*xdsroute.HeaderMatcher
	}
	tests := []struct {
		name string
		args args
		want []v2.HeaderMatcher
	}{
		{
			name: "case1",
			args: args{
				xdsHeaders: []*xdsroute.HeaderMatcher{
					{
						Name: "end-user",
						HeaderMatchSpecifier: &xdsroute.HeaderMatcher_ExactMatch{
							ExactMatch: "jason",
						},
						InvertMatch: false,
					},
				},
			},
			want: []v2.HeaderMatcher{
				{
					Name:  "end-user",
					Value: "jason",
					Regex: false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertHeaders(tt.args.xdsHeaders); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertHeaders(xdsHeaders []*xdsroute.HeaderMatcher) = %v, want %v", got, tt.want)
			}
		})
	}
}

func NewBoolValue(val bool) *wrappers.BoolValue {
	return &wrappers.BoolValue{
		Value: val,
	}
}

func Test_convertListenerConfig(t *testing.T) {
	type args struct {
		xdsListener  *xdsapi.Listener
		address      core.Address
		filterName   string
		filterConfig *xdshttp.HttpConnectionManager
	}

	accessLogFilterConfig := messageToAny(t, &xdsaccesslog.FileAccessLog{
		Path: "/dev/stdout",
	})

	zeroSecond := new(duration.Duration)
	zeroSecond.Seconds = 0

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "0.0.0.0_80",
			args: args{
				filterConfig: &xdshttp.HttpConnectionManager{
					CodecType:  xdshttp.HttpConnectionManager_AUTO,
					StatPrefix: "0.0.0.0_80",
					RouteSpecifier: &xdshttp.HttpConnectionManager_RouteConfig{
						RouteConfig: &xdsapi.RouteConfiguration{
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
					ServerName: "",
					AccessLog: []*xdsfal.AccessLog{{
						Name:   "envoy.file_access_log",
						Filter: nil,
						ConfigType: &xdsfal.AccessLog_TypedConfig{
							TypedConfig: accessLogFilterConfig,
						},
					}},
					UseRemoteAddress:            NewBoolValue(false),
					XffNumTrustedHops:           0,
					SkipXffAppend:               false,
					Via:                         "",
					GenerateRequestId:           NewBoolValue(true),
					ForwardClientCertDetails:    xdshttp.HttpConnectionManager_SANITIZE,
					SetCurrentClientCertDetails: nil,
					Proxy_100Continue:           false,
					RepresentIpv4RemoteAddressAsIpv4MappedIpv6: false,
				},
				filterName: "envoy.http_connection_manager",
				address: core.Address{
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
				},
			},
			want: `{"name":"0.0.0.0_80","address":"","bind_port":false,"handoff_restoreddestination":false,"log_path":"stdout","access_logs":[{"log_path":"/dev/stdout"}],"filter_chains":[{"match":"\u003cnil\u003e","tls_context":{"status":false,"type":"","extend_verify":null},"filters":[{"type":"proxy","config":{"downstream_protocol":"Http1","extend_config":null,"name":"","support_dynamic_route":true,"upstream_protocol":"Http1","validate_clusters":false,"virtual_hosts":[{"domains":["istio-egressgateway.istio-system.svc.cluster.local","istio-egressgateway.istio-system.svc.cluster.local:80","istio-egressgateway.istio-system","istio-egressgateway.istio-system:80","istio-egressgateway.istio-system.svc.cluster","istio-egressgateway.istio-system.svc.cluster:80","istio-egressgateway.istio-system.svc","istio-egressgateway.istio-system.svc:80","172.19.3.204","172.19.3.204:80"],"name":"istio-egressgateway.istio-system.svc.cluster.local:80","require_tls":"NONE","routers":[{"decorator":"operation:\"istio-egressgateway.istio-system.svc.cluster.local:80/*\" ","match":{"case_sensitive":false,"headers":null,"path":"","prefix":"/","regex":"","runtime":{"default_value":0,"runtime_key":""}},"metadata":{"filter_metadata":{"mosn.lb":null}},"redirect":{"host_redirect":"","path_redirect":"","response_code":0},"route":{"cluster_header":"","cluster_name":"outbound|80||istio-egressgateway.istio-system.svc.cluster.local","metadata_match":{"filter_metadata":{"mosn.lb":null}},"retry_policy":{"num_retries":0,"retry_on":false,"retry_timeout":"0s"},"timeout":"0s","weighted_clusters":null}}],"virtual_clusters":null},{"domains":["istio-ingressgateway.istio-system.svc.cluster.local","istio-ingressgateway.istio-system.svc.cluster.local:80","istio-ingressgateway.istio-system","istio-ingressgateway.istio-system:80","istio-ingressgateway.istio-system.svc.cluster","istio-ingressgateway.istio-system.svc.cluster:80","istio-ingressgateway.istio-system.svc","istio-ingressgateway.istio-system.svc:80","172.19.8.101","172.19.8.101:80"],"name":"istio-ingressgateway.istio-system.svc.cluster.local:80","require_tls":"NONE","routers":[{"decorator":"operation:\"istio-ingressgateway.istio-system.svc.cluster.local:80/*\" ","match":{"case_sensitive":false,"headers":null,"path":"","prefix":"/","regex":"","runtime":{"default_value":0,"runtime_key":""}},"metadata":{"filter_metadata":{"mosn.lb":null}},"redirect":{"host_redirect":"","path_redirect":"","response_code":0},"route":{"cluster_header":"","cluster_name":"outbound|80||istio-ingressgateway.istio-system.svc.cluster.local","metadata_match":{"filter_metadata":{"mosn.lb":null}},"retry_policy":{"num_retries":0,"retry_on":false,"retry_timeout":"0s"},"timeout":"0s","weighted_clusters":null}}],"virtual_clusters":null},{"domains":["nginx-ingress-lb.kube-system.svc.cluster.local","nginx-ingress-lb.kube-system.svc.cluster.local:80","nginx-ingress-lb.kube-system","nginx-ingress-lb.kube-system:80","nginx-ingress-lb.kube-system.svc.cluster","nginx-ingress-lb.kube-system.svc.cluster:80","nginx-ingress-lb.kube-system.svc","nginx-ingress-lb.kube-system.svc:80","172.19.6.192:80","172.19.8.101:80"],"name":"nginx-ingress-lb.kube-system.svc.cluster.local:80","require_tls":"NONE","routers":[{"decorator":"operation:\"nginx-ingress-lb.kube-system.svc.cluster.local:80/*\" ","match":{"case_sensitive":false,"headers":null,"path":"","prefix":"/","regex":"","runtime":{"default_value":0,"runtime_key":""}},"metadata":{"filter_metadata":{"mosn.lb":null}},"redirect":{"host_redirect":"","path_redirect":"","response_code":0},"route":{"cluster_header":"","cluster_name":"outbound|80||nginx-ingress-lb.kube-system.svc.cluster.local","metadata_match":{"filter_metadata":{"mosn.lb":null}},"retry_policy":{"num_retries":0,"retry_on":false,"retry_timeout":"0s"},"timeout":"0s","weighted_clusters":null}}],"virtual_clusters":null}]}}]}],"inspector":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := messageToAny(t, tt.args.filterConfig)
			listenerConfig := &xdsapi.Listener{
				Name:    "0.0.0.0_80",
				Address: &tt.args.address,
				FilterChains: []*xdslistener.FilterChain{
					{
						FilterChainMatch: nil,
						TlsContext: &envoy_api_v2_auth.DownstreamTlsContext{
							RequireClientCertificate: NewBoolValue(false),
							CommonTlsContext: &envoy_api_v2_auth.CommonTlsContext{
								TlsCertificateSdsSecretConfigs: []*envoy_api_v2_auth.SdsSecretConfig{
									{
										Name: "mosn-first-cert",
									},
								},
								ValidationContextType: &envoy_api_v2_auth.CommonTlsContext_CombinedValidationContext{
									CombinedValidationContext: &envoy_api_v2_auth.CommonTlsContext_CombinedCertificateValidationContext{
										ValidationContextSdsSecretConfig: &envoy_api_v2_auth.SdsSecretConfig{
											Name: "mosn-first-ca",
										},
									},
								},
							},
						},
						Filters: []*xdslistener.Filter{
							{
								Name: tt.args.filterName,
								ConfigType: &xdslistener.Filter_TypedConfig{
									conf,
								},
							},
						},
					},
					{
						FilterChainMatch: nil,
						Filters:          nil,
						TlsContext: &envoy_api_v2_auth.DownstreamTlsContext{
							RequireClientCertificate: NewBoolValue(true),
							CommonTlsContext: &envoy_api_v2_auth.CommonTlsContext{
								TlsCertificateSdsSecretConfigs: []*envoy_api_v2_auth.SdsSecretConfig{
									{
										Name: "mosn-second-cert",
									},
								},
								ValidationContextType: &envoy_api_v2_auth.CommonTlsContext_CombinedValidationContext{
									CombinedValidationContext: &envoy_api_v2_auth.CommonTlsContext_CombinedCertificateValidationContext{
										ValidationContextSdsSecretConfig: &envoy_api_v2_auth.SdsSecretConfig{
											Name: "mosn-second-ca",
										},
									},
								},
							},
						},
					},
				},
				DeprecatedV1: &xdsapi.Listener_DeprecatedV1{
					BindToPort: NewBoolValue(false),
				},
				DrainType: xdsapi.Listener_DEFAULT,
				ListenerFilters: []*xdslistener.ListenerFilter{
					{
						Name:       "original_dst",
						ConfigType: &xdslistener.ListenerFilter_TypedConfig{},
					},
				},
			}

			tt.want = `{"name":"0.0.0.0_80","address":"0.0.0.0:80", "access_logs":[{"log_path":"/dev/stdout"}],"listener_filters":[{"type":"original_dst"}],"filter_chains":[{"match":"\u003cnil\u003e","tls_context_set":[{"status": true,"verify_client":false,"sds_source":{"CertificateConfig":{"name":"mosn-first-cert"},"ValidationConfig":{"name":"mosn-first-ca"}}},{"status": true,"verify_client":true,"sds_source":{"CertificateConfig":{"name":"mosn-second-cert"},"ValidationConfig":{"name":"mosn-second-ca"}}}],"filters":[{"type":"proxy","config":{"downstream_protocol":"Http1","router_config_name":"80","upstream_protocol":"Http1"}},{"type":"connection_manager","config":{"router_config_name":"80","virtual_hosts":[{"domains":["istio-egressgateway.istio-system.svc.cluster.local","istio-egressgateway.istio-system.svc.cluster.local:80","istio-egressgateway.istio-system","istio-egressgateway.istio-system:80","istio-egressgateway.istio-system.svc.cluster","istio-egressgateway.istio-system.svc.cluster:80","istio-egressgateway.istio-system.svc","istio-egressgateway.istio-system.svc:80","172.19.3.204","172.19.3.204:80"],"name":"istio-egressgateway.istio-system.svc.cluster.local:80","routers":[{"match":{"prefix":"/"},"route":{"cluster_name":"outbound|80||istio-egressgateway.istio-system.svc.cluster.local","retry_policy":{"retry_timeout":"0s"},"timeout":"0s"}}]},{"domains":["istio-ingressgateway.istio-system.svc.cluster.local","istio-ingressgateway.istio-system.svc.cluster.local:80","istio-ingressgateway.istio-system","istio-ingressgateway.istio-system:80","istio-ingressgateway.istio-system.svc.cluster","istio-ingressgateway.istio-system.svc.cluster:80","istio-ingressgateway.istio-system.svc","istio-ingressgateway.istio-system.svc:80","172.19.8.101","172.19.8.101:80"],"name":"istio-ingressgateway.istio-system.svc.cluster.local:80","routers":[{"match":{"prefix":"/"},"route":{"cluster_name":"outbound|80||istio-ingressgateway.istio-system.svc.cluster.local","retry_policy":{"retry_timeout":"0s"},"timeout":"0s"}}]},{"domains":["nginx-ingress-lb.kube-system.svc.cluster.local","nginx-ingress-lb.kube-system.svc.cluster.local:80","nginx-ingress-lb.kube-system","nginx-ingress-lb.kube-system:80","nginx-ingress-lb.kube-system.svc.cluster","nginx-ingress-lb.kube-system.svc.cluster:80","nginx-ingress-lb.kube-system.svc","nginx-ingress-lb.kube-system.svc:80","172.19.6.192:80","172.19.8.101:80"],"name":"nginx-ingress-lb.kube-system.svc.cluster.local:80","routers":[{"match":{"prefix":"/"},"route":{"cluster_name":"outbound|80||nginx-ingress-lb.kube-system.svc.cluster.local","retry_policy":{"retry_timeout":"0s"},"timeout":"0s"}}]}]}}]}],"inspector":true}`
			got := ConvertListenerConfig(listenerConfig)
			want := &v2.Listener{}
			err := json.Unmarshal([]byte(tt.want), want)
			if err != nil {
				t.Errorf("json.Unmarshal([]byte(tt.want) got error: %v", err)
			}

			if !reflect.DeepEqual(want.ListenerConfig.ListenerFilters, got.ListenerConfig.ListenerFilters) {
				t.Errorf("convertListenerConfig(xdsListener *xdsapi.Listener)\ngot=%+v\nwant=%+v\n", got.ListenerConfig.ListenerFilters, want.ListenerConfig.ListenerFilters)
			}
		})
	}

}

func Test_convertCidrRange(t *testing.T) {
	type args struct {
		cidr []*xdscore.CidrRange
	}
	tests := []struct {
		name string
		args args
		want []v2.CidrRange
	}{
		{
			name: "case1",
			args: args{
				cidr: []*xdscore.CidrRange{
					{
						AddressPrefix: "192.168.1.1",
						PrefixLen:     &wrappers.UInt32Value{Value: 32},
					},
				},
			},
			want: []v2.CidrRange{
				{
					Address: "192.168.1.1",
					Length:  32,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertCidrRange(tt.args.cidr); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertCidrRange(cidr []*xdscore.CidrRange) = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_convertTCPRoute(t *testing.T) {
	type args struct {
		deprecatedV1 *xdstcp.TcpProxy_DeprecatedV1
	}
	tests := []struct {
		name string
		args args
		want []*v2.StreamRoute
	}{
		{
			name: "case1",
			args: args{
				deprecatedV1: &xdstcp.TcpProxy_DeprecatedV1{
					Routes: []*xdstcp.TcpProxy_DeprecatedV1_TCPRoute{
						{
							Cluster: "tcp",
							DestinationIpList: []*xdscore.CidrRange{
								{
									AddressPrefix: "192.168.1.1",
									PrefixLen:     &wrappers.UInt32Value{Value: 32},
								},
							},
							DestinationPorts: "50",
							SourceIpList: []*xdscore.CidrRange{
								{
									AddressPrefix: "192.168.1.2",
									PrefixLen:     &wrappers.UInt32Value{Value: 32},
								},
							},
							SourcePorts: "40",
						},
					},
				},
			},
			want: []*v2.StreamRoute{
				{
					Cluster: "tcp",
					DestinationAddrs: []v2.CidrRange{
						{
							Address: "192.168.1.1",
							Length:  32,
						},
					},
					DestinationPort: "50",
					SourceAddrs: []v2.CidrRange{
						{
							Address: "192.168.1.2",
							Length:  32,
						},
					},
					SourcePort: "40",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//if got := convertTCPRoute(tt.args.deprecatedV1); !reflect.DeepEqual(got, tt.want) {
			//	t.Errorf("convertTCPRoute(deprecatedV1 *xdstcp.TcpProxy_DeprecatedV1) = %v, want %v", got, tt.want)
			//}
		})
	}
}

func Test_convertHeadersToAdd(t *testing.T) {
	type args struct {
		headerValueOption []*xdscore.HeaderValueOption
	}

	FALSE := false

	tests := []struct {
		name string
		args args
		want []*v2.HeaderValueOption
	}{
		{
			name: "case1",
			args: args{
				headerValueOption: []*xdscore.HeaderValueOption{
					{
						Header: &xdscore.HeaderValue{
							Key:   "namespace",
							Value: "demo",
						},
						Append: &wrappers.BoolValue{Value: false},
					},
				},
			},
			want: []*v2.HeaderValueOption{
				{
					Header: &v2.HeaderValue{
						Key:   "namespace",
						Value: "demo",
					},
					Append: &FALSE,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertHeadersToAdd(tt.args.headerValueOption); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertHeadersToAdd(headerValueOption []*xdscore.HeaderValueOption) = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_convertRedirectAction(t *testing.T) {
	testCases := []struct {
		name     string
		in       *xdsroute.RedirectAction
		expected *v2.RedirectAction
	}{
		{
			name: "host redirect",
			in: &xdsroute.RedirectAction{
				HostRedirect: "example.com",
			},
			expected: &v2.RedirectAction{
				HostRedirect: "example.com",
			},
		},
		{
			name: "path redirect",
			in: &xdsroute.RedirectAction{
				PathRewriteSpecifier: &xdsroute.RedirectAction_PathRedirect{
					PathRedirect: "/foo",
				},
			},
			expected: &v2.RedirectAction{
				PathRedirect: "/foo",
			},
		},
		{
			name: "scheme redirect",
			in: &xdsroute.RedirectAction{
				SchemeRewriteSpecifier: &xdsroute.RedirectAction_SchemeRedirect{
					SchemeRedirect: "https",
				},
			},
			expected: &v2.RedirectAction{
				SchemeRedirect: "https",
			},
		},
		{
			name: "set redirect code",
			in: &xdsroute.RedirectAction{
				ResponseCode: http.StatusTemporaryRedirect,
				HostRedirect: "example.com",
			},
			expected: &v2.RedirectAction{
				HostRedirect: "example.com",
				ResponseCode: http.StatusTemporaryRedirect,
			},
		},
	}
	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			got := convertRedirectAction(tc.in)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("Unexpected redirect action\nExpected: %#v\nGot: %#v\n", tc.expected, got)
			}
		})
	}
}

func Test_convertDirectResponseAction(t *testing.T) {
	testCases := []struct {
		name     string
		in       *xdsroute.DirectResponseAction
		expected *v2.DirectResponseAction
	}{
		{
			name: "directResponse with body",
			in: &xdsroute.DirectResponseAction{
				Status: 200,
				Body: &core.DataSource{
					Specifier: &core.DataSource_InlineString{
						InlineString: "directResponse with body",
					},
				},
			},
			expected: &v2.DirectResponseAction{
				StatusCode: 200,
				Body:       "directResponse with body",
			},
		},
		{
			name: "directResponse no body",
			in: &xdsroute.DirectResponseAction{
				Status: 200,
				Body: &core.DataSource{
					Specifier: &core.DataSource_InlineString{
						InlineString: "",
					},
				},
			},
			expected: &v2.DirectResponseAction{
				StatusCode: 200,
				Body:       "",
			},
		},
	}
	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			got := convertDirectResponseAction(tc.in)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("Unexpected directResponse action\nExpected: %#v\nGot: %#v\n", tc.expected, got)
			}
		})
	}
}

// Test stream filters convert for envoy.fault
// Test stream filters convert for envoy.fault
func Test_convertStreamFilter_IsitoFault(t *testing.T) {
	faultInjectConfig := &xdshttpfault.HTTPFault{
		Delay: &xdsfault.FaultDelay{
			Percentage: &xdstype.FractionalPercent{
				Numerator:   100,
				Denominator: xdstype.FractionalPercent_HUNDRED,
			},
			FaultDelaySecifier: &xdsfault.FaultDelay_FixedDelay{FixedDelay: &duration.Duration{Seconds: 0}},
		},
		Abort: &xdshttpfault.FaultAbort{
			Percentage: &xdstype.FractionalPercent{
				Numerator:   100,
				Denominator: xdstype.FractionalPercent_HUNDRED,
			},
			ErrorType: &xdshttpfault.FaultAbort_HttpStatus{
				HttpStatus: 500,
			},
		},
		UpstreamCluster: "testupstream",
		Headers: []*xdsroute.HeaderMatcher{
			{
				Name: "end-user",
				HeaderMatchSpecifier: &xdsroute.HeaderMatcher_ExactMatch{
					ExactMatch: "jason",
				},
				InvertMatch: false,
			},
		},
	}

	faultStruct := messageToAny(t, faultInjectConfig)
	// empty types.Struct will makes a default empty filter
	testCases := []struct {
		config   *any.Any
		expected *v2.StreamFaultInject
	}{
		{
			config: faultStruct,
			expected: &v2.StreamFaultInject{
				Delay: &v2.DelayInject{
					Delay: 0,
					DelayInjectConfig: v2.DelayInjectConfig{
						Percent: 100,
					},
				},
				Abort: &v2.AbortInject{
					Status:  500,
					Percent: 100,
				},
				Headers: []v2.HeaderMatcher{
					{
						Name:  "end-user",
						Value: "jason",
						Regex: false,
					},
				},
				UpstreamCluster: "testupstream",
			},
		},
		{
			config:   nil,
			expected: &v2.StreamFaultInject{},
		},
	}
	for i, tc := range testCases {
		convertFilter := convertStreamFilter(v2.FaultStream, tc.config)
		if convertFilter.Type != v2.FaultStream {
			t.Errorf("#%d convert to mosn stream filter not expected, want %s, got %s", i, v2.FaultStream, convertFilter.Type)
			continue
		}
		rawFault := &v2.StreamFaultInject{}
		b, _ := json.Marshal(convertFilter.Config)
		if err := json.Unmarshal(b, rawFault); err != nil {
			t.Errorf("#%d unexpected config for fault", i)
			continue
		}
		if tc.expected.Abort == nil {
			if rawFault.Abort != nil {
				t.Errorf("#%d abort check unexpected", i)
			}
		} else {
			if rawFault.Abort.Status != tc.expected.Abort.Status || rawFault.Abort.Percent != tc.expected.Abort.Percent {
				t.Errorf("#%d abort check unexpected", i)
			}
		}
		if tc.expected.Delay == nil {
			if rawFault.Delay != nil {
				t.Errorf("#%d delay check unexpected", i)
			}
		} else {
			if rawFault.Delay.Delay != tc.expected.Delay.Delay || rawFault.Delay.Percent != tc.expected.Delay.Percent {
				t.Errorf("#%d delay check unexpected", i)
			}
		}
		if rawFault.UpstreamCluster != tc.expected.UpstreamCluster || !reflect.DeepEqual(rawFault.Headers, tc.expected.Headers) {
			t.Errorf("#%d fault config is not expected, %v", i, rawFault)
		}

	}
}

func Test_convertPerRouteConfig(t *testing.T) {
	mixerFilterConfig := &client.HttpClientConfig{
		ServiceConfigs: map[string]*client.ServiceConfig{v2.MIXER: &client.ServiceConfig{
			MixerAttributes: &v1.Attributes{
				Attributes: map[string]*v1.Attributes_AttributeValue{
					"test": &v1.Attributes_AttributeValue{
						Value: &v1.Attributes_AttributeValue_StringValue{
							StringValue: "test_value",
						},
					},
				},
			},
		},
		},
	}
	mixerStruct := messageToAny(t, mixerFilterConfig)
	fixedDelay := duration.Duration{Seconds: 1}
	faultInjectConfig := &xdshttpfault.HTTPFault{
		Delay: &xdsfault.FaultDelay{
			Percentage: &xdstype.FractionalPercent{
				Numerator:   100,
				Denominator: xdstype.FractionalPercent_HUNDRED,
			},
			FaultDelaySecifier: &xdsfault.FaultDelay_FixedDelay{
				FixedDelay: &fixedDelay,
			},
		},
		Abort: &xdshttpfault.FaultAbort{
			Percentage: &xdstype.FractionalPercent{
				Numerator:   100,
				Denominator: xdstype.FractionalPercent_HUNDRED,
			},
			ErrorType: &xdshttpfault.FaultAbort_HttpStatus{
				HttpStatus: 500,
			},
		},
		UpstreamCluster: "testupstream",
		Headers: []*xdsroute.HeaderMatcher{
			{
				Name: "end-user",
				HeaderMatchSpecifier: &xdsroute.HeaderMatcher_ExactMatch{
					ExactMatch: "jason",
				},
				InvertMatch: false,
			},
		},
	}
	faultStruct := messageToAny(t, faultInjectConfig)
	configs := map[string]*any.Any{
		v2.MIXER:       mixerStruct,
		v2.FaultStream: faultStruct,
	}
	perRouteConfig := convertPerRouteConfig(configs)
	if len(perRouteConfig) != 2 {
		t.Fatalf("want to get %d configs, but got %d", 2, len(perRouteConfig))
	}
	// verify
	if mixerPer, ok := perRouteConfig[v2.MIXER]; !ok {
		t.Error("no mixer config found")
	} else {
		res := client.HttpClientConfig{}
		jsonStr, err := json.Marshal(mixerPer.(map[string]interface{}))
		if err != nil {
			t.Errorf("mixer Marshal err: %v", err)
		}

		err = json.Unmarshal([]byte(jsonStr), &res)
		if err != nil {
			t.Errorf("mixer Unmarshal err: %v", err)
		}

		if !reflect.DeepEqual(&res, mixerFilterConfig) {
			t.Error("mixer config is not expected")
		}
	}
	if faultPer, ok := perRouteConfig[v2.FaultStream]; !ok {
		t.Error("no fault inject config found")
	} else {
		b, err := json.Marshal(faultPer)
		if err != nil {
			t.Fatal("marshal fault inject config failed")
		}
		conf := make(map[string]interface{})
		json.Unmarshal(b, &conf)
		rawFault, err := faultinject.ParseStreamFaultInjectFilter(conf)
		if err != nil {
			t.Fatal("fault config is not expected")
		}
		expectedHeader := v2.HeaderMatcher{
			Name:  "end-user",
			Value: "jason",
			Regex: false,
		}
		if !(rawFault.Abort.Status == 500 &&
			rawFault.Abort.Percent == 100 &&
			rawFault.Delay.Delay == time.Second &&
			rawFault.Delay.Percent == 100 &&
			rawFault.UpstreamCluster == "testupstream" &&
			len(rawFault.Headers) == 1 &&
			reflect.DeepEqual(rawFault.Headers[0], expectedHeader)) {
			t.Errorf("fault config is not expected, %+v, %+v, %v", rawFault.Abort, rawFault.Delay, rawFault)
		}

	}
}

func Test_convertHashPolicy(t *testing.T) {
	xdsHashPolicy := []*xdsroute.RouteAction_HashPolicy{
		{
			PolicySpecifier: &xdsroute.RouteAction_HashPolicy_Header_{
				Header: &xdsroute.RouteAction_HashPolicy_Header{
					HeaderName: "header_name",
				},
			},
		},
	}

	hp := convertHashPolicy(xdsHashPolicy)
	if !assert.NotNilf(t, hp[0].Header, "hashPolicy header field should not be nil") {
		t.FailNow()
	}
	if !assert.Equalf(t, "header_name", hp[0].Header.Key,
		"hashPolicy header field should be 'header_name'") {
		t.FailNow()
	}

	xdsHashPolicy = []*xdsroute.RouteAction_HashPolicy{
		{
			PolicySpecifier: &xdsroute.RouteAction_HashPolicy_Cookie_{
				Cookie: &xdsroute.RouteAction_HashPolicy_Cookie{
					Name: "cookie_name",
					Path: "cookie_path",
					Ttl: &duration.Duration{
						Seconds: 5,
						Nanos:   0,
					},
				},
			},
		},
	}

	hp = convertHashPolicy(xdsHashPolicy)
	if !assert.NotNilf(t, hp[0].Cookie, "hashPolicy Cookie field should not be nil") {
		t.FailNow()
	}
	if !assert.Equalf(t, "cookie_name", hp[0].Cookie.Name,
		"hashPolicy Cookie Name field should be 'cookie_name'") {
		t.FailNow()
	}
	if !assert.Equalf(t, "cookie_path", hp[0].Cookie.Path,
		"hashPolicy Cookie Path field should be 'cookie_path'") {
		t.FailNow()
	}
	if !assert.Equalf(t, 5*time.Second, hp[0].Cookie.TTL.Duration,
		"hashPolicy Cookie TTL field should be '5s'") {
		t.FailNow()
	}

	xdsHashPolicy = []*xdsroute.RouteAction_HashPolicy{
		{
			PolicySpecifier: &xdsroute.RouteAction_HashPolicy_ConnectionProperties_{
				ConnectionProperties: &xdsroute.RouteAction_HashPolicy_ConnectionProperties{
					SourceIp: true,
				},
			},
		},
	}
	hp = convertHashPolicy(xdsHashPolicy)
	if !assert.NotNilf(t, hp[0].SourceIP, "hashPolicy SourceIP field should not be nil") {
		t.FailNow()
	}
}

// Test stream filters convert for envoy.gzip
func Test_convertStreamFilter_Gzip(t *testing.T) {
	gzipConfig := &xdshttpgzip.Gzip{
		ContentLength:    &wrappers.UInt32Value{Value: 1024},
		CompressionLevel: xdshttpgzip.Gzip_CompressionLevel_BEST,
		ContentType:      []string{"test"},
	}

	gzipStruct := messageToAny(t, gzipConfig)
	// empty types.Struct will makes a default empty filter
	testCases := []struct {
		config   *any.Any
		expected *v2.StreamGzip
	}{
		{
			config: gzipStruct,
			expected: &v2.StreamGzip{
				GzipLevel:     fasthttp.CompressBestCompression,
				ContentLength: 1024,
				ContentType:   []string{"test"},
			},
		},
	}
	for i, tc := range testCases {
		convertFilter := convertStreamFilter(xdswellknown.Gzip, tc.config)
		if convertFilter.Type != v2.Gzip {
			t.Errorf("#%d convert to mosn stream filter not expected, want %s, got %s", i, v2.Gzip, convertFilter.Type)
			continue
		}
		rawGzip := &v2.StreamGzip{}
		b, _ := json.Marshal(convertFilter.Config)
		if err := json.Unmarshal(b, rawGzip); err != nil {
			t.Errorf("#%d unexpected config for fault", i)
			continue
		}

		if tc.expected.GzipLevel != rawGzip.GzipLevel {
			t.Errorf("#%d GzipLevel check unexpected", i)
		}
		if tc.expected.ContentLength != rawGzip.ContentLength {
			t.Errorf("#%d ContentLength check unexpected", i)
		}
		for k, _ := range tc.expected.ContentType {
			if tc.expected.ContentType[k] != rawGzip.ContentType[k] {
				t.Errorf("#%d ContentType check unexpected", i)
			}
		}
	}
}
