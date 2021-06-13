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

	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_config_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_extensions_filters_common_fault_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/common/fault/v3"
	envoy_extensions_filters_http_compressor_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/compressor/v3"
	envoy_extensions_filters_http_fault_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/fault/v3"
	envoy_extensions_filters_http_gzip_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/gzip/v3"
	envoy_extensions_filters_network_tcp_proxy_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	wellknown "github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/proto"
	ptypes "github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	v2 "mosn.io/mosn/pkg/config/v2"
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

func Test_convertHeaders(t *testing.T) {
	type args struct {
		xdsHeaders []*envoy_config_route_v3.HeaderMatcher
	}
	tests := []struct {
		name string
		args args
		want []v2.HeaderMatcher
	}{
		{
			name: "case1",
			args: args{
				xdsHeaders: []*envoy_config_route_v3.HeaderMatcher{
					{
						Name: "end-user",
						HeaderMatchSpecifier: &envoy_config_route_v3.HeaderMatcher_ExactMatch{
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
		// XXX_NoUnkeyedLiteral: struct{}{},
		// XXX_unrecognized:     nil,
		// XXX_sizecache:        0,
	}
}

func Test_convertCidrRange(t *testing.T) {
	type args struct {
		cidr []*envoy_config_core_v3.CidrRange
	}
	tests := []struct {
		name string
		args args
		want []v2.CidrRange
	}{
		{
			name: "case1",
			args: args{
				cidr: []*envoy_config_core_v3.CidrRange{
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
		deprecatedV1 *envoy_extensions_filters_network_tcp_proxy_v3.TcpProxy_DeprecatedV1
	}
	tests := []struct {
		name string
		args args
		want []*v2.StreamRoute
	}{
		{
			name: "case1",
			args: args{
				deprecatedV1: &envoy_extensions_filters_network_tcp_proxy_v3.TcpProxy_DeprecatedV1{
					Routes: []*envoy_extensions_filters_network_tcp_proxy_v3.TcpProxy_DeprecatedV1_TCPRoute{
						{
							Cluster: "tcp",
							DestinationIpList: []*envoy_config_core_v3.CidrRange{
								{
									AddressPrefix: "192.168.1.1",
									PrefixLen:     &wrappers.UInt32Value{Value: 32},
								},
							},
							DestinationPorts: "50",
							SourceIpList: []*envoy_config_core_v3.CidrRange{
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
		headerValueOption []*envoy_config_core_v3.HeaderValueOption
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
				headerValueOption: []*envoy_config_core_v3.HeaderValueOption{
					{
						Header: &envoy_config_core_v3.HeaderValue{
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
		in       *envoy_config_route_v3.RedirectAction
		expected *v2.RedirectAction
	}{
		{
			name: "host redirect",
			in: &envoy_config_route_v3.RedirectAction{
				HostRedirect: "example.com",
			},
			expected: &v2.RedirectAction{
				HostRedirect: "example.com",
			},
		},
		{
			name: "path redirect",
			in: &envoy_config_route_v3.RedirectAction{
				PathRewriteSpecifier: &envoy_config_route_v3.RedirectAction_PathRedirect{
					PathRedirect: "/foo",
				},
			},
			expected: &v2.RedirectAction{
				PathRedirect: "/foo",
			},
		},
		{
			name: "scheme redirect",
			in: &envoy_config_route_v3.RedirectAction{
				SchemeRewriteSpecifier: &envoy_config_route_v3.RedirectAction_SchemeRedirect{
					SchemeRedirect: "https",
				},
			},
			expected: &v2.RedirectAction{
				SchemeRedirect: "https",
			},
		},
		{
			name: "set redirect code",
			in: &envoy_config_route_v3.RedirectAction{
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
		in       *envoy_config_route_v3.DirectResponseAction
		expected *v2.DirectResponseAction
	}{
		{
			name: "directResponse with body",
			in: &envoy_config_route_v3.DirectResponseAction{
				Status: 200,
				Body: &envoy_config_core_v3.DataSource{
					Specifier: &envoy_config_core_v3.DataSource_InlineString{
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
			in: &envoy_config_route_v3.DirectResponseAction{
				Status: 200,
				Body: &envoy_config_core_v3.DataSource{
					Specifier: &envoy_config_core_v3.DataSource_InlineString{
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
	faultInjectConfig := &envoy_extensions_filters_http_fault_v3.HTTPFault{
		Delay: &envoy_extensions_filters_common_fault_v3.FaultDelay{
			Percentage: &envoy_type_v3.FractionalPercent{
				Numerator:   100,
				Denominator: envoy_type_v3.FractionalPercent_HUNDRED,
			},
			FaultDelaySecifier: &envoy_extensions_filters_common_fault_v3.FaultDelay_FixedDelay{FixedDelay: &duration.Duration{Seconds: 0}},
		},
		Abort: &envoy_extensions_filters_http_fault_v3.FaultAbort{
			Percentage: &envoy_type_v3.FractionalPercent{
				Numerator:   100,
				Denominator: envoy_type_v3.FractionalPercent_HUNDRED,
			},
			ErrorType: &envoy_extensions_filters_http_fault_v3.FaultAbort_HttpStatus{
				HttpStatus: 500,
			},
		},
		UpstreamCluster: "testupstream",
		Headers: []*envoy_config_route_v3.HeaderMatcher{
			{
				Name: "end-user",
				HeaderMatchSpecifier: &envoy_config_route_v3.HeaderMatcher_ExactMatch{
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
		convertFilter := convertStreamFilter(wellknown.Fault, tc.config)
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

func Test_convertHashPolicy(t *testing.T) {
	xdsHashPolicy := []*envoy_config_route_v3.RouteAction_HashPolicy{
		{
			PolicySpecifier: &envoy_config_route_v3.RouteAction_HashPolicy_Header_{
				Header: &envoy_config_route_v3.RouteAction_HashPolicy_Header{
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

	xdsHashPolicy = []*envoy_config_route_v3.RouteAction_HashPolicy{
		{
			PolicySpecifier: &envoy_config_route_v3.RouteAction_HashPolicy_Cookie_{
				Cookie: &envoy_config_route_v3.RouteAction_HashPolicy_Cookie{
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

	xdsHashPolicy = []*envoy_config_route_v3.RouteAction_HashPolicy{
		{
			PolicySpecifier: &envoy_config_route_v3.RouteAction_HashPolicy_ConnectionProperties_{
				ConnectionProperties: &envoy_config_route_v3.RouteAction_HashPolicy_ConnectionProperties{
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
	gzipConfig := &envoy_extensions_filters_http_gzip_v3.Gzip{
		CompressionLevel: envoy_extensions_filters_http_gzip_v3.Gzip_CompressionLevel_BEST,
		Compressor: &envoy_extensions_filters_http_compressor_v3.Compressor{
			ContentLength: &wrappers.UInt32Value{Value: 1024},
			ContentType:   []string{"test"},
		},
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
		convertFilter := convertStreamFilter(wellknown.Gzip, tc.config)
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
