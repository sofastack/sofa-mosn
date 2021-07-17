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

package main

/*

#include <stdlib.h>
#include <string.h>

#include "api.h"

*/
import "C"

import (
	"reflect"
	"runtime/debug"
	"unsafe"

	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/networkextention/l7/stream"
	"mosn.io/mosn/pkg/protocol"
	"mosn.io/mosn/pkg/types"
	"mosn.io/pkg/buffer"
	"mosn.io/pkg/utils"
)

// sizeof pointer offset
var pointerSize = unsafe.Sizeof(uintptr(0))

// note need to make sure there is no concurrency
var postDecodePointer C.fc
var postEncodePointer C.fc

//export setPostDecodeCallbackFunc
func setPostDecodeCallbackFunc(f C.fc) {
	postDecodePointer = f
}

//export setPostEncodeCallbackFunc
func setPostEncodeCallbackFunc(f C.fc) {
	postEncodePointer = f
}

//export initReqAndResp
func initReqAndResp(req *C.Request, resp *C.Response, headerLen C.size_t, trailerLen C.size_t) {
	defer func() {
		if r := recover(); r != nil {
			log.DefaultLogger.Errorf("[golang_extention] initReqAndResp() panic %v\n%s", r, string(debug.Stack()))
		}
	}()

	// headers -> |key-data|key-len|val-data|val-len|null|
	req.headers = (**C.char)(C.calloc(C.size_t(4*headerLen+1), C.size_t(pointerSize)))
	req.trailers = (**C.char)(C.calloc(C.size_t(4*trailerLen+1), C.size_t(pointerSize)))

	// req_body[0] save request body pointer, req_body[1] save request body length
	req.req_body[0] = nil
	req.req_body[1] = nil

	// filter
	req.filter = nil
	// resp_body[0] save response body pointer, resp_body[1] save response body length
	resp.resp_body[0] = nil
	resp.resp_body[1] = nil
	resp.headers = nil
	resp.trailers = nil
	resp.direct_response = 0
	resp.need_async = 0
}

//export freeReqAndResp
func freeReqAndResp(req *C.Request, resp *C.Response) {
	defer func() {
		if r := recover(); r != nil {
			log.DefaultLogger.Errorf("[golang_extention] freeReqAndResp() panic %v\n%s", r, string(debug.Stack()))
		}
	}()

	if req == nil || resp == nil {
		return
	}

	// free header
	freeCharPointer(req.headers)
	freeCharPointerArray(resp.headers)

	// free trailer
	freeCharPointer(req.trailers)
	freeCharPointerArray(resp.trailers)

	// free body when direct response
	if resp.resp_body[0] != nil {
		C.free(unsafe.Pointer(resp.resp_body[0]))
	}
}

//export runReceiveStreamFilter
func runReceiveStreamFilter(req C.Request) C.Response {
	defer func() {
		if r := recover(); r != nil {
			log.DefaultLogger.Errorf("[golang_extention] runReceiveStreamFilter() panic %v\n%s", r, string(debug.Stack()))
		}
	}()

	// headers
	requestHeader := createStreamHeaderorTrailer(req.headers)

	// body
	var data string
	if req.req_body[0] != nil && req.req_body[1] != nil {
		data = dataToString(req.req_body[0], req.req_body[1])
	}

	// trailers
	requestTrailer := createStreamHeaderorTrailer(req.trailers)

	// create stream
	streamManager := stream.GetStreamManager()
	sm, _ := streamManager.GetOrCreateStreamByID(uint64(req.sid))

	handler := func() {
		sm.InitResuestStream(&requestHeader, buffer.NewIoBufferString(data), &requestTrailer)
		sm.SetConnectionID(uint64(req.cid))
		sm.SetStreamID(uint64(req.sid))

		sm.OnReceive(sm.GetRequestHeaders(), sm.GetRequestData(), sm.GetRequestTrailers())
	}

	needAsync := stream.GetFilterAsyncMod()
	if needAsync {
		// deep copy some memory of request stream member
		// avoid using an invalid pointer in Go when the host terminates prematurely.
		sm.DirectDeepCopyRequestStream()
		utils.GoWithRecover(func() {
			handler()
			postDecode(req.filter, sm)
		}, nil)

	} else {
		handler()
	}

	return convertCRequest(sm, needAsync)
}

//export runSendStreamFilter
func runSendStreamFilter(resp C.Request) C.Response {
	defer func() {
		if r := recover(); r != nil {
			log.DefaultLogger.Errorf("[golang_extention] runSendStreamFilter() panic %v\n%s", r, string(debug.Stack()))
		}
	}()

	// headers
	requestHeader := createStreamHeaderorTrailer(resp.headers)

	// body
	var data string
	if resp.req_body[0] != nil && resp.req_body[1] != nil {
		data = dataToString(resp.req_body[0], resp.req_body[1])
	}

	// trailers
	requestTrailer := createStreamHeaderorTrailer(resp.trailers)

	// stream
	streamManager := stream.GetStreamManager()
	sm, _ := streamManager.GetOrCreateStreamByID(uint64(resp.sid))
	handler := func() {
		sm.InitResponseStream(&requestHeader, buffer.NewIoBufferString(data), &requestTrailer)
		//sm.SetConnectionID(uint64(resp.cid))
		//sm.SetStreamID(uint64(resp.sid))

		sm.OnSend(sm.GetResponseHeaders(), sm.GetResponseData(), sm.GetResponseTrailers())
	}

	needAsync := stream.GetFilterAsyncMod()
	if needAsync {
		// deep copy some memory of response stream member
		// avoid using an invalid pointer in Go when the host terminates prematurely.
		sm.DirectDeepCopyResponseStream()
		utils.GoWithRecover(func() {
			handler()
			postEncode(resp.filter, sm)
		}, nil)

	} else {
		handler()
	}

	return convertCResponse(sm, needAsync)
}

//export cancelPostcallback
func cancelPostcallback(streamID C.ulonglong) {
	streamManager := stream.GetStreamManager()
	sm, _ := streamManager.GetOrCreateStreamByID(uint64(streamID))

	sm.Lock()
	defer sm.Unlock()
	sm.ResetStream(stream.StreamRemoteReset)
}

//export destoryStream
func destoryStream(streamID C.ulonglong) {
	streamManager := stream.GetStreamManager()
	streamManager.DestoryStreamByID(uint64(streamID))
}

func dataToString(data *C.char, lenPointer *C.char) string {
	var s0 string
	var s0Hdr = (*reflect.StringHeader)(unsafe.Pointer(&s0))
	s0Hdr.Data = uintptr(unsafe.Pointer(data))
	s0Hdr.Len = int(uintptr(unsafe.Pointer(lenPointer)))
	return s0
}

func rangeCstring(headers **C.char, f func(k, v string) bool) {
	for *headers != nil {
		kData := headers
		kLen := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(headers)) + pointerSize))
		vData := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(headers)) + 2*pointerSize))
		vLen := (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(headers)) + 3*pointerSize))
		if !f(dataToString(*kData, *kLen), dataToString(*vData, *vLen)) {
			break
		}

		headers = (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(headers)) + (4 * pointerSize)))
	}
}

func rangeMapToChar(header types.HeaderMap, dstHeaders ***C.char) {

	hLen := 0
	header.Range(func(k, v string) bool {
		hLen++
		return true
	})

	*dstHeaders = (**C.char)(C.calloc(C.size_t(4*hLen+1), C.size_t(pointerSize)))

	tempHeader := *dstHeaders
	header.Range(func(k, v string) bool {
		// tempHeaders -> |key-data|key-len|val-data|val-len|null|
		*tempHeader = C.CString(k)
		tempHeader = (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(tempHeader)) + pointerSize))

		*tempHeader = (*C.char)(unsafe.Pointer(uintptr(len(k))))
		tempHeader = (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(tempHeader)) + pointerSize))

		*tempHeader = C.CString(v)
		tempHeader = (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(tempHeader)) + pointerSize))

		*tempHeader = (*C.char)(unsafe.Pointer(uintptr(len(v))))
		tempHeader = (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(tempHeader)) + pointerSize))
		return true
	})
}

func freeCharPointerArray(p **C.char) {
	if p == nil {
		return
	}

	temp := p
	for *temp != nil {
		C.free(unsafe.Pointer(*temp))
		// |key-data|key-len|val-data|val-len|null| -> |heap mem|int|heap mem|int|null|
		temp = (**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(temp)) + 2*pointerSize))
	}

	freeCharPointer(p)
}

func freeCharPointer(p **C.char) {
	if p == nil {
		return
	}

	C.free(unsafe.Pointer(p))
}

func createStreamHeaderorTrailer(headers **C.char) stream.Headers {
	header := make(map[string]string)
	update := make(map[string]string)
	if headers != nil {
		rangeCstring(headers, func(k, v string) bool {
			header[k] = v
			return true
		})
	}

	return stream.Headers{
		CommonHeader: protocol.CommonHeader(header),
		Update:       update,
	}
}

func generateRequest(sm *stream.ActiveStream) C.Response {
	var resp C.Response

	if sm.IsDirectResponse() {
		resp.status = C.int(sm.GetResponseCode())
		resp.direct_response = 1
		if data := sm.GetResponseData(); data != nil && len(data.String()) != 0 {
			resp.resp_body[0] = C.CString(data.String())
			resp.resp_body[1] = (*C.char)(unsafe.Pointer(uintptr(len(data.String()))))
		}
		rangeMapToChar(sm.GetResponseHeaders(), &resp.headers)
		rangeMapToChar(sm.GetResponseTrailers(), &resp.trailers)
		return resp
	}

	rangeMapToChar(sm.GetRequestUpdatedHeaders(), &resp.headers)

	return resp
}

func generateResponse(sm *stream.ActiveStream) C.Response {
	var resp C.Response

	if sm.IsDirectResponse() {
		resp.status = C.int(sm.GetResponseCode())
		resp.direct_response = 1
		if data := sm.GetResponseData(); data != nil && len(data.String()) != 0 {
			resp.resp_body[0] = C.CString(data.String())
			resp.resp_body[1] = (*C.char)(unsafe.Pointer(uintptr(len(data.String()))))
		}
		rangeMapToChar(sm.GetResponseHeaders(), &resp.headers)
		rangeMapToChar(sm.GetResponseTrailers(), &resp.trailers)
		return resp
	}

	rangeMapToChar(sm.GetResponseUpdatedHeaders(), &resp.headers)

	return resp
}

func postDecode(filter unsafe.Pointer, sm *stream.ActiveStream) {
	sm.RLock()
	defer sm.RUnlock()
	if !sm.CheckStreamValid() || postDecodePointer == nil {
		return
	}

	C.runPostCallback(postDecodePointer, filter, generateRequest(sm))
}

func postEncode(filter unsafe.Pointer, sm *stream.ActiveStream) {
	sm.RLock()
	defer sm.RUnlock()
	if !sm.CheckStreamValid() || postEncodePointer == nil {
		return
	}

	C.runPostCallback(postEncodePointer, filter, generateResponse(sm))
}

func convertCRequest(sm *stream.ActiveStream, needAsync bool) C.Response {
	var resp C.Response

	if needAsync {
		resp.need_async = 1
		resp.resp_body[0] = nil
		resp.resp_body[1] = nil
		resp.headers = nil
		resp.trailers = nil
		resp.direct_response = 0
		return resp
	}

	return generateRequest(sm)
}

func convertCResponse(sm *stream.ActiveStream, needAsync bool) C.Response {
	var resp C.Response

	if needAsync {
		resp.need_async = 1
		resp.resp_body[0] = nil
		resp.resp_body[1] = nil
		resp.headers = nil
		resp.trailers = nil
		resp.direct_response = 0
		return resp
	}

	return generateResponse(sm)
}
