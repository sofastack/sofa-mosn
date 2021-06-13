package extract

import (
	"context"
	"encoding/base64"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"mosn.io/api"
	"mosn.io/mosn/pkg/istio/utils"
	v1 "mosn.io/mosn/pkg/mixer/v1"
	"mosn.io/mosn/pkg/protocol"
	mtypes "mosn.io/mosn/pkg/types"
	"mosn.io/mosn/pkg/variable"
	"mosn.io/pkg/buffer"
)

func Test_paresClusterName(t *testing.T) {
	type args struct {
		clusterName string
	}
	tests := []struct {
		name string
		args args
		want *clusterNameInfo
	}{
		{
			args: args{""},
			want: &clusterNameInfo{Kind: "", Version: "", Name: "", Namespace: "", Host: ""},
		},
		{
			args: args{"outbound|12220|0.0.1|hellomeshfacade-poc-test-client.default.svc.cluster.local"},
			want: &clusterNameInfo{Kind: "outbound", Version: "0.0.1", Name: "hellomeshfacade-poc-test-client", Namespace: "default", Host: "hellomeshfacade-poc-test-client.default.svc.cluster.local"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := paresClusterName(tt.args.clusterName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("paresClusterName() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestExtractAttributes(t *testing.T) {
	now := time.Now()
	buf := buffer.GetIoBuffer(1)
	buf.Reset()
	buf.Append([]byte{1})
	defer buffer.PutIoBuffer(buf)
	type args struct {
		varCtx      context.Context
		reqHeaders  api.HeaderMap
		respHeaders api.HeaderMap
		requestInfo api.RequestInfo
		buf         buffer.IoBuffer
		trailers    api.HeaderMap
	}

	ctx1 := variable.NewVariableContext(context.Background())
	variable.SetVariableValue(ctx1, mtypes.VarPath, "/path")
	variable.SetVariableValue(ctx1, mtypes.VarHost, "host")
	variable.SetVariableValue(ctx1, mtypes.VarMethod, "GET")

	ctx2 := variable.NewVariableContext(context.Background())
	variable.SetVariableValue(ctx2, mtypes.VarPath, "/path")
	variable.SetVariableValue(ctx2, mtypes.VarHost, "host")
	variable.SetVariableValue(ctx2, mtypes.VarMethod, "GET")
	variable.SetVariableValue(ctx2, mtypes.VarQueryString, "k1=v1&k2=v2")

	tests := []struct {
		name string
		args args
		want map[string]interface{}
	}{
		{
			args: args{
				varCtx:      variable.NewVariableContext(context.Background()),
				reqHeaders:  protocol.CommonHeader{},
				respHeaders: protocol.CommonHeader{},
				requestInfo: &MockRequestInfo{
					startTime: now,
					endTime:   now,
				},
				buf: buf,
			},
			want: map[string]interface{}{"context.protocol": "http", "request.size": int64(0), "request.time": now, "request.total_size": int64(1), "response.code": int64(0), "response.duration": time.Duration(0), "response.headers": protocol.CommonHeader{}, "response.size": int64(0), "response.total_size": int64(0), "response.time": now},
		},
		{
			args: args{
				varCtx:      variable.NewVariableContext(context.Background()),
				reqHeaders:  protocol.CommonHeader{},
				respHeaders: protocol.CommonHeader{},
				requestInfo: &MockRequestInfo{
					startTime:              now,
					endTime:                now,
					downstreamLocalAddress: &net.TCPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 80},
					upstreamHost: &MockHostInfo{
						addressString: "10.0.0.2:80",
					},
				},
				buf: buf,
			},
			want: map[string]interface{}{"context.protocol": "http", "request.size": int64(0), "request.time": now, "request.total_size": int64(1), "response.code": int64(0), "response.duration": time.Duration(0), "response.headers": protocol.CommonHeader{}, "response.size": int64(0), "response.total_size": int64(0), "response.time": now, "destination.ip": net.IPv4(10, 0, 0, 2), "destination.port": int64(80), "origin.ip": net.IPv4(10, 0, 0, 1)},
		},
		{
			args: args{
				varCtx:      variable.NewVariableContext(context.Background()),
				reqHeaders:  protocol.CommonHeader{},
				respHeaders: protocol.CommonHeader{},
				requestInfo: &MockRequestInfo{
					startTime: now,
					endTime:   now,
					protocol:  protocol.HTTP1,
				},
				buf: buf,
			},
			want: map[string]interface{}{"context.protocol": "http", "request.size": int64(0), "request.time": now, "request.total_size": int64(1), "response.code": int64(0), "response.duration": time.Duration(0), "response.headers": protocol.CommonHeader{}, "response.size": int64(0), "response.total_size": int64(0), "response.time": now},
		},
		{
			args: args{
				varCtx:      variable.NewVariableContext(context.Background()),
				reqHeaders:  protocol.CommonHeader{},
				respHeaders: protocol.CommonHeader{},
				requestInfo: &MockRequestInfo{
					startTime: now,
					endTime:   now,
					protocol:  protocol.HTTP2,
				},
				buf: buf,
			},
			want: map[string]interface{}{"context.protocol": "h2", "request.size": int64(0), "request.time": now, "request.total_size": int64(1), "response.code": int64(0), "response.duration": time.Duration(0), "response.headers": protocol.CommonHeader{}, "response.size": int64(0), "response.total_size": int64(0), "response.time": now},
		},
		{
			args: args{
				varCtx:      variable.NewVariableContext(context.Background()),
				reqHeaders:  protocol.CommonHeader{},
				respHeaders: protocol.CommonHeader{},
				requestInfo: &MockRequestInfo{
					startTime: now,
					endTime:   now,
					protocol:  protocol.Auto,
				},
				buf: buf,
			},
			want: map[string]interface{}{"context.protocol": string(protocol.Auto), "request.size": int64(0), "request.time": now, "request.total_size": int64(1), "response.code": int64(0), "response.duration": time.Duration(0), "response.headers": protocol.CommonHeader{}, "response.size": int64(0), "response.total_size": int64(0), "response.time": now},
		},
		{
			args: args{
				varCtx:      ctx1,
				reqHeaders:  protocol.CommonHeader{},
				respHeaders: protocol.CommonHeader{},
				requestInfo: &MockRequestInfo{
					startTime: now,
					endTime:   now,
					protocol:  protocol.Auto,
				},
				buf: buf,
			},
			want: map[string]interface{}{"context.protocol": string(protocol.Auto), "request.size": int64(0), "request.time": now, "request.total_size": int64(1), "response.code": int64(0), "response.duration": time.Duration(0), "response.headers": protocol.CommonHeader{}, "response.size": int64(0), "response.total_size": int64(0), "response.time": now, "request.host": "host", "request.path": "/path", "request.url_path": "/path", "request.method": "GET"},
		},
		{
			args: args{
				varCtx:      ctx2,
				reqHeaders:  protocol.CommonHeader{},
				respHeaders: protocol.CommonHeader{},
				requestInfo: &MockRequestInfo{
					startTime: now,
					endTime:   now,
					protocol:  protocol.Auto,
				},
				buf: buf,
			},
			want: map[string]interface{}{"context.protocol": string(protocol.Auto), "request.size": int64(0), "request.time": now, "request.total_size": int64(1), "response.code": int64(0), "response.duration": time.Duration(0), "response.headers": protocol.CommonHeader{}, "response.size": int64(0), "response.total_size": int64(0), "response.time": now, "request.host": "host", "request.path": "/path", "request.url_path": "/path?k1=v1&k2=v2", "request.method": "GET", "request.query_params": protocol.CommonHeader{"k1": "v1", "k2": "v2"}},
		},
		{
			args: args{
				varCtx: variable.NewVariableContext(context.Background()),
				reqHeaders: protocol.CommonHeader{
					utils.KIstioAttributeHeader: func() string {
						b, _ := proto.Marshal(&v1.Attributes{
							Attributes: map[string]*v1.Attributes_AttributeValue{
								"source.workload.name": {
									Value: &v1.Attributes_AttributeValue_StringValue{
										StringValue: "name",
									},
								},
							},
						})
						return base64.StdEncoding.EncodeToString(b)
					}(),
				},
				respHeaders: protocol.CommonHeader{},
				requestInfo: &MockRequestInfo{
					startTime: now,
					endTime:   now,
				},
				buf: buf,
			},
			want: map[string]interface{}{"context.protocol": "http", "request.size": int64(0), "request.time": now, "request.total_size": int64(63), "response.code": int64(0), "response.duration": time.Duration(0), "response.headers": protocol.CommonHeader{}, "response.size": int64(0), "response.total_size": int64(0), "response.time": now, "source.workload.name": "name"},
		},
		{
			args: args{
				varCtx: variable.NewVariableContext(context.Background()),
				reqHeaders: protocol.CommonHeader{
					utils.KIstioAttributeHeader: `Cj8KGGRlc3RpbmF0aW9uLnNlcnZpY2UuaG9zdBIjEiFodHRwYmluLmRlZmF1bHQuc3ZjLmNsdXN0ZXIubG9jYWwKPQoXZGVzdGluYXRpb24uc2VydmljZS51aWQSIhIgaXN0aW86Ly9kZWZhdWx0L3NlcnZpY2VzL2h0dHBiaW4KKgodZGVzdGluYXRpb24uc2VydmljZS5uYW1lc3BhY2USCRIHZGVmYXVsdAolChhkZXN0aW5hdGlvbi5zZXJ2aWNlLm5hbWUSCRIHaHR0cGJpbgo6Cgpzb3VyY2UudWlkEiwSKmt1YmVybmV0ZXM6Ly9zbGVlcC03YjlmOGJmY2QtMmRqeDUuZGVmYXVsdAo6ChNkZXN0aW5hdGlvbi5zZXJ2aWNlEiMSIWh0dHBiaW4uZGVmYXVsdC5zdmMuY2x1c3Rlci5sb2NhbA==`,
				},
				respHeaders: protocol.CommonHeader{},
				requestInfo: &MockRequestInfo{
					startTime: now,
					endTime:   now,
				},
				buf: buf,
			},
			want: map[string]interface{}{"context.protocol": "http", "request.size": int64(0), "request.time": now, "request.total_size": int64(463), "response.code": int64(0), "response.duration": time.Duration(0), "response.headers": protocol.CommonHeader{}, "response.size": int64(0), "response.total_size": int64(0), "response.time": now, "destination.service": "httpbin.default.svc.cluster.local", "destination.service.host": "httpbin.default.svc.cluster.local", "destination.service.name": "httpbin", "destination.service.namespace": "default", "destination.service.uid": "istio://default/services/httpbin", "source.uid": "kubernetes://sleep-7b9f8bfcd-2djx5.default"},
		},
		{
			args: args{
				varCtx: variable.NewVariableContext(context.Background()),
				reqHeaders: protocol.CommonHeader{
					utils.KIstioAttributeHeader: `Cj8KGGRlc3RpbmF0aW9uLnNlcnZpY2UuaG9zdBIjEiFodHRwYmluLmRlZmF1bHQuc3ZjLmNsdXN0ZXIubG9jYWwKPQoXZGVzdGluYXRpb24uc2VydmljZS51aWQSIhIgaXN0aW86Ly9kZWZhdWx0L3NlcnZpY2VzL2h0dHBiaW4KKgodZGVzdGluYXRpb24uc2VydmljZS5uYW1lc3BhY2USCRIHZGVmYXVsdAolChhkZXN0aW5hdGlvbi5zZXJ2aWNlLm5hbWUSCRIHaHR0cGJpbgo6Cgpzb3VyY2UudWlkEiwSKmt1YmVybmV0ZXM6Ly9zbGVlcC03YjlmOGJmY2QtMmRqeDUuZGVmYXVsdAo6ChNkZXN0aW5hdGlvbi5zZXJ2aWNlEiMSIWh0dHBiaW4uZGVmYXVsdC5zdmMuY2x1c3Rlci5sb2NhbA==`,
				},
				respHeaders: protocol.CommonHeader{},
				requestInfo: &MockRequestInfo{
					startTime: now,
					endTime:   now,
					routerRule: &MockRouteRule{
						clusterName: "outbound|12220|0.0.1|hellomeshfacade-poc-test-client.default2.svc.cluster.local",
					},
				},
				buf: buf,
			},
			want: map[string]interface{}{"context.protocol": "http", "request.size": int64(0), "request.time": now, "request.total_size": int64(463), "response.code": int64(0), "response.duration": time.Duration(0), "response.headers": protocol.CommonHeader{}, "response.size": int64(0), "response.total_size": int64(0), "response.time": now, "destination.service.host": "hellomeshfacade-poc-test-client.default2.svc.cluster.local", "destination.service.name": "hellomeshfacade-poc-test-client", "destination.service.namespace": "default2", "context.reporter.kind": "outbound"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractAttributes(tt.args.varCtx, tt.args.reqHeaders, tt.args.respHeaders, tt.args.requestInfo, tt.args.buf, tt.args.trailers, now)
			for k, v := range tt.want {
				if g, ok := got.Get(k); !ok || !reflect.DeepEqual(g, v) {
					t.Errorf("ExtractAttributes() key %q = %v, want %v", k, g, v)
				}
			}
		})
	}
}

func Test_attributesToStringInterfaceMap(t *testing.T) {
	type args struct {
		attributes v1.Attributes
	}
	tests := []struct {
		name string
		args args
		want map[string]interface{}
	}{
		{
			args: args{
				attributes: v1.Attributes{
					Attributes: map[string]*v1.Attributes_AttributeValue{
						"key": {
							Value: &v1.Attributes_AttributeValue_StringValue{
								StringValue: "string",
							},
						},
					},
				},
			},
			want: map[string]interface{}{
				"key": "string",
			},
		},
		{
			args: args{
				attributes: v1.Attributes{
					Attributes: map[string]*v1.Attributes_AttributeValue{
						"key": {
							Value: &v1.Attributes_AttributeValue_Int64Value{
								Int64Value: 1,
							},
						},
					},
				},
			},
			want: map[string]interface{}{
				"key": int64(1),
			},
		},
		{
			args: args{
				attributes: v1.Attributes{
					Attributes: map[string]*v1.Attributes_AttributeValue{
						"key": {
							Value: &v1.Attributes_AttributeValue_DoubleValue{
								DoubleValue: 1.1,
							},
						},
					},
				},
			},
			want: map[string]interface{}{
				"key": 1.1,
			},
		},
		{
			args: args{
				attributes: v1.Attributes{
					Attributes: map[string]*v1.Attributes_AttributeValue{
						"key": {
							Value: &v1.Attributes_AttributeValue_BoolValue{
								BoolValue: true,
							},
						},
					},
				},
			},
			want: map[string]interface{}{
				"key": true,
			},
		},
		{
			args: args{
				attributes: v1.Attributes{
					Attributes: map[string]*v1.Attributes_AttributeValue{
						"key": {
							Value: &v1.Attributes_AttributeValue_BytesValue{
								BytesValue: []byte{1},
							},
						},
					},
				},
			},
			want: map[string]interface{}{
				"key": []byte{1},
			},
		},
		{
			args: args{
				attributes: v1.Attributes{
					Attributes: map[string]*v1.Attributes_AttributeValue{
						"key": {
							Value: &v1.Attributes_AttributeValue_TimestampValue{
								TimestampValue: &types.Timestamp{Seconds: 1, Nanos: 2},
							},
						},
					},
				},
			},
			want: map[string]interface{}{
				"key": time.Unix(1, 2),
			},
		},
		{
			args: args{
				attributes: v1.Attributes{
					Attributes: map[string]*v1.Attributes_AttributeValue{
						"key": {
							Value: &v1.Attributes_AttributeValue_DurationValue{
								DurationValue: &types.Duration{Seconds: 1, Nanos: 2},
							},
						},
					},
				},
			},
			want: map[string]interface{}{
				"key": time.Second + 2,
			},
		},
		{
			args: args{
				attributes: v1.Attributes{
					Attributes: map[string]*v1.Attributes_AttributeValue{
						"key": {
							Value: &v1.Attributes_AttributeValue_StringMapValue{
								StringMapValue: &v1.Attributes_StringMap{
									Entries: map[string]string{
										"vk": "vv",
									},
								},
							},
						},
					},
				},
			},
			want: map[string]interface{}{
				"key": protocol.CommonHeader(map[string]string{
					"vk": "vv",
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := attributesToStringInterfaceMap(nil, tt.args.attributes); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("attributesToStringInterfaceMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

type MockRouteRule struct {
	vHost               api.VirtualHost
	clusterName         string
	upstreamProtocol    string
	globalTimeout       time.Duration
	policy              api.Policy
	perFilterConfig     map[string]interface{}
	pathMatchCriterion  api.PathMatchCriterion
	headerMatchCriteria api.KeyValueMatchCriteria
}

func (r *MockRouteRule) VirtualHost() api.VirtualHost {
	return r.vHost
}

func (r *MockRouteRule) ClusterName() string {
	return r.clusterName
}

func (r *MockRouteRule) UpstreamProtocol() string {
	return r.upstreamProtocol
}

func (r *MockRouteRule) GlobalTimeout() time.Duration {
	return r.globalTimeout
}

func (r *MockRouteRule) Policy() api.Policy {
	return r.policy
}

func (r *MockRouteRule) MetadataMatchCriteria(clusterName string) api.MetadataMatchCriteria {
	return nil
}

func (r *MockRouteRule) PerFilterConfig() map[string]interface{} {
	return r.perFilterConfig
}

func (r *MockRouteRule) FinalizeRequestHeaders(ctx context.Context, headers api.HeaderMap, requestInfo api.RequestInfo) {

}

func (r *MockRouteRule) FinalizeResponseHeaders(ctx context.Context, headers api.HeaderMap, requestInfo api.RequestInfo) {

}

func (r *MockRouteRule) PathMatchCriterion() api.PathMatchCriterion {
	return r.pathMatchCriterion
}

func (r *MockRouteRule) HeaderMatchCriteria() api.KeyValueMatchCriteria {
	return r.headerMatchCriteria
}

// MockRequestInfo
type MockRequestInfo struct {
	protocol                 api.ProtocolName
	startTime                time.Time
	endTime                  time.Time
	responseFlag             api.ResponseFlag
	upstreamHost             api.HostInfo
	requestReceivedDuration  time.Duration
	requestFinishedDuration  time.Duration
	responseReceivedDuration time.Duration
	processTimeDuration      time.Duration
	bytesSent                uint64
	bytesReceived            uint64
	responseCode             int
	localAddress             string
	downstreamLocalAddress   net.Addr
	downstreamRemoteAddress  net.Addr
	isHealthCheckRequest     bool
	routerRule               api.RouteRule
}

func (r *MockRequestInfo) StartTime() time.Time {
	return r.startTime
}

func (r *MockRequestInfo) SetStartTime() {
	r.startTime = time.Now()
}

func (r *MockRequestInfo) RequestReceivedDuration() time.Duration {
	return r.requestReceivedDuration
}

func (r *MockRequestInfo) SetRequestReceivedDuration(t time.Time) {
	r.requestReceivedDuration = t.Sub(r.startTime)
}

func (r *MockRequestInfo) ResponseReceivedDuration() time.Duration {
	return r.responseReceivedDuration
}

func (r *MockRequestInfo) SetResponseReceivedDuration(t time.Time) {
	r.responseReceivedDuration = t.Sub(r.startTime)
}

func (r *MockRequestInfo) RequestFinishedDuration() time.Duration {
	return r.requestFinishedDuration
}

func (r *MockRequestInfo) SetRequestFinishedDuration(t time.Time) {
	r.requestFinishedDuration = t.Sub(r.startTime)

}

func (r *MockRequestInfo) ProcessTimeDuration() time.Duration {
	return r.processTimeDuration
}

func (r *MockRequestInfo) SetProcessTimeDuration(d time.Duration) {
	r.processTimeDuration = d
}

func (r *MockRequestInfo) BytesSent() uint64 {
	return r.bytesSent
}

func (r *MockRequestInfo) SetBytesSent(bytesSent uint64) {
	r.bytesSent = bytesSent
}

func (r *MockRequestInfo) BytesReceived() uint64 {
	return r.bytesReceived
}

func (r *MockRequestInfo) SetBytesReceived(bytesReceived uint64) {
	r.bytesReceived = bytesReceived
}

func (r *MockRequestInfo) Protocol() api.ProtocolName {
	return r.protocol
}

func (r *MockRequestInfo) SetProtocol(p api.ProtocolName) {
	r.protocol = p
}

func (r *MockRequestInfo) ResponseCode() int {
	return r.responseCode
}

func (r *MockRequestInfo) SetResponseCode(code int) {
	r.responseCode = code
}

func (r *MockRequestInfo) Duration() time.Duration {
	return r.endTime.Sub(r.startTime)
}

func (r *MockRequestInfo) GetResponseFlag(flag api.ResponseFlag) bool {
	return r.responseFlag&flag != 0
}

func (r *MockRequestInfo) SetResponseFlag(flag api.ResponseFlag) {
	r.responseFlag |= flag
}

func (r *MockRequestInfo) UpstreamHost() api.HostInfo {
	return r.upstreamHost
}

func (r *MockRequestInfo) OnUpstreamHostSelected(host api.HostInfo) {
	r.upstreamHost = host
}

func (r *MockRequestInfo) UpstreamLocalAddress() string {
	return r.localAddress
}

func (r *MockRequestInfo) SetUpstreamLocalAddress(addr string) {
	r.localAddress = addr
}

func (r *MockRequestInfo) IsHealthCheck() bool {
	return r.isHealthCheckRequest
}

func (r *MockRequestInfo) SetHealthCheck(isHc bool) {
	r.isHealthCheckRequest = isHc
}

func (r *MockRequestInfo) DownstreamLocalAddress() net.Addr {
	return r.downstreamLocalAddress
}

func (r *MockRequestInfo) SetDownstreamLocalAddress(addr net.Addr) {
	r.downstreamLocalAddress = addr
}

func (r *MockRequestInfo) DownstreamRemoteAddress() net.Addr {
	return r.downstreamRemoteAddress
}

func (r *MockRequestInfo) SetDownstreamRemoteAddress(addr net.Addr) {
	r.downstreamRemoteAddress = addr
}

func (r *MockRequestInfo) RouteEntry() api.RouteRule {
	return r.routerRule
}

func (r *MockRequestInfo) SetRouteEntry(routerRule api.RouteRule) {
	r.routerRule = routerRule
}

type MockHostInfo struct {
	hostname      string
	metadata      api.Metadata
	addressString string
	weight        uint32
	supportTLS    bool
	healthFlag    api.HealthFlag
	health        bool
}

func (h *MockHostInfo) Hostname() string {
	return h.hostname
}

func (h *MockHostInfo) Metadata() api.Metadata {
	return h.metadata
}

func (h *MockHostInfo) AddressString() string {
	return h.addressString
}

func (h *MockHostInfo) Weight() uint32 {
	return h.weight
}

func (h *MockHostInfo) SupportTLS() bool {
	return h.supportTLS
}

func (h *MockHostInfo) ClearHealthFlag(flag api.HealthFlag) {

}

func (h *MockHostInfo) ContainHealthFlag(flag api.HealthFlag) bool {
	return false
}

func (h *MockHostInfo) SetHealthFlag(flag api.HealthFlag) {

}

func (h *MockHostInfo) HealthFlag() api.HealthFlag {
	return h.healthFlag
}

func (h *MockHostInfo) Health() bool {
	return h.health
}
