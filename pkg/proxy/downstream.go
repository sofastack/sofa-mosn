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

package proxy

import (
	"container/list"
	"context"
	"fmt"
	"net"
	nethttp "net/http"
	"net/url"
	"reflect"
	"runtime/debug"
	"strconv"
	"sync/atomic"
	"time"

	uatomic "go.uber.org/atomic"
	"mosn.io/api"
	mbuffer "mosn.io/mosn/pkg/buffer"
	v2 "mosn.io/mosn/pkg/config/v2"
	mosnctx "mosn.io/mosn/pkg/context"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/protocol"
	"mosn.io/mosn/pkg/protocol/http"
	"mosn.io/mosn/pkg/router"
	"mosn.io/mosn/pkg/trace"
	"mosn.io/mosn/pkg/track"
	"mosn.io/mosn/pkg/types"
	"mosn.io/mosn/pkg/variable"
	"mosn.io/pkg/buffer"
	"mosn.io/pkg/utils"
)

// types.StreamEventListener
// types.StreamReceiveListener
// Downstream stream, as a controller to handle downstream and upstream proxy flow
type downStream struct {
	ID      uint32
	proxy   *proxy
	route   types.Route
	cluster types.ClusterInfo
	element *list.Element

	// flow control
	bufferLimit uint32

	// ~~~ control args
	timeout    Timeout
	retryState *retryState

	requestInfo     types.RequestInfo
	responseSender  types.StreamSender
	upstreamRequest *upstreamRequest
	perRetryTimer   *utils.Timer
	responseTimer   *utils.Timer

	// ~~~ downstream request buf
	downstreamReqHeaders  types.HeaderMap
	downstreamReqDataBuf  types.IoBuffer
	downstreamReqTrailers types.HeaderMap

	// ~~~ downstream response buf
	downstreamRespHeaders  types.HeaderMap
	downstreamRespDataBuf  types.IoBuffer
	downstreamRespTrailers types.HeaderMap

	// ~~~ state
	// if upstreamResponseReceived == 1 means response is received
	// 1. upstream response is received
	// 2. timeout / terminate triggered
	// the flag will be reset when do a retry
	upstreamResponseReceived uint32
	// starts to send back downstream response
	downstreamResponseStarted bool
	// downstream request received done
	downstreamRecvDone bool
	// upstream req sent
	upstreamRequestSent bool
	// 1. at the end of upstream response 2. by a upstream reset due to exceptions, such as no healthy upstream, connection close, etc.
	upstreamProcessDone uatomic.Bool
	// don't convert headers, data and trailers.  e.g. streamReceiverFilterHandler.Appendxx
	noConvert bool
	// direct response.  e.g. sendHijack
	directResponse bool
	// oneway
	oneway bool

	notify chan struct{}

	downstreamReset   uint32
	downstreamCleaned uint32
	upstreamReset     uint32
	reuseBuffer       uint32

	resetReason uatomic.String //types.StreamResetReason

	// stream filter chain
	streamFilterChain         streamFilterChain
	receiverFiltersAgainPhase types.Phase

	context context.Context
	tracks  *track.Tracks

	// stream access logs
	logDone uint32

	snapshot types.ClusterSnapshot

	phase types.Phase
}

func newActiveStream(ctx context.Context, proxy *proxy, responseSender types.StreamSender, span api.Span) *downStream {
	if span != nil && trace.IsEnabled() {
		ctx = mosnctx.WithValue(ctx, types.ContextKeyActiveSpan, span)
		ctx = mosnctx.WithValue(ctx, types.ContextKeyTraceSpanKey, &trace.SpanKey{TraceId: span.TraceId(), SpanId: span.SpanId()})
	}

	proxyBuffers := proxyBuffersByContext(ctx)

	// save downstream protocol
	// it should priority return real protocol name
	proto := proxy.serverStreamConn.Protocol()
	if proto == protocol.Xprotocol {
		proto = types.ProtocolName(mosnctx.Get(ctx, types.ContextSubProtocol).(string))
	}

	ctx = mosnctx.WithValue(ctx, types.ContextKeyDownStreamProtocol, proto)
	ctx = mosnctx.WithValue(ctx, types.ContextKeyConfigDownStreamProtocol, proxy.config.DownstreamProtocol)
	ctx = mosnctx.WithValue(ctx, types.ContextKeyConfigUpStreamProtocol, proxy.config.UpstreamProtocol)

	stream := &proxyBuffers.stream
	atomic.StoreUint32(&stream.ID, atomic.AddUint32(&currProxyID, 1))
	stream.proxy = proxy
	stream.requestInfo = &proxyBuffers.info
	stream.requestInfo.SetStartTime()
	stream.requestInfo.SetProtocol(proto)
	stream.requestInfo.SetDownstreamLocalAddress(proxy.readCallbacks.Connection().LocalAddr())
	// todo: detect remote addr
	stream.requestInfo.SetDownstreamRemoteAddress(proxy.readCallbacks.Connection().RemoteAddr())
	stream.context = ctx
	stream.reuseBuffer = 1
	stream.notify = make(chan struct{}, 1)

	stream.initStreamFilterChain()

	if responseSender == nil || reflect.ValueOf(responseSender).IsNil() {
		stream.oneway = true
	} else {
		stream.responseSender = responseSender
		stream.responseSender.GetStream().AddEventListener(stream)
	}

	proxy.stats.DownstreamRequestTotal.Inc(1)
	proxy.stats.DownstreamRequestActive.Inc(1)
	proxy.listenerStats.DownstreamRequestTotal.Inc(1)
	proxy.listenerStats.DownstreamRequestActive.Inc(1)

	// info message for new downstream
	if log.Proxy.GetLogLevel() >= log.DEBUG {
		requestID := mosnctx.Get(stream.context, types.ContextKeyStreamID)
		log.Proxy.Debugf(stream.context, "[proxy] [downstream] new stream, proxyId = %d , requestId =%v, oneway=%t", stream.ID, requestID, stream.oneway)
	}
	return stream
}

func (s *downStream) initStreamFilterChain() {
	s.streamFilterChain.init(s)
	s.receiverFiltersAgainPhase = types.InitPhase
}

func (s *downStream) receiverFilterStatusHandler(phase api.ReceiverFilterPhase, status api.StreamFilterStatus) {
	switch status {
	case api.StreamFiltertermination:
		// no reuse buffer
		atomic.StoreUint32(&s.reuseBuffer, 0)
		s.cleanStream()
	case api.StreamFilterReMatchRoute:
		// Retry only at the AfterRoute phase
		if phase == api.AfterRoute {
			// FiltersIndex is not increased until no retry is required
			s.receiverFiltersAgainPhase = types.MatchRoute
		}
	case api.StreamFilterReChooseHost:
		// Retry only at the AfterChooseHost phase
		if phase == api.AfterChooseHost {
			// FiltersIndex is not increased until no retry is required
			s.receiverFiltersAgainPhase = types.ChooseHost
		}
	}
}

func (s *downStream) senderFilterStatusHandler(phase api.SenderFilterPhase, status api.StreamFilterStatus) {
	if status == api.StreamFiltertermination {
		// no reuse buffer
		atomic.StoreUint32(&s.reuseBuffer, 0)
		s.cleanStream()
	}
}

// downstream's lifecycle ends normally
func (s *downStream) endStream() {
	if s.responseSender != nil && !s.downstreamRecvDone {
		// not reuse buffer
		atomic.StoreUint32(&s.reuseBuffer, 0)
	}
	s.cleanStream()

	// note: if proxy logic resets the stream, there maybe some underlying data in the conn.
	// we ignore this for now, fix as a todo
}

// Clean up on the very end of the stream: end stream or reset stream
// Resources to clean up / reset:
// 	+ upstream request
// 	+ all timers
// 	+ all filters
//  + remove stream in proxy context
func (s *downStream) cleanStream() {
	if !atomic.CompareAndSwapUint32(&s.downstreamCleaned, 0, 1) {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Proxy.Alertf(s.context, types.ErrorKeyProxyPanic, "[proxy] [downstream] cleanStream panic: %v, downstream: %+v, streamID: %d\n%s",
				r, s, s.ID, string(debug.Stack()))
		}
	}()

	s.requestInfo.SetRequestFinishedDuration(time.Now())

	// reset corresponding upstream stream
	if s.upstreamRequest != nil && !s.upstreamProcessDone.Load() && !s.oneway {
		log.Proxy.Errorf(s.context, "[proxy] [downstream] upstreamRequest.resetStream, proxyId: %d", s.ID)
		s.upstreamProcessDone.Store(true)
		s.upstreamRequest.resetStream()
	}

	// clean up timers
	s.cleanUp()

	// record metrics
	s.requestMetrics()

	// finish tracing
	s.finishTracing()

	// write access log
	s.writeLog()

	// tell filters it's time to destroy
	// after this func call, we should never touch the s.streamFilterChain
	s.streamFilterChain.destroy()

	// delete stream reference
	s.delete()

	// recycle if no reset events
	s.giveStream()
}

// requestMetrics records the request metrics when cleanStream
func (s *downStream) requestMetrics() {
	streamDurationNs := s.requestInfo.RequestFinishedDuration().Nanoseconds()
	responseReceivedNs := s.requestInfo.ResponseReceivedDuration().Nanoseconds()
	requestReceivedNs := s.requestInfo.RequestReceivedDuration().Nanoseconds()
	// TODO: health check should not count in stream
	if s.requestInfo.IsHealthCheck() {
		s.proxy.stats.DownstreamRequestTotal.Dec(1)
		s.proxy.listenerStats.DownstreamRequestTotal.Dec(1)
	} else {
		processTime := requestReceivedNs // if no response, ignore the network
		if responseReceivedNs > 0 {
			processTime = requestReceivedNs + (streamDurationNs - responseReceivedNs)
		}

		s.proxy.stats.DownstreamProcessTime.Update(processTime)
		s.proxy.stats.DownstreamProcessTimeTotal.Inc(processTime)

		s.proxy.listenerStats.DownstreamProcessTime.Update(processTime)
		s.proxy.listenerStats.DownstreamProcessTimeTotal.Inc(processTime)

		s.proxy.stats.DownstreamRequestTime.Update(streamDurationNs)
		s.proxy.stats.DownstreamRequestTimeTotal.Inc(streamDurationNs)

		s.proxy.listenerStats.DownstreamRequestTime.Update(streamDurationNs)
		s.proxy.listenerStats.DownstreamRequestTimeTotal.Inc(streamDurationNs)

		s.proxy.stats.DownstreamUpdateRequestCode(s.requestInfo.ResponseCode())
		s.proxy.listenerStats.DownstreamUpdateRequestCode(s.requestInfo.ResponseCode())

		if s.isRequestFailed() {
			s.proxy.stats.DownstreamRequestFailed.Inc(1)
			s.proxy.listenerStats.DownstreamRequestFailed.Inc(1)
		}

		s.requestInfo.SetProcessTimeDuration(time.Duration(processTime))

	}
	// countdown metrics
	s.proxy.stats.DownstreamRequestActive.Dec(1)
	s.proxy.listenerStats.DownstreamRequestActive.Dec(1)
}

// isRequestFailed marks request failed due to mosn process
func (s *downStream) isRequestFailed() bool {
	return s.requestInfo.GetResponseFlag(types.MosnProcessFailedFlags)
}

func (s *downStream) writeLog() {

	defer func() {
		if r := recover(); r != nil {
			log.Proxy.Alertf(s.context, types.ErrorKeyProxyPanic, "[proxy] [downstream] writeLog panic %v, downstream %+v", r, s)
		}
	}()

	if !atomic.CompareAndSwapUint32(&s.logDone, 0, 1) {
		return
	}
	// proxy access log
	if s.proxy != nil && s.proxy.accessLogs != nil {
		for _, al := range s.proxy.accessLogs {
			al.Log(s.context, s.downstreamReqHeaders, s.downstreamRespHeaders, s.requestInfo)
		}
	}

	// per-stream access log
	s.streamFilterChain.Log(s.context, s.downstreamReqHeaders, s.downstreamRespHeaders, s.requestInfo)
}

func (s *downStream) delete() {
	if s.proxy != nil {
		s.proxy.deleteActiveStream(s)
	}
}

// types.StreamEventListener
// Called by stream layer normally
func (s *downStream) OnResetStream(reason types.StreamResetReason) {
	if !atomic.CompareAndSwapUint32(&s.downstreamReset, 0, 1) {
		return
	}
	if log.DefaultLogger.GetLogLevel() >= log.WARN {
		log.DefaultLogger.Warnf("[downStream] reset stream reason %v", reason)
	}
	s.resetReason.Store(reason)

	s.sendNotify()
}

func (s *downStream) ResetStream(reason types.StreamResetReason) {
	s.proxy.stats.DownstreamRequestReset.Inc(1)
	s.proxy.listenerStats.DownstreamRequestReset.Inc(1)
	// we assume downstream client close the connection when timeout, we do not care about the network makes connection closed.
	s.requestInfo.SetResponseCode(api.TimeoutExceptionCode)
	s.cleanStream()
}

func (s *downStream) OnDestroyStream() {}

// types.StreamReceiveListener
func (s *downStream) OnReceive(ctx context.Context, headers types.HeaderMap, data types.IoBuffer, trailers types.HeaderMap) {
	s.downstreamReqHeaders = headers
	s.context = mosnctx.WithValue(s.context, types.ContextKeyDownStreamHeaders, headers)
	s.downstreamReqDataBuf = data
	s.downstreamReqTrailers = trailers
	s.tracks = track.TrackBufferByContext(ctx).Tracks

	if log.Proxy.GetLogLevel() >= log.DEBUG {
		log.Proxy.Debugf(s.context, "[proxy] [downstream] OnReceive headers:%+v, data:%+v, trailers:%+v", headers, data, trailers)
	}

	id := atomic.LoadUint32(&s.ID)
	var task = func() {
		defer func() {
			if r := recover(); r != nil {
				log.Proxy.Alertf(s.context, types.ErrorKeyProxyPanic, "[proxy] [downstream] OnReceive panic: %v, downstream: %+v, oldId: %d, newId: %d\n%s",
					r, s, id, s.ID, string(debug.Stack()))

				if id == atomic.LoadUint32(&s.ID) {
					s.cleanStream()
				}
			}
		}()

		phase := types.InitPhase
		for i := 0; i < 10; i++ {
			s.cleanNotify()

			phase = s.receive(s.context, id, phase)
			switch phase {
			case types.End:
				return
			case types.MatchRoute:
				if log.Proxy.GetLogLevel() >= log.DEBUG {
					log.Proxy.Debugf(s.context, "[proxy] [downstream] redo match route %+v", s)
				}
			case types.Retry:
				if log.Proxy.GetLogLevel() >= log.DEBUG {
					log.Proxy.Debugf(s.context, "[proxy] [downstream] retry %+v", s)
				}
			case types.UpFilter:
				if log.Proxy.GetLogLevel() >= log.DEBUG {
					log.Proxy.Debugf(s.context, "[proxy] [downstream] directResponse %+v", s)
				}
			}
		}
	}

	if s.proxy.serverStreamConn.EnableWorkerPool() {
		// should enable workerpool
		// goroutine for proxy
		if s.proxy.workerpool != nil {
			// use the worker pool for current proxy
			// NOTE: should this be configurable?
			// eg, use config to control Schedule or to ScheduleAuto
			s.proxy.workerpool.Schedule(task)
		} else {
			// use the global shared worker pool
			pool.ScheduleAuto(task)
		}
		return
	}

	task()
	return

}

func (s *downStream) printPhaseInfo(phaseId types.Phase, proxyId uint32) {
	if log.Proxy.GetLogLevel() >= log.DEBUG {
		log.Proxy.Debugf(s.context, "[proxy] [downstream] enter phase %+v[%d], proxyId = %d  ", types.PhaseName[phaseId], phaseId, proxyId)
	}
}

func (s *downStream) receive(ctx context.Context, id uint32, phase types.Phase) types.Phase {
	for i := 0; i <= int(types.End-types.InitPhase); i++ {
		s.phase = phase

		switch phase {
		// init phase
		case types.InitPhase:
			s.printPhaseInfo(phase, id)
			phase++

		// downstream filter before route
		case types.DownFilter:
			s.printPhaseInfo(phase, id)
			s.tracks.StartTrack(track.StreamFilterBeforeRoute)

			s.streamFilterChain.RunReceiverFilter(s.context, api.BeforeRoute,
				s.downstreamReqHeaders, s.downstreamReqDataBuf, s.downstreamReqTrailers, s.receiverFilterStatusHandler)
			s.tracks.EndTrack(track.StreamFilterBeforeRoute)

			if p, err := s.processError(id); err != nil {
				return p
			}
			phase++

		// match route
		case types.MatchRoute:
			s.printPhaseInfo(phase, id)

			s.tracks.StartTrack(track.MatchRoute)
			s.matchRoute()
			s.tracks.EndTrack(track.MatchRoute)

			if p, err := s.processError(id); err != nil {
				return p
			}
			phase++

		// downstream filter after route
		case types.DownFilterAfterRoute:
			s.printPhaseInfo(phase, id)

			s.tracks.StartTrack(track.StreamFilterAfterRoute)
			s.streamFilterChain.RunReceiverFilter(s.context, api.AfterRoute,
				s.downstreamReqHeaders, s.downstreamReqDataBuf, s.downstreamReqTrailers, s.receiverFilterStatusHandler)
			s.tracks.EndTrack(track.StreamFilterAfterRoute)

			if p, err := s.processError(id); err != nil {
				return p
			}
			phase++

		// TODO support retry
		// downstream choose host
		case types.ChooseHost:
			s.printPhaseInfo(phase, id)

			s.tracks.StartTrack(track.LoadBalanceChooseHost)
			s.chooseHost(s.downstreamReqDataBuf == nil && s.downstreamReqTrailers == nil)
			s.tracks.EndTrack(track.LoadBalanceChooseHost)

			if p, err := s.processError(id); err != nil {
				return p
			}
			phase++

		// downstream filter after choose host
		case types.DownFilterAfterChooseHost:
			s.printPhaseInfo(phase, id)

			s.tracks.StartTrack(track.StreamFilterAfterChooseHost)
			s.streamFilterChain.RunReceiverFilter(s.context, api.AfterChooseHost,
				s.downstreamReqHeaders, s.downstreamReqDataBuf, s.downstreamReqTrailers, s.receiverFilterStatusHandler)
			s.tracks.EndTrack(track.StreamFilterAfterChooseHost)

			if p, err := s.processError(id); err != nil {
				return p
			}
			phase++

		// downstream receive header
		case types.DownRecvHeader:
			if s.downstreamReqHeaders != nil {
				s.printPhaseInfo(phase, id)
				s.receiveHeaders(s.downstreamReqDataBuf == nil && s.downstreamReqTrailers == nil)

				if p, err := s.processError(id); err != nil {
					return p
				}
			}
			phase++

		// downstream receive data
		case types.DownRecvData:
			if s.downstreamReqDataBuf != nil {
				s.printPhaseInfo(phase, id)
				s.downstreamReqDataBuf.Count(1)
				s.receiveData(s.downstreamReqTrailers == nil)

				if p, err := s.processError(id); err != nil {
					return p
				}
			}
			phase++

		// downstream receive trailer
		case types.DownRecvTrailer:
			if s.downstreamReqTrailers != nil {
				s.printPhaseInfo(phase, id)
				s.receiveTrailers()

				if p, err := s.processError(id); err != nil {
					return p
				}
			}
			phase++

		// downstream oneway
		case types.Oneway:
			if s.oneway {
				s.printPhaseInfo(phase, id)
				s.cleanStream()

				// downstreamCleaned has set, return types.End
				if p, err := s.processError(id); err != nil {
					return p
				}
			}

			// no oneway, skip types.Retry
			phase = types.WaitNotify

		// retry request
		case types.Retry:
			s.printPhaseInfo(phase, id)
			if s.downstreamReqDataBuf != nil {
				s.downstreamReqDataBuf.Count(1)
			}
			s.doRetry()
			if p, err := s.processError(id); err != nil {
				return p
			}
			phase++

		// wait for upstreamRequest or reset
		case types.WaitNotify:
			s.printPhaseInfo(phase, id)
			if p, err := s.waitNotify(id); err != nil {
				return p
			}

			if log.Proxy.GetLogLevel() >= log.DEBUG {
				log.Proxy.Debugf(s.context, "[proxy] [downstream] OnReceive send downstream response %+v", s.downstreamRespHeaders)
			}

			phase++

		// upstream filter
		case types.UpFilter:
			s.printPhaseInfo(phase, id)

			s.tracks.StartTrack(track.StreamSendFilter)
			s.streamFilterChain.RunSenderFilter(s.context, api.BeforeSend,
				s.downstreamRespHeaders, s.downstreamRespDataBuf, s.downstreamRespTrailers, s.senderFilterStatusHandler)
			s.tracks.EndTrack(track.StreamSendFilter)

			if p, err := s.processError(id); err != nil {
				return p
			}

			// maybe direct response
			if s.upstreamRequest == nil {
				fakeUpstreamRequest := &upstreamRequest{
					downStream: s,
				}

				s.upstreamRequest = fakeUpstreamRequest
			}

			phase++

		// upstream receive header
		case types.UpRecvHeader:
			// send downstream response
			if s.downstreamRespHeaders != nil {
				s.printPhaseInfo(phase, id)

				s.context = mosnctx.WithValue(s.context, types.ContextKeyDownStreamRespHeaders, s.downstreamRespHeaders)
				s.upstreamRequest.receiveHeaders(s.downstreamRespDataBuf == nil && s.downstreamRespTrailers == nil)

				if p, err := s.processError(id); err != nil {
					return p
				}
			}
			phase++

		// upstream receive data
		case types.UpRecvData:
			if s.downstreamRespDataBuf != nil {
				s.printPhaseInfo(phase, id)
				s.upstreamRequest.receiveData(s.downstreamRespTrailers == nil)

				if p, err := s.processError(id); err != nil {
					return p
				}
			}
			phase++

		// upstream receive triler
		case types.UpRecvTrailer:
			if s.downstreamRespTrailers != nil {
				s.printPhaseInfo(phase, id)
				s.upstreamRequest.receiveTrailers()

				if p, err := s.processError(id); err != nil {
					return p
				}
			}
			phase++

		// process end
		case types.End:
			s.printPhaseInfo(phase, id)
			return types.End

		default:
			log.Proxy.Errorf(s.context, "[proxy] [downstream] unexpected phase: %d", phase)
			return types.End
		}
	}

	log.Proxy.Errorf(s.context, "[proxy] [downstream] unexpected phase cycle time")
	return types.End
}

func (s *downStream) matchRoute() {
	headers := s.downstreamReqHeaders
	if s.proxy.routersWrapper == nil || s.proxy.routersWrapper.GetRouters() == nil {
		log.Proxy.Alertf(s.context, types.ErrorKeyRouteMatch, "routersWrapper or routers in routersWrapper is nil while trying to get router")
		s.requestInfo.SetResponseFlag(api.NoRouteFound)
		s.sendHijackReply(api.RouterUnavailableCode, headers)
		return
	}

	// get router instance and do routing
	routers := s.proxy.routersWrapper.GetRouters()
	// call route handler to get route info
	s.snapshot, s.route = s.proxy.routeHandlerFactory.DoRouteHandler(s.context, headers, routers, s.proxy.clusterManager)

	// set RouteEntry so that it can be accessed in stream filters of api.AfterRoute phase.
	if s.route != nil {
		s.requestInfo.SetRouteEntry(s.route.RouteRule())
	}
}

func (s *downStream) convertProtocol() (dp, up types.ProtocolName) {
	dp = s.getDownstreamProtocol()
	up = s.getUpstreamProtocol()
	return
}

// used for adding stream filters.
func (s *downStream) getStreamFilterChainRegisterCallback() api.StreamFilterChainFactoryCallbacks {
	return &s.streamFilterChain
}

func (s *downStream) getDownstreamProtocol() (prot types.ProtocolName) {
	if s.proxy.serverStreamConn == nil {
		prot = types.ProtocolName(s.proxy.config.DownstreamProtocol)
	} else {
		prot = s.proxy.serverStreamConn.Protocol()
	}
	return prot
}

func (s *downStream) getUpstreamProtocol() (currentProtocol types.ProtocolName) {
	configProtocol := s.proxy.config.UpstreamProtocol

	// if route exists upstream protocol, it will replace the proxy config's upstream protocol
	if s.route != nil && s.route.RouteRule() != nil && s.route.RouteRule().UpstreamProtocol() != "" {
		configProtocol = s.route.RouteRule().UpstreamProtocol()
	}

	// Auto means same as downstream protocol
	if configProtocol == string(protocol.Auto) {
		currentProtocol = s.getDownstreamProtocol()
	} else {
		currentProtocol = types.ProtocolName(configProtocol)
	}

	return currentProtocol
}

// getStringOr returns the first argument if it is not empty, otherwise the second.
func getStringOr(s string, defVal string) string {
	if len(s) != 0 {
		return s
	}
	return defVal
}

func (s *downStream) chooseHost(endStream bool) {

	s.downstreamRecvDone = endStream

	// after stream filters run, check the route
	if s.route == nil {
		if log.Proxy.GetLogLevel() >= log.WARN {
			log.Proxy.Warnf(s.context, "[proxy] [downstream] no route to init upstream")
		}
		s.requestInfo.SetResponseFlag(api.NoRouteFound)
		s.sendHijackReply(api.RouterUnavailableCode, s.downstreamReqHeaders)
		return
	}
	// check if route have direct response
	// direct response will response now
	if resp := s.route.DirectResponseRule(); !(resp == nil || reflect.ValueOf(resp).IsNil()) {
		if log.Proxy.GetLogLevel() >= log.INFO {
			log.Proxy.Infof(s.context, "[proxy] [downstream] direct response, proxyId = %d", s.ID)
		}
		if resp.Body() != "" {
			s.sendHijackReplyWithBody(resp.StatusCode(), s.downstreamReqHeaders, resp.Body())
		} else {
			s.sendHijackReply(resp.StatusCode(), s.downstreamReqHeaders)
		}
		return
	}

	if rule := s.route.RedirectRule(); rule != nil {
		if log.Proxy.GetLogLevel() >= log.INFO {
			log.Proxy.Infof(s.context, "[proxy] [downstream] redirect response, proxyId = %d", s.ID)
		}
		currentScheme, err := variable.GetProtocolResource(s.context, api.SCHEME)
		if err != nil {
			log.Proxy.Errorf(s.context, "get protocol resource scheme: %s", err)
			s.sendHijackReply(nethttp.StatusInternalServerError, s.downstreamReqHeaders)
			return
		}
		getValueFunc := func(key string, defaultVal string) string {
			val, err := variable.GetString(s.context, key)
			if err != nil || val == "" {
				return defaultVal
			}
			return val
		}
		currentHost := getValueFunc(types.VarHost, "")
		currentPath := getValueFunc(types.VarPath, "")
		currentQuery := getValueFunc(types.VarQueryString, "")

		u := url.URL{
			Scheme:   getStringOr(rule.RedirectScheme(), currentScheme),
			Host:     getStringOr(rule.RedirectHost(), currentHost),
			Path:     getStringOr(rule.RedirectPath(), currentPath),
			RawQuery: currentQuery,
		}
		if u.Scheme != currentScheme {
			// The port in the host needs to be removed if:
			// 1. original scheme is http and the port is explicitly set to 80
			// 2. original scheme is https and the port is explicitly set to 443
			host, port, err := net.SplitHostPort(u.Host)
			if err == nil {
				if (u.Scheme == "http" && port == "443") ||
					(u.Scheme == "https" && port == "80") {
					u.Host = host
				}
			}
		}
		s.downstreamReqHeaders.Set("location", u.String())
		s.sendHijackReply(rule.RedirectCode(), s.downstreamReqHeaders)
		return
	}

	// not direct response, needs a cluster snapshot and route rule
	if rule := s.route.RouteRule(); rule == nil || reflect.ValueOf(rule).IsNil() {
		if log.Proxy.GetLogLevel() >= log.WARN {
			log.Proxy.Warnf(s.context, "[proxy] [downstream] no route rule to init upstream")
		}
		s.requestInfo.SetResponseFlag(api.NoRouteFound)
		s.sendHijackReply(api.RouterUnavailableCode, s.downstreamReqHeaders)
		return
	}
	if s.snapshot == nil || reflect.ValueOf(s.snapshot).IsNil() {
		// no available cluster
		log.Proxy.Alertf(s.context, types.ErrorKeyClusterGet, " cluster snapshot is nil, cluster name is: %s", s.route.RouteRule().ClusterName(s.context))
		s.requestInfo.SetResponseFlag(api.NoRouteFound)
		s.sendHijackReply(api.RouterUnavailableCode, s.downstreamReqHeaders)
		return
	}
	// as ClusterName has random factor when choosing weighted cluster,
	// so need determination at the first time
	clusterName := s.route.RouteRule().ClusterName(s.context)
	if log.Proxy.GetLogLevel() >= log.DEBUG {
		log.Proxy.Debugf(s.context, "[proxy] [downstream] route match result:%+v, clusterName=%v", s.route, clusterName)
	}

	s.cluster = s.snapshot.ClusterInfo()

	host, pool, err := s.initializeUpstreamConnectionPool(s)
	if err != nil {
		log.Proxy.Alertf(s.context, types.ErrorKeyUpstreamConn, "initialize Upstream Connection Pool error, request can't be proxyed, error = %v", err)
		s.requestInfo.SetResponseFlag(api.NoHealthyUpstream)
		s.sendHijackReply(api.NoHealthUpstreamCode, s.downstreamReqHeaders)
		return
	}

	parseProxyTimeout(s.context, &s.timeout, s.route, s.downstreamReqHeaders)

	if log.Proxy.GetLogLevel() >= log.DEBUG {
		log.Proxy.Debugf(s.context, "[proxy] [downstream] timeout info: %+v", s.timeout)
	}

	prot := s.getUpstreamProtocol()

	s.retryState = newRetryState(s.route.RouteRule().Policy().RetryPolicy(), s.downstreamReqHeaders, s.cluster, prot)

	// Build Request
	proxyBuffers := proxyBuffersByContext(s.context)
	s.upstreamRequest = &proxyBuffers.request
	s.upstreamRequest.downStream = s
	s.upstreamRequest.proxy = s.proxy
	s.upstreamRequest.protocol = prot
	s.upstreamRequest.connPool = pool
	s.upstreamRequest.host = host
}

func (s *downStream) receiveHeaders(endStream bool) {

	// Modify request headers
	s.route.RouteRule().FinalizeRequestHeaders(s.context, s.downstreamReqHeaders, s.requestInfo)
	// Call upstream's append header method to build upstream's request
	s.upstreamRequest.appendHeaders(endStream)

	if endStream {
		s.onUpstreamRequestSent()
	}
}

func (s *downStream) receiveData(endStream bool) {
	// if active stream finished before receive data, just ignore further data
	if s.processDone() {
		return
	}

	data := s.downstreamReqDataBuf
	if log.Proxy.GetLogLevel() >= log.DEBUG {
		log.Proxy.Debugf(s.context, "[proxy] [downstream] receive data = %v", data)
	}

	s.requestInfo.SetBytesReceived(s.requestInfo.BytesReceived() + uint64(data.Len()))
	s.downstreamRecvDone = endStream

	if endStream {
		s.onUpstreamRequestSent()
	}

	s.upstreamRequest.appendData(endStream)

	// if upstream process done in the middle of receiving data, just end stream
	if s.upstreamProcessDone.Load() {
		s.cleanStream()
	}
}

func (s *downStream) receiveTrailers() {
	// if active stream finished the lifecycle, just ignore further data
	if s.processDone() {
		return
	}

	s.downstreamRecvDone = true

	s.onUpstreamRequestSent()
	s.upstreamRequest.appendTrailers()

	// if upstream process done in the middle of receiving trailers, just end stream
	if s.upstreamProcessDone.Load() {
		s.cleanStream()
	}
}

func (s *downStream) OnDecodeError(context context.Context, err error, headers types.HeaderMap) {
	// if active stream finished the lifecycle, just ignore further data
	if s.upstreamProcessDone.Load() {
		return
	}

	// todo: enrich headers' information to do some hijack
	// Check headers' info to do hijack
	switch err.Error() {
	case types.CodecException:
		s.sendHijackReply(api.CodecExceptionCode, headers)
	case types.DeserializeException:
		s.sendHijackReply(api.DeserialExceptionCode, headers)
	default:
		s.sendHijackReply(api.UnknownCode, headers)
	}
}

func (s *downStream) onUpstreamRequestSent() {
	s.upstreamRequestSent = true
	s.requestInfo.SetRequestReceivedDuration(time.Now())

	if s.upstreamRequest != nil && !s.oneway {
		// setup per req timeout timer
		s.setupPerReqTimeout()

		// setup global timeout timer
		if s.timeout.GlobalTimeout > 0 {
			if log.Proxy.GetLogLevel() >= log.DEBUG {
				log.Proxy.Debugf(s.context, "[proxy] [downstream] start a request timeout timer")
			}
			if s.responseTimer != nil {
				s.responseTimer.Stop()
			}

			ID := atomic.LoadUint32(&s.ID)
			s.responseTimer = utils.NewTimer(s.timeout.GlobalTimeout,
				func() {
					// When a stream trigger timeout, this function will be called,
					// but maybe concurrent with upstream response received.
					// So we set the reuse buffer first to make sure this stream will not be
					// reused for another new stream.
					// And then, we check if the stream is cleaned.
					// If the stream is not cleaned, there are two cases:
					// 1. the stream is really not cleaned.
					// 2. the stream has been resetted
					// If case 2 occurs, the ID is not matched, so we check the ID.
					// The stream cleaned check must be called before ID check, otherwise the stream may
					// be cleand after ID check pass.
					atomic.StoreUint32(&s.reuseBuffer, 0)
					if atomic.LoadUint32(&s.downstreamCleaned) == 1 {
						return
					}
					if ID != atomic.LoadUint32(&s.ID) {
						return
					}
					// After ID checks passed, and we setted the reuse buffer flag,
					// the stream will never be reused. so we can do this CAS checks.
					if !atomic.CompareAndSwapUint32(&s.upstreamResponseReceived, 0, 1) {
						return
					}
					s.onResponseTimeout()
				})
		}
	}
}

// Note: global-timer MUST be stopped before active stream got recycled, otherwise resetting stream's properties will cause panic here
func (s *downStream) onResponseTimeout() {
	defer func() {
		if r := recover(); r != nil {
			log.Proxy.Alertf(s.context, types.ErrorKeyProxyPanic, "[proxy] [downstream] onResponseTimeout() panic %v\n%s", r, string(debug.Stack()))
		}
	}()
	s.cluster.Stats().UpstreamRequestTimeout.Inc(1)

	if s.upstreamRequest != nil {
		if s.upstreamRequest.host != nil {
			s.upstreamRequest.host.HostStats().UpstreamRequestTimeout.Inc(1)

			if log.Proxy.GetLogLevel() >= log.INFO {
				log.Proxy.Infof(s.context, "[proxy] [downstream] onResponseTimeout，host: %s, time: %s",
					s.upstreamRequest.host.AddressString(), s.timeout.GlobalTimeout.String())
			}
		}

		s.upstreamRequest.resetStream()
		s.upstreamRequest.OnResetStream(types.UpstreamGlobalTimeout)
	}
}

func (s *downStream) setupPerReqTimeout() {
	timeout := s.timeout

	if timeout.TryTimeout > 0 {
		if s.perRetryTimer != nil {
			s.perRetryTimer.Stop()
		}

		ID := atomic.LoadUint32(&s.ID)
		s.perRetryTimer = utils.NewTimer(timeout.TryTimeout,
			func() {
				atomic.StoreUint32(&s.reuseBuffer, 0)

				if atomic.LoadUint32(&s.downstreamCleaned) == 1 {
					return
				}
				if ID != atomic.LoadUint32(&s.ID) {
					return
				}
				if !atomic.CompareAndSwapUint32(&s.upstreamResponseReceived, 0, 1) {
					return
				}
				s.onPerReqTimeout()
			})
	}
}

// Note: per-try-timer MUST be stopped before active stream got recycled, otherwise resetting stream's properties will cause panic here
func (s *downStream) onPerReqTimeout() {
	defer func() {
		if r := recover(); r != nil {
			log.Proxy.Alertf(s.context, types.ErrorKeyProxyPanic, "[proxy] [downstream] onPerReqTimeout() panic %v\n%s", r, string(debug.Stack()))
		}
	}()

	if !s.downstreamResponseStarted {
		// handle timeout on response not

		s.cluster.Stats().UpstreamRequestTimeout.Inc(1)

		if s.upstreamRequest.host != nil {
			s.upstreamRequest.host.HostStats().UpstreamRequestTimeout.Inc(1)

			log.Proxy.Errorf(s.context, "[proxy] [downstream] onPerReqTimeout，host: %s, time: %s",
				s.upstreamRequest.host.AddressString(), s.timeout.TryTimeout.String())
		}

		s.upstreamRequest.resetStream()
		s.requestInfo.SetResponseFlag(api.UpstreamRequestTimeout)
		s.upstreamRequest.OnResetStream(types.UpstreamPerTryTimeout)

		return
	}

	if log.Proxy.GetLogLevel() >= log.DEBUG {
		log.Proxy.Debugf(s.context, "[proxy] [downstream] skip request timeout on getting upstream response")
	}
}

func (s *downStream) initializeUpstreamConnectionPool(lbCtx types.LoadBalancerContext) (types.Host, types.ConnectionPool, error) {
	var (
		host     types.Host
		connPool types.ConnectionPool
	)

	currentProtocol := s.getUpstreamProtocol()

	connPool, host = s.proxy.clusterManager.ConnPoolForCluster(lbCtx, s.snapshot, currentProtocol)

	if connPool == nil {
		return nil, nil, fmt.Errorf("[proxy] [downstream] no healthy upstream in cluster %s", s.cluster.Name())
	}

	s.requestInfo.OnUpstreamHostSelected(host)
	s.requestInfo.SetUpstreamLocalAddress(host.AddressString())

	// TODO: update upstream stats

	return host, connPool, nil
}

// ~~~ active stream sender wrapper

func (s *downStream) appendHeaders(endStream bool) {
	s.upstreamProcessDone.Store(endStream)
	headers := s.convertHeader(s.downstreamRespHeaders)
	// Currently, just log the error
	if err := s.responseSender.AppendHeaders(s.context, headers, endStream); err != nil {
		log.Proxy.Errorf(s.context, "append headers error: %s", err)
	}

	if endStream {
		s.endStream()
	}
}

func (s *downStream) convertHeader(headers types.HeaderMap) types.HeaderMap {
	if s.noConvert {
		return headers
	}

	dp, up := s.convertProtocol()

	// need protocol convert
	if dp != up {
		if convHeader, err := protocol.ConvertHeader(s.context, up, dp, headers); err == nil {
			return convHeader
		} else {
			if log.Proxy.GetLogLevel() >= log.WARN {
				log.Proxy.Warnf(s.context, "[proxy] [downstream] convert header from %s to %s failed, %s", up, dp, err.Error())
			}
		}
	}
	return headers
}

func (s *downStream) appendData(endStream bool) {
	s.upstreamProcessDone.Store(endStream)

	data := s.convertData(s.downstreamRespDataBuf)
	s.requestInfo.SetBytesSent(s.requestInfo.BytesSent() + uint64(data.Len()))
	s.responseSender.AppendData(s.context, data, endStream)

	if endStream {
		s.endStream()
	}
}

func (s *downStream) convertData(data types.IoBuffer) types.IoBuffer {
	if s.noConvert {
		return data
	}

	dp, up := s.convertProtocol()

	// need protocol convert
	if dp != up {
		if convData, err := protocol.ConvertData(s.context, up, dp, data); err == nil {
			return convData
		} else {
			if log.Proxy.GetLogLevel() >= log.WARN {
				log.Proxy.Warnf(s.context, "[proxy] [downstream] convert data from %s to %s failed, %s", up, dp, err.Error())
			}
		}
	}
	return data
}

func (s *downStream) appendTrailers() {
	s.upstreamProcessDone.Store(true)
	trailers := s.convertTrailer(s.downstreamRespTrailers)
	s.responseSender.AppendTrailers(s.context, trailers)
	s.endStream()
}

func (s *downStream) convertTrailer(trailers types.HeaderMap) types.HeaderMap {
	if s.noConvert {
		return trailers
	}

	dp, up := s.convertProtocol()

	// need protocol convert
	if dp != up {
		if convTrailer, err := protocol.ConvertTrailer(s.context, up, dp, trailers); err == nil {
			return convTrailer
		} else {
			if log.Proxy.GetLogLevel() >= log.WARN {
				log.Proxy.Warnf(s.context, "[proxy] [downstream] convert header from %s to %s failed, %s", up, dp, err.Error())
			}
		}
	}
	return trailers
}

// ~~~ upstream event handler
func (s *downStream) onUpstreamReset(reason types.StreamResetReason) {
	// todo: update stats
	// see if we need a retry
	if reason != types.UpstreamGlobalTimeout &&
		!s.downstreamResponseStarted && s.retryState != nil {
		retryCheck := s.retryState.retry(s.context, nil, reason)

		if retryCheck == api.ShouldRetry && s.setupRetry(true) {
			if s.upstreamRequest != nil && s.upstreamRequest.host != nil {
				s.upstreamRequest.host.HostStats().UpstreamResponseFailed.Inc(1)
				s.upstreamRequest.host.ClusterInfo().Stats().UpstreamResponseFailed.Inc(1)
			}

			// setup retry timer and return
			// clear reset flag
			if log.Proxy.GetLogLevel() >= log.INFO {
				log.Proxy.Infof(s.context, "[proxy] [downstream] onUpstreamReset, doRetry, reason %v", reason)
			}
			atomic.CompareAndSwapUint32(&s.upstreamReset, 1, 0)
			return
		}
		if retryCheck == api.RetryOverflow {
			s.requestInfo.SetResponseFlag(api.UpstreamOverflow)
		}
	}

	// clean up all timers
	s.cleanUp()

	// If we have not yet sent anything downstream, send a response with an appropriate status code.
	// Otherwise just reset the ongoing response.
	if s.downstreamResponseStarted {
		s.resetStream()
	} else {
		// send err response if response not started
		var code int

		reasonFlag := s.proxy.streamResetReasonToResponseFlag(reason)
		s.requestInfo.SetResponseFlag(reasonFlag)
		code = types.ConvertReasonToCode(reason)

		// clear reset flag
		if log.Proxy.GetLogLevel() >= log.INFO {
			log.Proxy.Infof(s.context, "[proxy] [downstream] onUpstreamReset, send hijack, reason %v", reason)
		}
		atomic.CompareAndSwapUint32(&s.upstreamReset, 1, 0)
		s.sendHijackReply(code, s.downstreamReqHeaders)
	}
}

func (s *downStream) onUpstreamHeaders(endStream bool) {
	headers := s.downstreamRespHeaders

	// check retry
	if s.retryState != nil {
		retryCheck := s.retryState.retry(s.context, headers, "")

		if retryCheck == api.ShouldRetry && s.setupRetry(endStream) {
			if s.upstreamRequest != nil && s.upstreamRequest.host != nil {
				s.upstreamRequest.host.HostStats().UpstreamResponseFailed.Inc(1)
				s.upstreamRequest.host.ClusterInfo().Stats().UpstreamResponseFailed.Inc(1)
			}

			return
		}
		if retryCheck == api.RetryOverflow {
			s.requestInfo.SetResponseFlag(api.UpstreamOverflow)
		}

		s.retryState.reset()
	}

	s.handleUpstreamStatusCode()

	s.downstreamResponseStarted = true

	// directResponse for no route should be nil
	if s.route != nil {
		s.route.RouteRule().FinalizeResponseHeaders(s.context, headers, s.requestInfo)
	}

	if endStream {
		s.onUpstreamResponseRecvFinished()
	}

	// todo: insert proxy headers
	s.appendHeaders(endStream)
}

func (s *downStream) handleUpstreamStatusCode() {
	// todo: support config?
	if s.upstreamRequest != nil && s.upstreamRequest.host != nil {
		if s.requestInfo.ResponseCode() >= http.InternalServerError {
			s.upstreamRequest.host.HostStats().UpstreamResponseFailed.Inc(1)
			s.upstreamRequest.host.ClusterInfo().Stats().UpstreamResponseFailed.Inc(1)
		} else {
			s.upstreamRequest.host.HostStats().UpstreamResponseSuccess.Inc(1)
			s.upstreamRequest.host.ClusterInfo().Stats().UpstreamResponseSuccess.Inc(1)
		}
	}
}

func (s *downStream) onUpstreamData(endStream bool) {
	if endStream {
		s.onUpstreamResponseRecvFinished()
	}

	s.appendData(endStream)
}

func (s *downStream) finishTracing() {
	if trace.IsEnabled() {
		if s.context == nil {
			return
		}
		span := trace.SpanFromContext(s.context)

		if span != nil {
			span.SetRequestInfo(s.requestInfo)
			span.FinishSpan()

			if mosnctx.Get(s.context, types.ContextKeyListenerType) == v2.INGRESS {
				trace.DeleteSpanIdGenerator(mosnctx.Get(s.context, types.ContextKeyTraceSpanKey).(*trace.SpanKey))
			}
		} else {
			if log.Proxy.GetLogLevel() >= log.WARN {
				log.Proxy.Warnf(s.context, "[proxy] [downstream] trace span is null")
			}
		}
	}
}

func (s *downStream) onUpstreamTrailers() {
	s.onUpstreamResponseRecvFinished()

	s.appendTrailers()
}

func (s *downStream) onUpstreamResponseRecvFinished() {
	if !s.upstreamRequestSent {
		s.upstreamRequest.resetStream()
	}

	// todo: stats
	// todo: logs

	s.cleanUp()
}

func (s *downStream) setupRetry(endStream bool) bool {
	s.upstreamRequest.setupRetry = true

	if !endStream {
		s.upstreamRequest.resetStream()
	}

	// reset per req timer
	if s.perRetryTimer != nil {
		s.perRetryTimer.Stop()
		s.perRetryTimer = nil
	}

	atomic.CompareAndSwapUint32(&s.upstreamResponseReceived, 1, 0)

	return true
}

// Note: retry-timer MUST be stopped before active stream got recycled, otherwise resetting stream's properties will cause panic here
func (s *downStream) doRetry() {
	// retry interval
	time.Sleep(10 * time.Millisecond)

	// no reuse buffer
	atomic.StoreUint32(&s.reuseBuffer, 0)

	host, pool, err := s.initializeUpstreamConnectionPool(s)

	if err != nil {
		log.Proxy.Alertf(s.context, types.ErrorKeyUpstreamConn, "retry choose conn pool failed, error = %v", err)
		s.sendHijackReply(api.NoHealthUpstreamCode, s.downstreamReqHeaders)
		s.cleanUp()
		return
	}

	s.upstreamRequest = &upstreamRequest{
		downStream: s,
		proxy:      s.proxy,
		connPool:   pool,
		host:       host,
		protocol:   s.getUpstreamProtocol(),
	}

	// if Data or Trailer exists, endStream should be false, else should be true
	s.upstreamRequest.appendHeaders(s.downstreamReqDataBuf == nil && s.downstreamReqTrailers == nil)

	if s.downstreamReqDataBuf != nil {
		s.upstreamRequest.appendData(s.downstreamReqTrailers == nil)
	}

	if s.downstreamReqTrailers != nil {
		s.upstreamRequest.appendTrailers()
	}

	// setup per try timeout timer
	s.setupPerReqTimeout()

	s.upstreamRequestSent = true
	s.downstreamRecvDone = true
}

// Downstream got reset in proxy context on scenario below:
// 1. downstream filter reset downstream
// 2. corresponding upstream got reset
func (s *downStream) resetStream() {
	if s.responseSender != nil && !s.upstreamProcessDone.Load() {
		// if downstream req received not done, or local proxy process not done by handle upstream response,
		// just mark it as done and reset stream as a failed case
		s.upstreamProcessDone.Store(true)

		// reset downstream will trigger a clean up, see OnResetStream
		s.responseSender.GetStream().ResetStream(types.StreamLocalReset)
	}
}

func (s *downStream) sendHijackReply(code int, headers types.HeaderMap) {
	log.Proxy.Warnf(s.context, "[proxy] [downstream] set hijack reply, proxyId = %d, code = %d, with headers = %t", s.ID, code, headers == nil)
	if headers == nil {
		raw := make(map[string]string, 5)
		headers = protocol.CommonHeader(raw)
	}
	s.requestInfo.SetResponseCode(code)
	status := strconv.Itoa(code)
	variable.SetString(s.context, types.VarHeaderStatus, status)
	atomic.StoreUint32(&s.reuseBuffer, 0)
	s.downstreamRespHeaders = headers
	s.downstreamRespDataBuf = nil
	s.downstreamRespTrailers = nil
	s.directResponse = true
}

// TODO: rpc status code may be not matched
// TODO: rpc content(body) is not matched the headers, rpc should not hijack with body, use sendHijackReply instead
func (s *downStream) sendHijackReplyWithBody(code int, headers types.HeaderMap, body string) {
	log.Proxy.Warnf(s.context, "[proxy] [downstream] set hijack reply with body, proxyId = %d, code = %d, with headers = %t", s.ID, code, headers == nil)
	if headers == nil {
		raw := make(map[string]string, 5)
		headers = protocol.CommonHeader(raw)
	}
	s.requestInfo.SetResponseCode(code)

	status := strconv.Itoa(code)
	variable.SetString(s.context, types.VarHeaderStatus, status)

	atomic.StoreUint32(&s.reuseBuffer, 0)
	s.downstreamRespHeaders = headers
	s.downstreamRespDataBuf = buffer.NewIoBufferString(body)
	s.downstreamRespTrailers = nil
	s.directResponse = true
}

func (s *downStream) cleanUp() {
	// reset retry state
	// if  a downstream filter ends downstream before send to upstream, retryState will be nil
	if s.retryState != nil {
		s.retryState.reset()
	}

	// reset pertry timer
	if s.perRetryTimer != nil {
		s.perRetryTimer.Stop()
		s.perRetryTimer = nil
	}

	// reset response timer
	if s.responseTimer != nil {
		s.responseTimer.Stop()
		s.responseTimer = nil
	}

}

func (s *downStream) setBufferLimit(bufferLimit uint32) {
	s.bufferLimit = bufferLimit

	// todo
}

// types.LoadBalancerContext
func (s *downStream) MetadataMatchCriteria() api.MetadataMatchCriteria {
	var varMeta map[string]string
	if v, err := variable.Get(s.context, types.VarRouterMeta); err == nil && v != nil {
		if m, ok := v.(map[string]string); ok {
			varMeta = m
		}
	}

	var routerMeta api.MetadataMatchCriteria
	if s.requestInfo.RouteEntry() != nil {
		routerMeta = s.requestInfo.RouteEntry().MetadataMatchCriteria(s.cluster.Name())
	}

	if varMeta == nil {
		return routerMeta
	}

	if routerMeta != nil {
		for _, kv := range routerMeta.MetadataMatchCriteria() {
			if _, ok := varMeta[kv.MetadataKeyName()]; !ok {
				varMeta[kv.MetadataKeyName()] = kv.MetadataValue()
			}
		}
	}

	return router.NewMetadataMatchCriteriaImpl(varMeta)
}

func (s *downStream) DownstreamConnection() net.Conn {
	return s.proxy.readCallbacks.Connection().RawConn()
}

func (s *downStream) DownstreamHeaders() types.HeaderMap {
	return s.downstreamReqHeaders
}

func (s *downStream) DownstreamContext() context.Context {
	return s.context
}

func (s *downStream) DownstreamCluster() types.ClusterInfo {
	return s.cluster
}

func (s *downStream) DownstreamRoute() api.Route {
	return s.route
}

func (s *downStream) giveStream() {
	if atomic.LoadUint32(&s.reuseBuffer) != 1 {
		return
	}
	if atomic.LoadUint32(&s.upstreamReset) == 1 || atomic.LoadUint32(&s.downstreamReset) == 1 {
		return
	}

	if log.Proxy.GetLogLevel() >= log.DEBUG {
		log.Proxy.Debugf(s.context, "[proxy] [downstream] giveStream %p %+v", s, s)
	}

	// reset downstreamReqBuf
	if s.downstreamReqDataBuf != nil {
		if e := buffer.PutIoBuffer(s.downstreamReqDataBuf); e != nil {
			log.Proxy.Errorf(s.context, "[proxy] [downstream] PutIoBuffer error: %v", e)
		}
	}

	// Give buffers to bufferPool
	if ctx := mbuffer.PoolContext(s.context); ctx != nil {
		ctx.Give()
	}
}

// check if proxy process done
func (s *downStream) processDone() bool {
	return s.upstreamProcessDone.Load() || atomic.LoadUint32(&s.downstreamReset) == 1 || atomic.LoadUint32(&s.upstreamReset) == 1
}

func (s *downStream) sendNotify() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func (s *downStream) cleanNotify() {
	select {
	case <-s.notify:
	default:
	}
}

func (s *downStream) waitNotify(id uint32) (phase types.Phase, err error) {
	if atomic.LoadUint32(&s.ID) != id {
		return types.End, types.ErrExit
	}

	if log.Proxy.GetLogLevel() >= log.DEBUG {
		log.Proxy.Debugf(s.context, "[proxy] [downstream] waitNotify begin %p, proxyId = %d", s, s.ID)
	}
	select {
	case <-s.notify:
	}
	return s.processError(id)
}

func (s *downStream) processError(id uint32) (phase types.Phase, err error) {
	sid := atomic.LoadUint32(&s.ID)
	if sid != id {
		return types.End, types.ErrExit
	}

	phase = types.End

	if atomic.LoadUint32(&s.downstreamCleaned) == 1 {
		err = types.ErrExit
		return
	}

	if atomic.LoadUint32(&s.upstreamReset) == 1 {
		if log.Proxy.GetLogLevel() >= log.INFO {
			log.Proxy.Infof(s.context, "[proxy] [downstream] processError=upstreamReset, proxyId: %d, reason: %+v", sid, s.resetReason.Load())
		}
		if s.oneway {
			phase = types.Oneway
			err = types.ErrExit
			return
		}
		s.onUpstreamReset(s.resetReason.Load())
		err = types.ErrExit
	}

	if atomic.LoadUint32(&s.downstreamReset) == 1 {
		log.Proxy.Errorf(s.context, "[proxy] [downstream] processError=downstreamReset proxyId: %d, reason: %+v", sid, s.resetReason.Load())
		s.ResetStream(s.resetReason.Load())
		err = types.ErrExit
		return
	}

	if s.directResponse {
		variable.SetString(s.context, types.VarProxyIsDirectResponse, types.IsDirectResponse)
		s.directResponse = false

		// don't retry
		s.retryState = nil

		if s.oneway {
			phase = types.Oneway
			err = types.ErrExit
			return
		}
		if s.phase != types.UpFilter {
			phase = types.UpFilter
			err = types.ErrExit
			return
		}
		return
	}

	if s.upstreamProcessDone.Load() {
		err = types.ErrExit
	}

	if s.upstreamRequest != nil && s.upstreamRequest.setupRetry {
		phase = types.Retry
		err = types.ErrExit
		return
	}

	if s.receiverFiltersAgainPhase != types.InitPhase {
		phase = s.receiverFiltersAgainPhase
		err = types.ErrExit
		s.receiverFiltersAgainPhase = types.InitPhase
		return
	}

	return
}
