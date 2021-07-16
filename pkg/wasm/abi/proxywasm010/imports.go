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

package proxywasm010

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"mosn.io/api"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/protocol"
	"mosn.io/pkg/buffer"
	"mosn.io/proxy-wasm-go-host/common"
	"mosn.io/proxy-wasm-go-host/proxywasm"
)

type DefaultImportsHandler struct {
	proxywasm.DefaultImportsHandler
	Instance common.WasmInstance
	hc       *httpCallout
}

// override
func (d *DefaultImportsHandler) Log(level proxywasm.LogLevel, msg string) proxywasm.WasmResult {
	logFunc := log.DefaultLogger.Infof

	switch level {
	case proxywasm.LogLevelTrace:
		logFunc = log.DefaultLogger.Tracef
	case proxywasm.LogLevelDebug:
		logFunc = log.DefaultLogger.Debugf
	case proxywasm.LogLevelInfo:
		logFunc = log.DefaultLogger.Infof
	case proxywasm.LogLevelWarn:
		logFunc = log.DefaultLogger.Warnf
	case proxywasm.LogLevelError:
		logFunc = log.DefaultLogger.Errorf
	case proxywasm.LogLevelCritical:
		logFunc = log.DefaultLogger.Errorf
	}

	logFunc(msg)

	return proxywasm.WasmResultOk
}

var httpCalloutID int32

type httpCallout struct {
	id         int32
	d          *DefaultImportsHandler
	instance   common.WasmInstance
	abiContext *ABIContext

	urlString  string
	client     *http.Client
	req        *http.Request
	resp       *http.Response
	respHeader api.HeaderMap
	respBody   buffer.IoBuffer
	reqOnFly   bool
}

// override
func (d *DefaultImportsHandler) HttpCall(reqURL string, header common.HeaderMap, body common.IoBuffer,
	trailer common.HeaderMap, timeoutMilliseconds int32) (int32, proxywasm.WasmResult) {
	u, err := url.Parse(reqURL)
	if err != nil {
		log.DefaultLogger.Errorf("[proxywasm010][imports] HttpCall fail to parse url, err: %v, reqURL: %v", err, reqURL)
		return 0, proxywasm.WasmResultBadArgument
	}

	calloutID := atomic.AddInt32(&httpCalloutID, 1)

	d.hc = &httpCallout{
		id:         calloutID,
		d:          d,
		instance:   d.Instance,
		abiContext: d.Instance.GetData().(*ABIContext),
		urlString:  reqURL,
	}

	d.hc.client = &http.Client{Timeout: time.Millisecond * time.Duration(timeoutMilliseconds)}

	d.hc.req, err = http.NewRequest(http.MethodGet, u.String(), buffer.NewIoBufferBytes(body.Bytes()))
	if err != nil {
		log.DefaultLogger.Errorf("[proxywasm010][imports] HttpCall fail to create http req, err: %v, reqURL: %v", err, reqURL)
		return 0, proxywasm.WasmResultInternalFailure
	}

	header.Range(func(key, value string) bool {
		d.hc.req.Header.Add(key, value)
		return true
	})

	d.hc.reqOnFly = true

	return calloutID, proxywasm.WasmResultOk
}

// override
func (d *DefaultImportsHandler) Wait() {
	if d.hc == nil || !d.hc.reqOnFly {
		return
	}

	// release the instance lock and do sync http req
	d.Instance.Unlock()
	resp, err := d.hc.client.Do(d.hc.req)
	d.Instance.Lock(d.hc.abiContext)

	d.hc.reqOnFly = false

	if err != nil {
		log.DefaultLogger.Errorf("[proxywasm010][imports] HttpCall id: %v fail to do http req, err: %v, reqURL: %v",
			d.hc.id, err, d.hc.urlString)
		return
	}
	d.hc.resp = resp

	// process http resp header
	var respHeader api.HeaderMap = protocol.CommonHeader{}
	for key, _ := range resp.Header {
		respHeader.Set(key, resp.Header.Get(key))
	}
	d.hc.respHeader = respHeader

	// process http resp body
	var respBody buffer.IoBuffer
	respBodyLen := 0

	respBodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.DefaultLogger.Errorf("[proxywasm010][imports] HttpCall id: %v fail to read bytes from resp body, err: %v, reqURL: %v",
			d.hc.id, err, d.hc.urlString)
	}

	err = resp.Body.Close()
	if err != nil {
		log.DefaultLogger.Errorf("[proxywasm010][imports] HttpCall id: %v fail to close resp body, err: %v, reqURL: %v",
			d.hc.id, err, d.hc.urlString)
	}

	if respBodyBytes != nil {
		respBody = buffer.NewIoBufferBytes(respBodyBytes)
		respBodyLen = respBody.Len()
	}
	d.hc.respBody = respBody

	// call proxy_on_http_call_response
	rootContextID := d.hc.abiContext.Imports.GetRootContextID()
	exports := d.hc.abiContext.GetExports()

	err = exports.ProxyOnHttpCallResponse(rootContextID, d.hc.id, int32(len(resp.Header)), int32(respBodyLen), 0)
	if err != nil {
		log.DefaultLogger.Errorf("[proxywasm010][imports] httpCall id: %v fail to call ProxyOnHttpCallResponse, err: %v", d.hc.id, err)
	}
}

// override
func (d *DefaultImportsHandler) GetHttpCallResponseHeaders() common.HeaderMap {
	if d.hc != nil && d.hc.respHeader != nil {
		return HeaderMapWrapper{HeaderMap: d.hc.respHeader}
	}

	return nil
}

// override
func (d *DefaultImportsHandler) GetHttpCallResponseBody() common.IoBuffer {
	if d.hc != nil && d.hc.respBody != nil {
		return IoBufferWrapper{IoBuffer: d.hc.respBody}
	}

	return nil
}
