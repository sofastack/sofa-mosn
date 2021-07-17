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

package stream

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"

	"mosn.io/api"
	"mosn.io/mosn/pkg/protocol"
	"mosn.io/mosn/pkg/streamfilter"
	"mosn.io/mosn/pkg/types"
	"mosn.io/pkg/buffer"
)

const (
	streamStateConnected uint32 = iota
	streamStateReset
	streamStateDestroyed
)

// StreamResetReason defines the reason why stream reset
type StreamResetReason string

// Group of stream reset reasons
const (
	StreamRemoteReset StreamResetReason = "StreamRemoteReset"
	StreamDestroy     StreamResetReason = "StreamDestroy"
)

var streamManagerInstance *StreamManager = &StreamManager{}

// GetStreamFilterManager return a global singleton of StreamFilterManager.
func GetStreamManager() *StreamManager {
	return streamManagerInstance
}

// StreamManager is used to manage stream
type StreamManager struct {
	streamManagerMap sync.Map
}

// GetStreamFilterFactory return StreamFilterFactory indexed by sid.
func (s *StreamManager) GetOrCreateStreamByID(sid uint64) (*ActiveStream, error) {
	stream, err := s.GetStreamByID(sid)
	if err == nil {
		return stream, nil
	} else {
		v, _ := s.streamManagerMap.LoadOrStore(sid, CreateActiveStream(context.TODO()))
		stream, ok := v.(*ActiveStream)
		if !ok {
			return nil, errors.New("[GetOrCreateStreamByID] get stream failed")
		}

		return stream, nil
	}
}

// DestoryStreamByID delete stream.
func (s *StreamManager) DestoryStreamByID(sid uint64) {
	stream, err := s.GetStreamByID(sid)
	if err != nil {
		return
	}

	s.streamManagerMap.Delete(sid)
	stream.ResetStream(StreamDestroy)
	stream.Destory()
}

func (s *StreamManager) GetStreamByID(sid uint64) (*ActiveStream, error) {
	if v, ok := s.streamManagerMap.Load(sid); ok {
		stream, ok := v.(*ActiveStream)
		if !ok {
			return nil, errors.New("[GetStreamByID] unexpected object in map")
		}

		return stream, nil
	}

	return nil, errors.New("[GetStreamByID] get stream failed")
}

// ActiveStream is a fake stream
type ActiveStream struct {
	sync.RWMutex

	state uint32

	// current receive phase
	currentReceivePhase api.ReceiverFilterPhase

	code int
	ctx  context.Context

	cid uint64
	sid uint64

	// request
	reqHeaders  *Headers
	reqTrailers *Headers
	reqDataBuf  types.IoBuffer

	// response
	respHeaders  *Headers
	respTrailers *Headers
	respDataBuf  types.IoBuffer

	directResponse bool
	needAsync      bool

	api.StreamReceiverFilterHandler
	api.StreamSenderFilterHandler

	fm *streamfilter.DefaultStreamFilterChainImpl
}

// CreateActiveStream create ActiveStream.
func CreateActiveStream(ctx context.Context) *ActiveStream {
	sm := &ActiveStream{
		ctx:            ctx,
		state:          streamStateConnected,
		directResponse: false,
		reqHeaders:     &Headers{},
		reqTrailers:    &Headers{},
		respHeaders:    &Headers{},
		respTrailers:   &Headers{},
		// TODO dynamic set filter chain name
		fm: CreateStreamFilter(ctx, DefaultFilterChainName),
	}

	sm.fm.SetReceiveFilterHandler(sm)
	sm.fm.SetSenderFilterHandler(sm)

	return sm
}

// InitResuestStream init request stream.
func (s *ActiveStream) InitResuestStream(headers *Headers, data types.IoBuffer, trailers *Headers) {
	s.reqHeaders = headers
	s.reqDataBuf = data
	s.reqTrailers = trailers
}

// InitResponseStream init response stream.
func (s *ActiveStream) InitResponseStream(headers *Headers, data types.IoBuffer, trailers *Headers) {
	s.respHeaders = headers
	s.respDataBuf = data
	s.respTrailers = trailers
	// reset directResponse flag for response
	s.directResponse = false
	// reset needAsync flag for response
	s.needAsync = false
}

// DirectDeepCopyRequestStream used to deep copy some request member for async.
func (s *ActiveStream) DirectDeepCopyRequestStream() {
	s.respHeaders.DirectDeepCopyKeyValue()
	s.reqTrailers.DirectDeepCopyKeyValue()
	s.reqDataBuf = s.reqDataBuf.Clone()
}

// DirectDeepCopyResponseStream used to deep copy some response member for async.
func (s *ActiveStream) DirectDeepCopyResponseStream() {
	s.respHeaders.DirectDeepCopyKeyValue()
	s.respTrailers.DirectDeepCopyKeyValue()
	s.respDataBuf = s.respDataBuf.Clone()
}

// FilterSyncCheck is used to check whether the filter has sync operation.
type FilterSyncCheck interface {
	CheckHasSync(context.Context, types.HeaderMap, types.IoBuffer, types.HeaderMap) bool
}

// check receive filters need sync operation.
func (s *ActiveStream) receiveFilterAsyncCheck(ctx context.Context, headers types.HeaderMap, data types.IoBuffer, trailers types.HeaderMap, filter api.StreamReceiverFilter) {
	if ck, ok := filter.(FilterSyncCheck); ok {
		if ck.CheckHasSync(ctx, headers, data, trailers) {
			s.needAsync = true
		}
	}
}

// check send filters need sync operation.
func (s *ActiveStream) sendFilterAsyncCheck(ctx context.Context, headers types.HeaderMap, data types.IoBuffer, trailers types.HeaderMap, filter api.StreamSenderFilter) {
	if ck, ok := filter.(FilterSyncCheck); ok {
		if ck.CheckHasSync(ctx, headers, data, trailers) {
			s.needAsync = true
		}
	}
}

// CheckReceiveFilterNeedAsync is used to check receive filter need async mode.
func (s *ActiveStream) CheckReceiveFilterNeedAsync() bool {
	s.fm.RangeReceiverFilter(s.GetStreamContext(), s.GetRequestHeaders(), s.GetRequestData(), s.GetRequestTrailers(), s.receiveFilterAsyncCheck)
	return s.NeedAsync()
}

// CheckSendFilterNeedAsync is used to check send filter need async mode.
func (s *ActiveStream) CheckSendFilterNeedAsync() bool {
	s.fm.RangeSenderFilter(s.GetStreamContext(), s.GetResponseHeaders(), s.GetResponseData(), s.GetResponseTrailers(), s.sendFilterAsyncCheck)
	return s.NeedAsync()
}

// OnReceive is used to run receive filters.
func (s *ActiveStream) OnReceive(headers api.HeaderMap, buf buffer.IoBuffer, trailers api.HeaderMap) {
	s.SetCurrentReveivePhase(api.BeforeRoute)
	s.fm.RunReceiverFilter(s.ctx, api.BeforeRoute, headers, buf, trailers, nil)
}

// OnSend is used to run send filters.
func (s *ActiveStream) OnSend(headers api.HeaderMap, buf buffer.IoBuffer, trailers api.HeaderMap) {
	s.fm.RunSenderFilter(s.ctx, api.BeforeSend, headers, buf, trailers, nil)
}

// Destory is used to destory current stream.
func (s *ActiveStream) Destory() {
	// call filter destroy handler
	s.fm.OnDestroy()
	DestoryStreamFilter(s.fm)
	//TODO reuse stream
}

// ResetStream is used to reset stream.
func (s *ActiveStream) ResetStream(reason StreamResetReason) {
	switch reason {
	case StreamRemoteReset:
		atomic.StoreUint32(&s.state, streamStateReset)
	case StreamDestroy:
		atomic.StoreUint32(&s.state, streamStateDestroyed)
	}
}

// CheckStream check whether the stream is valid.
func (s *ActiveStream) CheckStreamValid() bool {
	return atomic.LoadUint32(&s.state) == streamStateConnected
}

// SetCurrentReveivePhase set current reveive phase.
func (s *ActiveStream) SetCurrentReveivePhase(phase api.ReceiverFilterPhase) {
	s.currentReceivePhase = phase
}

// IsDirectResponse check whether it is direct response.
func (s *ActiveStream) IsDirectResponse() bool {
	return s.directResponse
}

// NeedAsync check whether it is need async operation.
func (s *ActiveStream) NeedAsync() bool {
	return s.needAsync
}

// GetResponseCode get response status code.
func (s *ActiveStream) GetResponseCode() int {
	return s.code
}

// GetStreamContext get stream context.
func (s *ActiveStream) GetStreamContext() context.Context {
	return s.ctx
}

// SetConnectionID is set connection id.
func (s *ActiveStream) SetConnectionID(cid uint64) {
	s.cid = cid
}

// SetStreamID is set stream id.
func (s *ActiveStream) SetStreamID(sid uint64) {
	s.sid = sid
}

// TODO: implement api.StreamFilterHandler after add context and stream id for stream
// implement api.StreamFilterHandler

// fakeConnection is used to implement Connection interface.
type fakeConnection struct {
	api.Connection // TODO implement more api
	id             uint64
}

func (c *fakeConnection) ID() uint64 {
	return c.id
}

func (f *ActiveStream) Connection() api.Connection {
	return &fakeConnection{id: f.cid}
}

func (f *ActiveStream) Route() types.Route {
	var route types.Route
	panic("not implemented")
	return route
}

func (f *ActiveStream) RequestInfo() types.RequestInfo {
	var requestInfo types.RequestInfo
	panic("not implemented")
	return requestInfo
}

func (s *ActiveStream) GetRequestUpdatedHeaders() types.HeaderMap {
	return protocol.CommonHeader(s.reqHeaders.Update)
}

func (s *ActiveStream) GetResponseUpdatedHeaders() types.HeaderMap {
	return protocol.CommonHeader(s.respHeaders.Update)
}

// implement api.StreamReceiverFilterHandler
func (s *ActiveStream) GetRequestHeaders() types.HeaderMap {
	return s.reqHeaders
}

func (s *ActiveStream) GetRequestData() types.IoBuffer {
	return s.reqDataBuf
}

func (s *ActiveStream) GetRequestTrailers() types.HeaderMap {
	return s.reqTrailers
}

// receiver filter handler maybe delete AppendXXX API
func (s *ActiveStream) AppendHeaders(headers types.HeaderMap, endStream bool) {
	panic("not implemented")
}

func (s *ActiveStream) AppendData(buf types.IoBuffer, endStream bool) {
	panic("not implemented")
}

func (s *ActiveStream) AppendTrailers(trailers types.HeaderMap) {
	panic("not implemented")
}

func (s *ActiveStream) SendHijackReply(code int, headers types.HeaderMap) {
	s.directResponse = true
	s.code = code
	h := make(map[string]string)
	if headers != nil {
		headers.Range(func(k, v string) bool {
			h[k] = v
			return true
		})
	}
	s.respHeaders = &Headers{
		CommonHeader: protocol.CommonHeader(h),
	}
}

func (s *ActiveStream) SendHijackReplyWithBody(code int, headers types.HeaderMap, body string) {
	s.directResponse = true
	s.code = code
	h := make(map[string]string)
	if headers != nil {
		headers.Range(func(k, v string) bool {
			h[k] = v
			return true
		})
	}
	s.respHeaders = &Headers{
		CommonHeader: protocol.CommonHeader(h),
	}
	s.respDataBuf = buffer.NewIoBufferString(body)
}

func (s *ActiveStream) SendDirectResponse(headers types.HeaderMap, buf buffer.IoBuffer, trailers types.HeaderMap) {
	s.directResponse = true
	// TODO status suppport auto mapping
	s.code = http.StatusOK
	h := make(map[string]string)
	if headers != nil {
		headers.Range(func(k, v string) bool {
			h[k] = v
			return true
		})
	}
	s.respHeaders = &Headers{
		CommonHeader: protocol.CommonHeader(h),
	}

	s.respDataBuf = buf
	t := make(map[string]string)
	if trailers != nil {
		trailers.Range(func(k, v string) bool {
			t[k] = v
			return true
		})
	}

	s.respTrailers = &Headers{
		CommonHeader: protocol.CommonHeader(t),
	}
}

func (s *ActiveStream) TerminateStream(code int) bool {
	// check if it has been responded.
	if s.GetResponseHeaders() != nil {
		return false
	}

	headers := s.GetRequestHeaders()
	if headers == nil {
		return false
	}

	s.SendHijackReply(code, headers)
	return true
}

func (s *ActiveStream) SetConvert(on bool) {
	panic("not implemented")
}

func (s *ActiveStream) GetFilterCurrentPhase() api.ReceiverFilterPhase {
	return s.currentReceivePhase
}

// implement api.StreamSenderFilterHandler
func (s *ActiveStream) GetResponseHeaders() types.HeaderMap {
	return s.respHeaders
}

func (s *ActiveStream) GetResponseData() types.IoBuffer {
	return s.respDataBuf
}

func (s *ActiveStream) GetResponseTrailers() types.HeaderMap {
	return s.respTrailers
}
