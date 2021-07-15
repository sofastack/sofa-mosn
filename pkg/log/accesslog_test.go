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

package log

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"mosn.io/api"
	"mosn.io/mosn/pkg/types"
	"mosn.io/mosn/pkg/variable"
	"mosn.io/pkg/log"
)

func prepareLocalIpv6Ctx() context.Context {
	ctx := context.Background()
	ctx = variable.NewVariableContext(ctx)

	reqHeaders := map[string]string{
		"service": "test",
	}
	reqHeaderMap := newHeaderMap(reqHeaders)
	ctx = context.WithValue(ctx, requestHeaderMapKey, reqHeaderMap)

	respHeaders := map[string]string{
		"server": "MOSN",
	}
	respHeaderMap := newHeaderMap(respHeaders)
	ctx = context.WithValue(ctx, responseHeaderMapKey, respHeaderMap)

	requestInfo := newRequestInfo()
	requestInfo.SetRequestReceivedDuration(time.Now())
	requestInfo.SetResponseReceivedDuration(time.Now().Add(time.Second * 2))
	requestInfo.SetBytesSent(2048)
	requestInfo.SetBytesReceived(2048)

	requestInfo.SetResponseFlag(0)
	requestInfo.SetUpstreamLocalAddress("127.0.0.1:23456")
	requestInfo.SetDownstreamLocalAddress(&net.TCPAddr{net.ParseIP("2001:db8::68"), 12200, ""})
	requestInfo.SetDownstreamRemoteAddress(&net.TCPAddr{net.ParseIP("127.0.0.1"), 53242, ""})
	requestInfo.OnUpstreamHostSelected(nil)
	ctx = context.WithValue(ctx, requestInfoKey, requestInfo)

	return ctx
}

func TestAccessLog(t *testing.T) {
	registerTestVarDefs()

	format := types.DefaultAccessLogFormat
	logName := "/tmp/mosn_bench/benchmark_access.log"
	os.Remove(logName)
	accessLog, err := NewAccessLog(logName, format)

	if err != nil {
		t.Errorf(err.Error())
		return
	}

	ctx := prepareLocalIpv6Ctx()
	accessLog.Log(ctx, nil, nil, nil)
	//l := "2018/12/14 18:08:33.054 1.329µs 2.00000227s 2048 2048 - 0 126.868µs false 0 127.0.0.1:23456 [2001:db8::68]:12200 127.0.0.1:53242 -\n"
	time.Sleep(2 * time.Second)
	f, err := os.Open(logName)
	if err != nil {
		t.Errorf("open log file: %s error: %v", logName, err)
	}
	defer f.Close()
	b := make([]byte, 1024)
	_, err = f.Read(b)
	if err != nil {
		t.Errorf("read log file: %s, error: %v", logName, err)
	}
	if err != nil {
		t.Errorf("test accesslog error")
	}
	t.Logf("%v", string(b))
	ok, err := regexp.Match("\\d\\d\\d\\d/\\d\\d/\\d\\d .* .* .* 2048 2048 \\- 0 .* false 0 127.0.0.1:23456 \\[2001:db8::68\\]:12200 127.0.0.1:53242 \\-\n", b)

	if !ok {
		t.Errorf("test accesslog error %v", err)
	}
}

func TestAccessLogStartTime(t *testing.T) {
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond)
		now := time.Now()
		t.Log(now.Format("2006/01/02 15:04:05.999"))
		t.Log(now.Format("2006/01/02 15:04:05.000"))
	}
}

func TestAccessLogWithCustomText(t *testing.T) {
	registerTestVarDefs()

	format := "send request to upstream by local address %upstream_local_address%"
	logName := "/tmp/mosn_bench/test_access.log"
	os.Remove(logName)
	accessLog, err := NewAccessLog(logName, format)
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	ctx := prepareLocalIpv6Ctx()
	accessLog.Log(ctx, nil, nil, nil)
	time.Sleep(2 * time.Second)
	f, err := os.Open(logName)
	if err != nil {
		t.Error("open accesslog error ", err)
	}
	b := make([]byte, 1024)
	n, err := f.Read(b)
	f.Close()
	if err != nil {
		t.Error("read accesslog error ", err)
	}

	if string(b)[0:n] != "send request to upstream by local address 127.0.0.1:23456\n" {
		t.Error("test accesslog error")
	}
}

func TestAccessLogWithPrefixVariables(t *testing.T) {
	registerTestVarDefs()

	format := "request header:%request_header_service% response header:%response_header_server%"
	logName := "/tmp/mosn_bench/test_access_log.log"
	os.Remove(logName)
	accessLog, err := NewAccessLog(logName, format)
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	ctx := prepareLocalIpv6Ctx()
	accessLog.Log(ctx, nil, nil, nil)
	time.Sleep(2 * time.Second)
	f, err := os.Open(logName)
	if err != nil {
		t.Error("open accesslog error ", err)
	}
	b := make([]byte, 1024)
	n, err := f.Read(b)
	f.Close()
	if err != nil {
		t.Error("read accesslog error ", err)
	}

	if string(b)[0:n] != "request header:test response header:MOSN\n" {
		t.Errorf("test accesslog error %s", string(b)[0:n])
	}
}

func TestAccessLogWithEmptyVar(t *testing.T) {
	registerTestVarDefs()

	format := "send request to upstream by local address %%"
	logName := "/tmp/mosn_bench/benchmark_access.log"
	os.Remove(logName)
	_, err := NewAccessLog(logName, format)

	if err == nil || !strings.Contains(err.Error(), "empty variable definition") {
		t.Error("should return empty variable definition error but actually not")
		return
	}
}

func TestAccessLogWithUnclosedVar(t *testing.T) {
	registerTestVarDefs()

	format := "send request to upstream by local address %"
	logName := "/tmp/mosn_bench/benchmark_access.log"
	os.Remove(logName)
	_, err := NewAccessLog(logName, format)

	if err == nil || !strings.Contains(err.Error(), "unclosed variable definition") {
		t.Error("should return unclosed variable definition error but actually not")
		return
	}
}

func TestAccessLogDisable(t *testing.T) {
	registerTestVarDefs()

	DefaultDisableAccessLog = true
	format := types.DefaultAccessLogFormat
	logName := "/tmp/mosn_accesslog/disable_access.log"
	os.Remove(logName)
	accessLog, err := NewAccessLog(logName, format)
	if err != nil {
		t.Fatal(err)
	}

	ctx := prepareLocalIpv6Ctx()
	// try write disable access log nothing happened
	accessLog.Log(ctx, nil, nil, nil)
	time.Sleep(time.Second)
	if b, err := ioutil.ReadFile(logName); err != nil || len(b) > 0 {
		t.Fatalf("verify log file failed, data len: %d, error: %v", len(b), err)
	}
	// enable access log
	if !ToggleLogger(logName, false) {
		t.Fatal("enable access log failed")
	}
	// retry, write success
	accessLog.Log(ctx, nil, nil, nil)
	time.Sleep(time.Second)
	if b, err := ioutil.ReadFile(logName); err != nil || len(b) == 0 {
		t.Fatalf("verify log file failed, data len: %d, error: %v", len(b), err)
	}
}

func TestAccessLogManage(t *testing.T) {
	registerTestVarDefs()

	defer log.CloseAll()
	DefaultDisableAccessLog = false
	format := "%start_time% %response_flag%"
	var logs []api.AccessLog
	for i := 0; i < 100; i++ {
		logName := fmt.Sprintf("/tmp/accesslog.%d.log", i)
		lg, err := NewAccessLog(logName, format)
		if err != nil {
			t.Fatal(err)
		}
		logs = append(logs, lg)
	}
	DisableAllAccessLog()
	// new access log is auto disabled
	for i := 200; i < 300; i++ {
		logName := fmt.Sprintf("/tmp/accesslog.%d.log", i)
		lg, err := NewAccessLog(logName, format)
		if err != nil {
			t.Fatal(err)
		}
		logs = append(logs, lg)
	}
	// verify
	// all accesslog is disabled
	for _, lg := range logs {
		alg := lg.(*accesslog)
		if !alg.logger.Disable() {
			t.Fatal("some access log is enabled")
		}
	}

	EnableAllAccessLog()
	// verify all accesslog is enabled
	for _, lg := range logs {
		alg := lg.(*accesslog)
		if alg.logger.Disable() {
			t.Fatal("some access log is disabled")
		}
	}
}

func prepareLocalIpv4Ctx() context.Context {
	ctx := context.Background()
	ctx = variable.NewVariableContext(ctx)

	reqHeaders := map[string]string{
		"service": "test",
	}
	ctx = context.WithValue(ctx, requestHeaderMapKey, reqHeaders)
	reqHeaderMap := newHeaderMap(reqHeaders)
	ctx = context.WithValue(ctx, requestHeaderMapKey, reqHeaderMap)

	respHeaders := map[string]string{
		"server": "MOSN",
	}
	ctx = context.WithValue(ctx, responseHeaderMapKey, respHeaders)
	respHeaderMap := newHeaderMap(respHeaders)
	ctx = context.WithValue(ctx, responseHeaderMapKey, respHeaderMap)

	requestInfo := newRequestInfo()
	requestInfo.SetRequestReceivedDuration(time.Now())
	requestInfo.SetResponseReceivedDuration(time.Now().Add(time.Second * 2))
	requestInfo.SetBytesSent(2048)
	requestInfo.SetBytesReceived(2048)

	requestInfo.SetResponseFlag(0)
	requestInfo.SetUpstreamLocalAddress("127.0.0.1:23456")
	requestInfo.SetDownstreamLocalAddress(&net.TCPAddr{[]byte("127.0.0.1"), 12200, ""})
	requestInfo.SetDownstreamRemoteAddress(&net.TCPAddr{[]byte("127.0.0.2"), 53242, ""})
	requestInfo.OnUpstreamHostSelected(nil)
	ctx = context.WithValue(ctx, requestInfoKey, requestInfo)

	return ctx
}

func BenchmarkAccessLog(b *testing.B) {
	registerTestVarDefs()
	InitDefaultLogger("", log.INFO)
	// ~ replace the path if needed
	format := types.DefaultAccessLogFormat
	accessLog, err := NewAccessLog("/tmp/mosn_bench/benchmark_access.log", format)

	if err != nil {
		b.Error(err)
		return
	}

	ctx := prepareLocalIpv4Ctx()
	for n := 0; n < b.N; n++ {
		accessLog.Log(ctx, nil, nil, nil)
	}
}

func BenchmarkAccessLogParallel(b *testing.B) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	registerTestVarDefs()
	InitDefaultLogger("", log.INFO)
	// ~ replace the path if needed
	format := types.DefaultAccessLogFormat
	accessLog, err := NewAccessLog("/tmp/mosn_bench/benchmark_access.log", format)

	if err != nil {
		b.Errorf(err.Error())
	}
	ctx := prepareLocalIpv4Ctx()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			accessLog.Log(ctx, nil, nil, nil)
		}
	})
}

type mock_HeaderMap struct {
	headerMap map[string]string
}

func newHeaderMap(headers map[string]string) api.HeaderMap {
	return &mock_HeaderMap{
		headerMap: headers,
	}
}

func (h *mock_HeaderMap) Get(key string) (string, bool) {
	return h.headerMap[key], h.headerMap[key] != ""
}

func (h *mock_HeaderMap) Set(key, value string) {

}

func (h *mock_HeaderMap) Add(key, value string) {

}

func (h *mock_HeaderMap) Del(key string) {

}

func (h *mock_HeaderMap) Range(f func(key, value string) bool) {

}

func (h *mock_HeaderMap) Clone() api.HeaderMap {
	return h
}

func (h *mock_HeaderMap) ByteSize() uint64 {
	return 0
}

type mock_requestInfo struct {
	protocol                 api.ProtocolName
	startTime                time.Time
	responseFlag             api.ResponseFlag
	upstreamHost             api.HostInfo
	requestReceivedDuration  time.Duration
	responseReceivedDuration time.Duration
	requestFinishedDuration  time.Duration
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

// NewrequestInfo
func newRequestInfo() api.RequestInfo {
	return &mock_requestInfo{
		startTime: time.Now(),
	}
}

func (r *mock_requestInfo) StartTime() time.Time {
	return r.startTime
}

func (r *mock_requestInfo) SetStartTime() {
	r.startTime = time.Now()
}

func (r *mock_requestInfo) RequestReceivedDuration() time.Duration {
	return r.requestReceivedDuration
}

func (r *mock_requestInfo) SetRequestReceivedDuration(t time.Time) {
	r.requestReceivedDuration = t.Sub(r.startTime)
}

func (r *mock_requestInfo) ResponseReceivedDuration() time.Duration {
	return r.responseReceivedDuration
}

func (r *mock_requestInfo) SetResponseReceivedDuration(t time.Time) {
	r.responseReceivedDuration = t.Sub(r.startTime)
}

func (r *mock_requestInfo) RequestFinishedDuration() time.Duration {
	return r.requestFinishedDuration
}

func (r *mock_requestInfo) SetRequestFinishedDuration(t time.Time) {
	r.requestFinishedDuration = t.Sub(r.startTime)
}

func (r *mock_requestInfo) ProcessTimeDuration() time.Duration {
	return r.processTimeDuration
}

func (r *mock_requestInfo) SetProcessTimeDuration(d time.Duration) {
	r.processTimeDuration = d
}

func (r *mock_requestInfo) BytesSent() uint64 {
	return r.bytesSent
}

func (r *mock_requestInfo) SetBytesSent(bytesSent uint64) {
	r.bytesSent = bytesSent
}

func (r *mock_requestInfo) BytesReceived() uint64 {
	return r.bytesReceived
}

func (r *mock_requestInfo) SetBytesReceived(bytesReceived uint64) {
	r.bytesReceived = bytesReceived
}

func (r *mock_requestInfo) Protocol() api.ProtocolName {
	return r.protocol
}

func (r *mock_requestInfo) SetProtocol(p api.ProtocolName) {
	r.protocol = p
}

func (r *mock_requestInfo) ResponseCode() int {
	return r.responseCode
}

func (r *mock_requestInfo) SetResponseCode(code int) {
	r.responseCode = code
}

func (r *mock_requestInfo) Duration() time.Duration {
	return time.Now().Sub(r.startTime)
}

func (r *mock_requestInfo) GetResponseFlag(flag api.ResponseFlag) bool {
	return r.responseFlag&flag != 0
}

func (r *mock_requestInfo) SetResponseFlag(flag api.ResponseFlag) {
	r.responseFlag |= flag
}

func (r *mock_requestInfo) UpstreamHost() api.HostInfo {
	return r.upstreamHost
}

func (r *mock_requestInfo) OnUpstreamHostSelected(host api.HostInfo) {
	r.upstreamHost = host
}

func (r *mock_requestInfo) UpstreamLocalAddress() string {
	return r.localAddress
}

func (r *mock_requestInfo) SetUpstreamLocalAddress(addr string) {
	r.localAddress = addr
}

func (r *mock_requestInfo) IsHealthCheck() bool {
	return r.isHealthCheckRequest
}

func (r *mock_requestInfo) SetHealthCheck(isHc bool) {
	r.isHealthCheckRequest = isHc
}

func (r *mock_requestInfo) DownstreamLocalAddress() net.Addr {
	return r.downstreamLocalAddress
}

func (r *mock_requestInfo) SetDownstreamLocalAddress(addr net.Addr) {
	r.downstreamLocalAddress = addr
}

func (r *mock_requestInfo) DownstreamRemoteAddress() net.Addr {
	return r.downstreamRemoteAddress
}

func (r *mock_requestInfo) SetDownstreamRemoteAddress(addr net.Addr) {
	r.downstreamRemoteAddress = addr
}

func (r *mock_requestInfo) RouteEntry() api.RouteRule {
	return r.routerRule
}

func (r *mock_requestInfo) SetRouteEntry(routerRule api.RouteRule) {
	r.routerRule = routerRule
}

// The identification of a request info's content
const (
	requestInfoKey       = "requestInfo"
	requestHeaderMapKey  = "requestHeaderMap"
	responseHeaderMapKey = "responseHeaderMap"

	varStartTime                string = "start_time"
	varRequestReceivedDuration  string = "request_received_duration"
	varResponseReceivedDuration string = "response_received_duration"
	varRequestFinishedDuration  string = "request_finished_duration"
	varBytesSent                string = "bytes_sent"
	varBytesReceived            string = "bytes_received"
	varProtocol                 string = "protocol"
	varResponseCode             string = "response_code"
	varDuration                 string = "duration"
	varResponseFlag             string = "response_flag"
	varUpstreamLocalAddress     string = "upstream_local_address"
	varDownstreamLocalAddress   string = "downstream_local_address"
	varDownstreamRemoteAddress  string = "downstream_remote_address"
	varUpstreamHost             string = "upstream_host"

	// ReqHeaderPrefix is the prefix of request header's formatter
	reqHeaderPrefix string = "request_header_"
	reqHeaderIndex         = len(reqHeaderPrefix)
	// RespHeaderPrefix is the prefix of response header's formatter
	respHeaderPrefix string = "response_header_"
	respHeaderIndex         = len(respHeaderPrefix)
)

var (
	builtinVariables = []variable.Variable{
		variable.NewStringVariable(varStartTime, nil, startTimeGetter, nil, 0),
		variable.NewStringVariable(varRequestReceivedDuration, nil, receivedDurationGetter, nil, 0),
		variable.NewStringVariable(varResponseReceivedDuration, nil, responseReceivedDurationGetter, nil, 0),
		variable.NewStringVariable(varRequestFinishedDuration, nil, requestFinishedDurationGetter, nil, 0),
		variable.NewStringVariable(varBytesSent, nil, bytesSentGetter, nil, 0),
		variable.NewStringVariable(varBytesReceived, nil, bytesReceivedGetter, nil, 0),
		variable.NewStringVariable(varProtocol, nil, protocolGetter, nil, 0),
		variable.NewStringVariable(varResponseCode, nil, responseCodeGetter, nil, 0),
		variable.NewStringVariable(varDuration, nil, durationGetter, nil, 0),
		variable.NewStringVariable(varResponseFlag, nil, responseFlagGetter, nil, 0),
		variable.NewStringVariable(varUpstreamLocalAddress, nil, upstreamLocalAddressGetter, nil, 0),
		variable.NewStringVariable(varDownstreamLocalAddress, nil, downstreamLocalAddressGetter, nil, 0),
		variable.NewStringVariable(varDownstreamRemoteAddress, nil, downstreamRemoteAddressGetter, nil, 0),
		variable.NewStringVariable(varUpstreamHost, nil, upstreamHostGetter, nil, 0),
	}

	prefixVariables = []variable.Variable{
		variable.NewStringVariable(reqHeaderPrefix, nil, requestHeaderMapGetter, nil, 0),
		variable.NewStringVariable(respHeaderPrefix, nil, responseHeaderMapGetter, nil, 0),
	}
)

func registerTestVarDefs() {
	// register built-in variables
	for idx := range builtinVariables {
		variable.Register(builtinVariables[idx])
	}

	// register prefix variables, like header_xxx/arg_xxx/cookie_xxx
	for idx := range prefixVariables {
		variable.RegisterPrefix(prefixVariables[idx].Name(), prefixVariables[idx])
	}
}

// StartTimeGetter
// get request's arriving time
func startTimeGetter(ctx context.Context, value *variable.IndexedValue, data interface{}) (string, error) {
	info := ctx.Value(requestInfoKey).(api.RequestInfo)

	return info.StartTime().Format("2006/01/02 15:04:05.000"), nil
}

// ReceivedDurationGetter
// get duration between request arriving and request resend to upstream
func receivedDurationGetter(ctx context.Context, value *variable.IndexedValue, data interface{}) (string, error) {
	info := ctx.Value(requestInfoKey).(api.RequestInfo)

	return info.RequestReceivedDuration().String(), nil
}

// ResponseReceivedDurationGetter
// get duration between request arriving and response sending
func responseReceivedDurationGetter(ctx context.Context, value *variable.IndexedValue, data interface{}) (string, error) {
	info := ctx.Value(requestInfoKey).(api.RequestInfo)

	return info.ResponseReceivedDuration().String(), nil
}

// RequestFinishedDurationGetter hets duration between request arriving and request finished
func requestFinishedDurationGetter(ctx context.Context, value *variable.IndexedValue, data interface{}) (string, error) {
	info := ctx.Value(requestInfoKey).(api.RequestInfo)

	return info.RequestFinishedDuration().String(), nil
}

// BytesSentGetter
// get bytes sent
func bytesSentGetter(ctx context.Context, value *variable.IndexedValue, data interface{}) (string, error) {
	info := ctx.Value(requestInfoKey).(api.RequestInfo)

	return strconv.FormatUint(info.BytesSent(), 10), nil
}

// BytesReceivedGetter
// get bytes received
func bytesReceivedGetter(ctx context.Context, value *variable.IndexedValue, data interface{}) (string, error) {
	info := ctx.Value(requestInfoKey).(api.RequestInfo)

	return strconv.FormatUint(info.BytesReceived(), 10), nil
}

// get request's protocol type
func protocolGetter(ctx context.Context, value *variable.IndexedValue, data interface{}) (string, error) {
	info := ctx.Value(requestInfoKey).(api.RequestInfo)
	if info.Protocol() == "" {
		return variable.ValueNotFound, nil
	}
	return string(info.Protocol()), nil
}

// ResponseCodeGetter
// get request's response code
func responseCodeGetter(ctx context.Context, value *variable.IndexedValue, data interface{}) (string, error) {
	info := ctx.Value(requestInfoKey).(api.RequestInfo)

	return strconv.FormatUint(uint64(info.ResponseCode()), 10), nil
}

// DurationGetter
// get duration since request's starting time
func durationGetter(ctx context.Context, value *variable.IndexedValue, data interface{}) (string, error) {
	info := ctx.Value(requestInfoKey).(api.RequestInfo)

	return info.Duration().String(), nil
}

// GetResponseFlagGetter
// get request's response flag
func responseFlagGetter(ctx context.Context, value *variable.IndexedValue, data interface{}) (string, error) {
	info := ctx.Value(requestInfoKey).(api.RequestInfo)

	return strconv.FormatBool(info.GetResponseFlag(0)), nil
}

// UpstreamLocalAddressGetter
// get upstream's local address
func upstreamLocalAddressGetter(ctx context.Context, value *variable.IndexedValue, data interface{}) (string, error) {
	info := ctx.Value(requestInfoKey).(api.RequestInfo)

	return info.UpstreamLocalAddress(), nil
}

// DownstreamLocalAddressGetter
// get downstream's local address
func downstreamLocalAddressGetter(ctx context.Context, value *variable.IndexedValue, data interface{}) (string, error) {
	info := ctx.Value(requestInfoKey).(api.RequestInfo)

	if info.DownstreamLocalAddress() != nil {
		return info.DownstreamLocalAddress().String(), nil
	}

	return variable.ValueNotFound, nil
}

// DownstreamRemoteAddressGetter
// get upstream's remote address
func downstreamRemoteAddressGetter(ctx context.Context, value *variable.IndexedValue, data interface{}) (string, error) {
	info := ctx.Value(requestInfoKey).(api.RequestInfo)

	if info.DownstreamRemoteAddress() != nil {
		return info.DownstreamRemoteAddress().String(), nil
	}

	return variable.ValueNotFound, nil
}

// upstreamHostGetter
// get upstream's selected host address
func upstreamHostGetter(ctx context.Context, value *variable.IndexedValue, data interface{}) (string, error) {
	info := ctx.Value(requestInfoKey).(api.RequestInfo)

	if info.UpstreamHost() != nil {
		return info.UpstreamHost().Hostname(), nil
	}

	return variable.ValueNotFound, nil
}

func requestHeaderMapGetter(ctx context.Context, value *variable.IndexedValue, data interface{}) (string, error) {
	headers := ctx.Value(requestHeaderMapKey).(api.HeaderMap)

	headerName := data.(string)
	headerValue, ok := headers.Get(headerName[reqHeaderIndex:])
	if !ok {
		return variable.ValueNotFound, nil
	}

	return headerValue, nil
}

func responseHeaderMapGetter(ctx context.Context, value *variable.IndexedValue, data interface{}) (string, error) {
	headers := ctx.Value(responseHeaderMapKey).(api.HeaderMap)

	headerName := data.(string)
	headerValue, ok := headers.Get(headerName[respHeaderIndex:])
	if !ok {
		return variable.ValueNotFound, nil
	}

	return headerValue, nil
}
