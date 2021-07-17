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

package http2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"mosn.io/api"
	mbuffer "mosn.io/mosn/pkg/buffer"
	mosnctx "mosn.io/mosn/pkg/context"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/module/http2"
	"mosn.io/mosn/pkg/mtls"
	"mosn.io/mosn/pkg/protocol"
	mhttp2 "mosn.io/mosn/pkg/protocol/http2"
	str "mosn.io/mosn/pkg/stream"
	"mosn.io/mosn/pkg/trace"
	"mosn.io/mosn/pkg/types"
	"mosn.io/mosn/pkg/variable"
	"mosn.io/pkg/buffer"
)

func init() {
	str.Register(protocol.HTTP2, &streamConnFactory{})
	protocol.RegisterProtocolConfigHandler(protocol.HTTP2, streamConfigHandler)
}

type streamConnFactory struct{}

func (f *streamConnFactory) CreateClientStream(context context.Context, connection types.ClientConnection,
	clientCallbacks types.StreamConnectionEventListener, connCallbacks api.ConnectionEventListener) types.ClientStreamConnection {
	return newClientStreamConnection(context, connection, clientCallbacks)
}

func (f *streamConnFactory) CreateServerStream(context context.Context, connection api.Connection,
	serverCallbacks types.ServerStreamConnectionEventListener) types.ServerStreamConnection {
	return newServerStreamConnection(context, connection, serverCallbacks)
}

func (f *streamConnFactory) CreateBiDirectStream(context context.Context, connection types.ClientConnection,
	clientCallbacks types.StreamConnectionEventListener,
	serverCallbacks types.ServerStreamConnectionEventListener) types.ClientStreamConnection {
	return nil
}

func (f *streamConnFactory) ProtocolMatch(context context.Context, prot string, magic []byte) error {
	var size int
	var again bool
	if len(magic) >= len(http2.ClientPreface) {
		size = len(http2.ClientPreface)
	} else {
		size = len(magic)
		again = true
	}

	if bytes.Equal(magic[:size], []byte(http2.ClientPreface[:size])) {
		if again {
			return str.EAGAIN
		} else {
			return nil
		}
	} else {
		return str.FAILED
	}
}

// types.DecodeFilter
// types.StreamConnection
// types.ClientStreamConnection
// types.ServerStreamConnection
type streamConnection struct {
	ctx  context.Context
	conn api.Connection
	cm   *str.ContextManager

	useStream bool

	protocol api.Protocol
}

func (conn *streamConnection) Protocol() types.ProtocolName {
	return protocol.HTTP2
}

func (conn *streamConnection) EnableWorkerPool() bool {
	return true
}

func (conn *streamConnection) GoAway() {
	// todo
}

// types.Stream
// types.StreamSender
type stream struct {
	str.BaseStream

	ctx      context.Context
	receiver types.StreamReceiveListener

	id      uint32
	header  types.HeaderMap
	recData types.IoBuffer
	trailer *mhttp2.HeaderMap
	conn    api.Connection
}

// ~~ types.Stream
func (s *stream) ID() uint64 {
	return uint64(s.id)
}

func (s *stream) ReadDisable(disable bool) {
	s.conn.SetReadDisable(disable)
}

func (s *stream) BufferLimit() uint32 {
	return s.conn.BufferLimit()
}

func (s *stream) GetStream() types.Stream {
	return s
}

type StreamConfig struct {
	Http2UseStream bool `json:"http2_use_stream,omitempty"`
}

var defaultStreamConfig = StreamConfig{
	Http2UseStream: false,
}

func streamConfigHandler(v interface{}) interface{} {
	extendConfig, ok := v.(map[string]interface{})
	if !ok {
		return defaultStreamConfig
	}

	config, ok := extendConfig[string(protocol.HTTP2)]
	if !ok {
		config = extendConfig
	}
	configBytes, err := json.Marshal(config)
	if err != nil {
		return defaultStreamConfig
	}
	streamConfig := defaultStreamConfig
	if err := json.Unmarshal(configBytes, &streamConfig); err != nil {
		return defaultStreamConfig
	}

	return streamConfig

}

func parseStreamConfig(ctx context.Context) StreamConfig {
	streamConfig := defaultStreamConfig
	// get extend config from ctx
	pgc := mosnctx.Get(ctx, types.ContextKeyProxyGeneralConfig)
	if cfg, ok := pgc.(StreamConfig); ok {
		streamConfig = cfg
	}
	return streamConfig
}

type serverStreamConnection struct {
	streamConnection
	mutex   sync.RWMutex
	streams map[uint32]*serverStream
	sc      *http2.MServerConn
	config  StreamConfig

	serverCallbacks types.ServerStreamConnectionEventListener
}

func newServerStreamConnection(ctx context.Context, connection api.Connection, serverCallbacks types.ServerStreamConnectionEventListener) types.ServerStreamConnection {

	h2sc := http2.NewServerConn(connection)

	sc := &serverStreamConnection{
		streamConnection: streamConnection{
			ctx:      ctx,
			conn:     connection,
			protocol: mhttp2.ServerProto(h2sc),

			cm: str.NewContextManager(ctx),
		},
		sc:     h2sc,
		config: parseStreamConfig(ctx),

		serverCallbacks: serverCallbacks,
	}

	// init first context
	sc.cm.Next()

	// set not support transfer connection
	sc.conn.SetTransferEventListener(func() bool {
		return false
	})

	sc.streams = make(map[uint32]*serverStream, 32)

	connection.AddConnectionEventListener(sc)
	if log.Proxy.GetLogLevel() >= log.DEBUG {
		log.Proxy.Debugf(ctx, "new http2 server stream connection, stream config: %v", sc.config)
	}

	return sc
}

var errClosedServerConn = errors.New("server conn is closed")

func (conn *serverStreamConnection) OnEvent(event api.ConnectionEvent) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	if event.IsClose() || event.ConnectFailure() {
		for _, stream := range conn.streams {
			stream.ResetStream(types.StreamRemoteReset)
		}
	}
}

// types.StreamConnectionM
func (conn *serverStreamConnection) Dispatch(buf types.IoBuffer) {
	for {
		// 1. pre alloc stream-level ctx with bufferCtx
		ctx := conn.cm.Get()

		// 2. decode process
		frame, err := conn.protocol.Decode(ctx, buf)
		// No enough data
		if err == http2.ErrAGAIN {
			break
		}

		// Do handle staff. Error would also be passed to this function.
		conn.handleFrame(ctx, frame, err)
		if err != nil {
			break
		}

		conn.cm.Next()
	}
}

func (conn *serverStreamConnection) ActiveStreamsNum() int {
	conn.mutex.RLock()
	defer conn.mutex.Unlock()

	return len(conn.streams)
}

func (conn *serverStreamConnection) CheckReasonError(connected bool, event api.ConnectionEvent) (types.StreamResetReason, bool) {
	reason := types.StreamConnectionSuccessed
	if event.IsClose() || event.ConnectFailure() {
		reason = types.StreamConnectionFailed
		if connected {
			reason = types.StreamConnectionTermination
		}
		return reason, false

	}

	return reason, true
}

func (conn *serverStreamConnection) Reset(reason types.StreamResetReason) {
	conn.mutex.RLock()
	defer conn.mutex.Unlock()

	for _, stream := range conn.streams {
		stream.ResetStream(reason)
	}
}

func (conn *serverStreamConnection) handleFrame(ctx context.Context, i interface{}, err error) {
	f, _ := i.(http2.Frame)
	if err != nil {
		conn.handleError(ctx, f, err)
		return
	}
	var h2s *http2.MStream
	var endStream, hasTrailer bool
	var data []byte

	h2s, data, hasTrailer, endStream, err = conn.sc.HandleFrame(ctx, f)

	if err != nil {
		conn.handleError(ctx, f, err)
		return
	}

	if h2s == nil && data == nil && !hasTrailer && !endStream {
		return
	}

	id := f.Header().StreamID

	var stream *serverStream
	// header
	if h2s != nil {
		stream, err = conn.onNewStreamDetect(mosnctx.Clone(ctx), h2s, endStream)
		if err != nil {
			conn.handleError(ctx, f, err)
			return
		}
		header := mhttp2.NewReqHeader(h2s.Request)

		scheme := "http"
		if _, ok := conn.conn.RawConn().(*mtls.TLSConn); ok {
			scheme = "https"
		}

		h2s.Request.URL.Scheme = strings.ToLower(scheme)

		variable.SetString(ctx, types.VarScheme, scheme)
		variable.SetString(ctx, types.VarMethod, h2s.Request.Method)
		variable.SetString(ctx, types.VarHost, h2s.Request.Host)
		variable.SetString(ctx, types.VarIstioHeaderHost, h2s.Request.Host) // be consistent with http1
		variable.SetString(ctx, types.VarPath, h2s.Request.URL.Path)
		variable.SetString(ctx, types.VarPathOriginal, h2s.Request.URL.EscapedPath())

		if h2s.Request.URL.RawQuery != "" {
			variable.SetString(ctx, types.VarQueryString, h2s.Request.URL.RawQuery)
		}

		if log.Proxy.GetLogLevel() >= log.DEBUG {
			log.Proxy.Debugf(stream.ctx, "http2 server header: %d, %+v", id, h2s.Request.Header)
		}

		if endStream {
			stream.receiver.OnReceive(stream.ctx, header, nil, nil)
			return
		}
		stream.header = header
		stream.trailer = &mhttp2.HeaderMap{}
		if conn.useStream {
			stream.recData = buffer.NewIoBufferChain(0)
			stream.receiver.OnReceive(stream.ctx, stream.header, stream.recData, stream.trailer)
		} else {
			stream.recData = buffer.GetIoBuffer(0)
		}
	}

	// data
	if data != nil {
		stream = conn.onStreamRecv(ctx, id, endStream)
		if stream == nil {
			log.Proxy.Errorf(ctx, "http2 server OnStreamRecv error, invaild id = %d", id)
			return
		}

		if log.Proxy.GetLogLevel() >= log.DEBUG {
			log.Proxy.Debugf(ctx, "http2 server receive data: %d", id)
		}

		if _, err = stream.recData.Write(data); err != nil {
			conn.handleError(ctx, f, http2.StreamError{
				StreamID: id,
				Code:     http2.ErrCodeCancel,
				Cause:    err,
			})
			return
		}
	}

	if hasTrailer {
		stream.trailer.H = stream.h2s.Request.Trailer
		if log.Proxy.GetLogLevel() >= log.DEBUG {
			log.Proxy.Debugf(stream.ctx, "http2 server trailer: %d, %v", id, stream.h2s.Request.Trailer)
		}
	}

	if endStream {
		if conn.useStream {
			stream.recData.CloseWithError(io.EOF)
		} else {
			stream.receiver.OnReceive(stream.ctx, stream.header, stream.recData, stream.trailer)
		}
		if log.Proxy.GetLogLevel() >= log.DEBUG {
			log.Proxy.Debugf(stream.ctx, "http2 server stream end %d", id)
		}
	}

}

func (conn *serverStreamConnection) handleError(ctx context.Context, f http2.Frame, err error) {
	conn.sc.HandleError(ctx, f, err)
	if err != nil {
		switch err := err.(type) {
		// todo: other error scenes
		case http2.StreamError:
			if err.Code == http2.ErrCodeNo {
				return
			}
			log.Proxy.Errorf(ctx, "Http2 server handleError stream error: %v", err)
			conn.mutex.Lock()
			s := conn.streams[err.StreamID]
			if s != nil {
				delete(conn.streams, err.StreamID)
			}
			conn.mutex.Unlock()
			if s != nil {
				s.ResetStream(types.StreamLocalReset)
			}
		case http2.ConnectionError:
			log.Proxy.Errorf(ctx, "Http2 server handleError conn err: %v", err)
			conn.conn.Close(api.NoFlush, api.OnReadErrClose)
		default:
			log.Proxy.Errorf(ctx, "Http2 server handleError err: %v", err)
			conn.conn.Close(api.NoFlush, api.RemoteClose)
		}
	}
}

func (conn *serverStreamConnection) onNewStreamDetect(ctx context.Context, h2s *http2.MStream, endStream bool) (*serverStream, error) {
	stream := &serverStream{}
	stream.id = h2s.ID()
	stream.ctx = mosnctx.WithValue(ctx, types.ContextKeyStreamID, stream.id)
	stream.sc = conn
	stream.h2s = h2s
	stream.h2s.UseStream = conn.useStream
	stream.conn = conn.conn

	conn.mutex.Lock()
	conn.streams[stream.id] = stream
	conn.mutex.Unlock()

	var span api.Span
	if trace.IsEnabled() {
		// try build trace span
		tracer := trace.Tracer(protocol.HTTP2)

		if tracer != nil {
			span = tracer.Start(ctx, h2s.Request, time.Now())
		}
	}

	stream.receiver = conn.serverCallbacks.NewStreamDetect(stream.ctx, stream, span)
	return stream, nil
}

func (conn *serverStreamConnection) onStreamRecv(ctx context.Context, id uint32, endStream bool) *serverStream {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	if stream, ok := conn.streams[id]; ok {
		if log.Proxy.GetLogLevel() >= log.DEBUG {
			log.Proxy.Debugf(stream.ctx, "http2 server OnStreamRecv, id = %d", stream.id)
		}
		return stream
	}
	return nil
}

type serverStream struct {
	stream
	h2s *http2.MStream
	sc  *serverStreamConnection
}

// types.StreamSender
func (s *serverStream) AppendHeaders(ctx context.Context, headers api.HeaderMap, endStream bool) error {
	var rsp *http.Response

	var status int

	value, err := variable.GetString(ctx, types.VarHeaderStatus)
	if err != nil || value == "" {
		status = 200
	} else {
		status, _ = strconv.Atoi(value)
	}

	switch header := headers.(type) {
	case *mhttp2.RspHeader:
		rsp = header.Rsp
	case *mhttp2.ReqHeader:
		// indicates the invocation is under hijack scene
		rsp = new(http.Response)
		rsp.StatusCode = status
		rsp.Header = s.h2s.Request.Header
	default:
		rsp = new(http.Response)
		rsp.StatusCode = status
		rsp.Header = mhttp2.EncodeHeader(headers)
	}

	s.h2s.Response = rsp

	if log.Proxy.GetLogLevel() >= log.DEBUG {
		log.Proxy.Debugf(s.ctx, "http2 server ApppendHeaders id = %d, headers = %+v", s.id, rsp.Header)
	}

	if endStream {
		s.endStream()
	}

	return nil
}

func (s *serverStream) AppendData(context context.Context, data buffer.IoBuffer, endStream bool) error {
	s.h2s.SendData = data
	if log.Proxy.GetLogLevel() >= log.DEBUG {
		log.Proxy.Debugf(s.ctx, "http2 server ApppendData id = %d", s.id)
	}

	if endStream {
		s.endStream()
	}

	return nil
}

func (s *serverStream) AppendTrailers(context context.Context, trailers api.HeaderMap) error {
	if trailers != nil {
		switch trailer := trailers.(type) {
		case *mhttp2.HeaderMap:
			s.h2s.Trailer = &trailer.H
		default:
			header := mhttp2.EncodeHeader(trailer)
			s.h2s.Trailer = &header
		}
		if log.Proxy.GetLogLevel() >= log.DEBUG {
			log.Proxy.Debugf(s.ctx, "http2 server ApppendTrailers id = %d, trailer = %+v", s.id, s.h2s.Trailer)
		}
	}
	s.endStream()

	return nil
}

func (s *serverStream) ResetStream(reason types.StreamResetReason) {
	// on stream reset
	if log.Proxy.GetLogLevel() >= log.WARN {
		log.Proxy.Warnf(s.ctx, "http2 server reset stream id = %d, error = %v", s.id, reason)
	}
	if s.sc.useStream && s.recData != nil {
		s.recData.CloseWithError(io.EOF)
	}

	s.h2s.Reset()
	s.stream.ResetStream(reason)
}

func (s *serverStream) GetStream() types.Stream {
	return s
}

func (s *serverStream) endStream() {
	if s.h2s.SendData != nil {
		// Need to reset the 'Content-Length' response header when it's a direct response.
		isDirectResponse, _ := variable.GetString(s.ctx, types.VarProxyIsDirectResponse)
		if isDirectResponse == types.IsDirectResponse {
			s.h2s.Response.Header.Set("Content-Length", strconv.Itoa(s.h2s.SendData.Len()))
		}
	}

	_, err := s.sc.protocol.Encode(s.ctx, s.h2s)

	s.sc.mutex.Lock()
	delete(s.sc.streams, s.id)
	s.sc.mutex.Unlock()

	if err != nil {
		log.Proxy.Errorf(s.ctx, "http2 server SendResponse error :%v", err)
		s.ResetStream(types.StreamLocalReset)
		return
	}
	if s.sc.useStream && s.recData != nil {
		s.recData.CloseWithError(io.EOF)
	}

	if log.Proxy.GetLogLevel() >= log.DEBUG {
		log.Proxy.Debugf(s.ctx, "http2 server SendResponse id = %d", s.id)
	}
}

type clientStreamConnection struct {
	streamConnection
	lastStream                    uint32
	mutex                         sync.RWMutex
	streams                       map[uint32]*clientStream
	mClientConn                   *http2.MClientConn
	streamConnectionEventListener types.StreamConnectionEventListener
}

func newClientStreamConnection(ctx context.Context, connection api.Connection,
	clientCallbacks types.StreamConnectionEventListener) types.ClientStreamConnection {

	h2cc := http2.NewClientConn(connection)

	sc := &clientStreamConnection{
		streamConnection: streamConnection{
			ctx:      ctx,
			conn:     connection,
			protocol: mhttp2.ClientProto(h2cc),

			cm: str.NewContextManager(ctx),
		},
		mClientConn:                   h2cc,
		streamConnectionEventListener: clientCallbacks,
	}

	if pgc := mosnctx.Get(ctx, types.ContextKeyProxyGeneralConfig); pgc != nil {
		if extendConfig, ok := pgc.(map[string]interface{}); ok {
			if v, ok := extendConfig["http2_use_stream"]; ok {
				if bv, ok := v.(bool); ok {
					sc.useStream = bv
				}
			}
		}
	}

	// init first context
	sc.cm.Next()

	sc.streams = make(map[uint32]*clientStream, 32)
	connection.AddConnectionEventListener(sc)
	return sc
}

var errClosedClientConn = errors.New("http2: client conn is closed")

func (conn *clientStreamConnection) OnEvent(event api.ConnectionEvent) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	if event.IsClose() || event.ConnectFailure() {
		for _, stream := range conn.streams {
			if buf := stream.recData; buf != nil {
				buf.CloseWithError(errClosedClientConn)
			}
		}

	}
}

// types.StreamConnection
func (conn *clientStreamConnection) Dispatch(buf types.IoBuffer) {
	for {
		// 1. pre alloc stream-level ctx with bufferCtx
		ctx := conn.cm.Get()

		// 2. decode process
		frame, err := conn.protocol.Decode(ctx, buf)
		// No enough data
		if err == http2.ErrAGAIN {
			break
		}

		// Do handle staff. Error would also be passed to this function.
		conn.handleFrame(ctx, frame, err)
		if err != nil {
			break
		}

		conn.cm.Next()
	}
}

func (conn *clientStreamConnection) ActiveStreamsNum() int {
	conn.mutex.RLock()
	defer conn.mutex.RUnlock()

	return len(conn.streams)
}

func (conn *clientStreamConnection) CheckReasonError(connected bool, event api.ConnectionEvent) (types.StreamResetReason, bool) {
	reason := types.StreamConnectionSuccessed
	if event.IsClose() || event.ConnectFailure() {
		reason = types.StreamConnectionFailed
		if connected {
			reason = types.StreamConnectionTermination
		}
		return reason, false

	}

	return reason, true
}

func (conn *clientStreamConnection) Reset(reason types.StreamResetReason) {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()

	for _, stream := range conn.streams {
		stream.connReset = true
		stream.ResetStream(reason)
	}
}

func (conn *clientStreamConnection) NewStream(ctx context.Context, receiver types.StreamReceiveListener) types.StreamSender {
	stream := &clientStream{}

	stream.ctx = ctx
	stream.sc = conn
	stream.receiver = receiver
	stream.conn = conn.conn

	return stream
}

func (conn *clientStreamConnection) handleFrame(ctx context.Context, i interface{}, err error) {
	f, _ := i.(http2.Frame)
	if err != nil {
		conn.handleError(ctx, f, err)
		return
	}
	var endStream bool
	var data []byte
	var trailer http.Header
	var rsp *http.Response
	var lastStream uint32

	rsp, data, trailer, endStream, lastStream, err = conn.mClientConn.HandleFrame(ctx, f)

	if err != nil {
		conn.handleError(ctx, f, err)
		return
	}

	if rsp == nil && trailer == nil && data == nil && !endStream && lastStream == 0 {
		return
	}

	if lastStream != 0 {
		conn.lastStream = lastStream
		conn.streamConnectionEventListener.OnGoAway()
		if log.DefaultLogger.GetLogLevel() >= log.DEBUG {
			log.DefaultLogger.Debugf("http2 client recevice goaway lastStremID = %d", conn.lastStream)
		}
		return
	}

	id := f.Header().StreamID

	conn.mutex.Lock()
	stream := conn.streams[id]
	conn.mutex.Unlock()

	if stream == nil {
		log.Proxy.Errorf(ctx, "http2 client invaild steamID :%v", f)
		return
	}

	if rsp != nil {
		header := mhttp2.NewRspHeader(rsp)

		// set header-status into stream ctx
		variable.SetString(stream.ctx, types.VarHeaderStatus, strconv.Itoa(rsp.StatusCode))

		mbuffer.TransmitBufferPoolContext(stream.ctx, ctx)

		if log.Proxy.GetLogLevel() >= log.DEBUG {
			log.Proxy.Debugf(stream.ctx, "http2 client header: id = %d, headers = %+v", id, rsp.Header)
		}

		if endStream {
			if stream.receiver == nil {
				return
			}

			stream.receiver.OnReceive(stream.ctx, header, nil, nil)
			conn.mutex.Lock()
			delete(conn.streams, id)
			conn.mutex.Unlock()

			return
		}
		stream.header = header
		stream.trailer = &mhttp2.HeaderMap{}
		if conn.useStream {
			stream.recData = buffer.NewIoBufferChain(0)
			stream.receiver.OnReceive(stream.ctx, stream.header, stream.recData, stream.trailer)
		} else {
			stream.recData = buffer.GetIoBuffer(0)
		}
	}

	// data
	if data != nil {
		if log.Proxy.GetLogLevel() >= log.DEBUG {
			log.DefaultLogger.Debugf("http2 client receive data: %d", id)
		}
		if _, err = stream.recData.Write(data); err != nil {
			conn.handleError(ctx, f, &http2.StreamError{
				StreamID: id,
				Code:     http2.ErrCodeCancel,
				Cause:    err,
			})
		}

	}
	if trailer != nil {
		stream.trailer.H = trailer
		if log.Proxy.GetLogLevel() >= log.DEBUG {
			log.Proxy.Debugf(stream.ctx, "http2 client trailer: id = %d, trailer = %+v", id, trailer)
		}
	}

	if endStream {
		if conn.useStream {
			stream.recData.CloseWithError(io.EOF)
		} else if stream.receiver != nil {
			stream.receiver.OnReceive(stream.ctx, stream.header, stream.recData, stream.trailer)
		}
		if log.Proxy.GetLogLevel() >= log.DEBUG {
			log.DefaultLogger.Debugf("http2 client stream receive end %d", id)
		}
		conn.mutex.Lock()
		delete(conn.streams, id)
		conn.mutex.Unlock()
	}
}

func (conn *clientStreamConnection) handleError(ctx context.Context, f http2.Frame, err error) {
	//conn.mClientConn.HandleError(ctx, f, err)
	if err != nil {
		switch err := err.(type) {
		// todo: other error scenes
		case http2.StreamError:
			if err.Code == http2.ErrCodeNo {
				return
			}
			log.Proxy.Errorf(ctx, "Http2 client handleError stream err: %v", err)
			conn.mutex.Lock()
			s := conn.streams[err.StreamID]
			conn.mutex.Unlock()
			if s != nil {
				s.ResetStream(types.StreamRemoteReset)
			}
		case http2.ConnectionError:
			log.Proxy.Errorf(ctx, "Http2 client handleError conn err: %v", err)
			conn.conn.Close(api.FlushWrite, api.OnReadErrClose)
		default:
			log.Proxy.Errorf(ctx, "Http2 client handleError err: %v", err)
			conn.conn.Close(api.NoFlush, api.RemoteClose)
		}
	}
}

type clientStream struct {
	stream
	useStream bool
	connReset bool

	h2s *http2.MClientStream
	sc  *clientStreamConnection
}

func (s *clientStream) AppendHeaders(ctx context.Context, headersIn api.HeaderMap, endStream bool) error {
	var req *http.Request
	var isReqHeader bool

	// clone for retry
	headersIn = headersIn.Clone()
	switch header := headersIn.(type) {
	case *mhttp2.ReqHeader:
		req = header.Req
		isReqHeader = true
	default:
		req = new(http.Request)
	}

	scheme := "http"
	if _, ok := s.conn.RawConn().(*mtls.TLSConn); ok {
		scheme = "https"
	}

	var method string
	method, err := variable.GetString(ctx, types.VarMethod)
	if err != nil || method == "" {
		if endStream {
			method = http.MethodGet
		} else {
			method = http.MethodPost
		}
	}

	var host string
	h, err := variable.GetString(ctx, types.VarHost)
	if err == nil && h != "" {
		host = h
	} else if h, ok := headersIn.Get("Host"); ok {
		host = h
	} else {
		host = s.conn.RemoteAddr().String()
	}

	if h, err := variable.GetString(ctx, types.VarIstioHeaderHost); err != nil && h != "" { // be consistent with http1
		host = h
	}

	var path string
	path, _ = variable.GetString(ctx, types.VarPath)

	var pathOriginal string
	pathOriginal, _ = variable.GetString(ctx, types.VarPathOriginal)

	var query string
	query, _ = variable.GetString(ctx, types.VarQueryString)

	URL := &url.URL{
		Scheme:   scheme,
		Host:     req.Host,
		Path:     path,
		RawPath:  pathOriginal,
		RawQuery: query,
	}

	if !isReqHeader {
		req.Method = method
		req.Host = host
		req.URL = URL
		req.Header = mhttp2.EncodeHeader(headersIn)
	}

	if log.Proxy.GetLogLevel() >= log.DEBUG {
		log.Proxy.Debugf(s.ctx, "http2 client AppendHeaders: id = %d, headers = %+v", s.id, req.Header)
	}

	s.h2s = http2.NewMClientStream(s.sc.mClientConn, req)
	s.h2s.UseStream = s.sc.useStream

	if endStream {
		s.endStream()
	}
	return nil
}

func (s *clientStream) AppendData(context context.Context, data buffer.IoBuffer, endStream bool) error {
	s.h2s.SendData = data
	if log.Proxy.GetLogLevel() >= log.DEBUG {
		log.Proxy.Debugf(s.ctx, "http2 client AppendData: id = %d", s.id)
	}
	if endStream {
		s.endStream()
	}

	return nil
}

func (s *clientStream) AppendTrailers(context context.Context, trailers api.HeaderMap) error {
	if trailers != nil {
		switch trailer := trailers.(type) {
		case *mhttp2.HeaderMap:
			s.h2s.Trailer = &trailer.H
		default:
			header := mhttp2.EncodeHeader(trailer)
			s.h2s.Trailer = &header
		}
		if log.Proxy.GetLogLevel() >= log.DEBUG {
			log.Proxy.Debugf(s.ctx, "http2 client AppendTrailers: id = %d, trailer = %+v", s.id, s.h2s.Trailer)
		}
	}
	s.endStream()

	return nil
}

func (s *clientStream) AppendPing(context context.Context) {
	err := s.sc.mClientConn.Ping(context)
	if err != nil {
		log.Proxy.Errorf(s.ctx, "http2 client ping error = %v", err)
		if err == types.ErrConnectionHasClosed {
			s.ResetStream(types.StreamConnectionFailed)
		} else {
			s.ResetStream(types.StreamLocalReset)
		}
	}
}

func (s *clientStream) endStream() {
	// send header
	s.sc.mutex.Lock()
	_, err := s.sc.protocol.Encode(s.ctx, s.h2s)
	if err == nil {
		s.id = s.h2s.GetID()
		s.sc.streams[s.id] = s
		s.sc.mutex.Unlock()
	} else {
		s.sc.mutex.Unlock()
		goto reset
	}

	if log.Proxy.GetLogLevel() >= log.DEBUG {
		log.Proxy.Debugf(s.ctx, "http2 client SendRequest id = %d", s.id)
	}
	// send body and trailer
	_, err = s.sc.protocol.Encode(s.ctx, s.h2s)
	if err == nil {
		return
	}

reset:
	log.Proxy.Errorf(s.ctx, "http2 client endStream error = %v", err)
	if err == types.ErrConnectionHasClosed || err == errClosedClientConn {
		s.ResetStream(types.StreamConnectionFailed)
		return
	}
	// fix: https://github.com/mosn/mosn/issues/1672
	if strings.Contains(err.Error(), "broken pipe") || strings.Contains(err.Error(), "connection reset by peer") {
		s.ResetStream(types.StreamConnectionFailed)
		return
	}
	s.ResetStream(types.StreamLocalReset)
}

func (s *clientStream) GetStream() types.Stream {
	return s
}

func (s *clientStream) ResetStream(reason types.StreamResetReason) {
	// reset by goaway, support retry.
	if s.sc.lastStream > 0 && s.id > s.sc.lastStream {
		if log.DefaultLogger.GetLogLevel() >= log.WARN {
			log.DefaultLogger.Warnf("http2 client reset by goaway, retry it, lastStream = %d, streamId = %d", s.sc.lastStream, s.id)
		}
		reason = types.StreamConnectionFailed
	}
	switch reason {
	case types.StreamConnectionTermination:
		reason = types.StreamConnectionFailed
	}

	if s.h2s != nil {
		s.h2s.Reset()
	}

	if !s.connReset {
		s.sc.mutex.Lock()
		delete(s.sc.streams, s.id)
		s.sc.mutex.Unlock()
	}

	s.stream.ResetStream(reason)
}
