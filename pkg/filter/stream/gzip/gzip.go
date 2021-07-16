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

package gzip

import (
	"context"
	"strings"

	"github.com/valyala/fasthttp"
	"mosn.io/api"
	"mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/types"
	"mosn.io/mosn/pkg/variable"
	"mosn.io/pkg/buffer"
)

/*
 * test first for the most common case "gzip,...":
 *   MSIE:    "gzip, deflate"
 *   Firefox: "gzip,deflate"
 *   Chrome:  "gzip,deflate,sdch"
 *   Safari:  "gzip, deflate"
 *   Opera:   "gzip, deflate"
 */

var gzipCheck = map[string]bool{
	"gzip":              true,
	"gzip, deflate":     true,
	"gzip , deflate":    true,
	"gzip ,deflate":     true,
	"gzip,deflate":      true,
	"gzip,deflate,sdch": true,
}

// gzipConfig is parsed from v2.StreamGzip
// TODO support minLength,ContentType,gziplevel etc.
type gzipConfig struct {
	gzipLevel      uint32
	minCompressLen uint32
	contentType    map[string]bool
}

func makegzipConfig(cfg *v2.StreamGzip) *gzipConfig {

	contentTypes := map[string]bool{}
	for _, ct := range cfg.ContentType {
		contentTypes[strings.ToLower(ct)] = true
	}

	gzipConfig := &gzipConfig{
		contentType:    contentTypes,
		gzipLevel:      cfg.GzipLevel,
		minCompressLen: cfg.ContentLength,
	}

	return gzipConfig
}

// streamGzipFilter is an implement of api.StreamReceiverFilter
type streamGzipFilter struct {
	config         *gzipConfig
	needGzip       bool
	receiveHandler api.StreamReceiverFilterHandler
	sendHandler    api.StreamSenderFilterHandler
}

func NewStreamFilter(ctx context.Context, cfg *v2.StreamGzip) *streamGzipFilter {
	if log.Proxy.GetLogLevel() >= log.DEBUG {
		log.Proxy.Debugf(ctx, "[stream filter] [gzip] create a new gzip filter")
	}
	gf := makegzipConfig(cfg)

	return &streamGzipFilter{
		config: gf,
	}
}

func (f *streamGzipFilter) SetReceiveFilterHandler(handler api.StreamReceiverFilterHandler) {
	f.receiveHandler = handler
}

func (f *streamGzipFilter) OnReceive(ctx context.Context, headers types.HeaderMap, buf types.IoBuffer, trailers types.HeaderMap) api.StreamFilterStatus {
	// check request need gzip
	if !f.checkGzip(ctx, headers) {
		f.needGzip = false
		return api.StreamFilterContinue
	}

	// for response check
	f.needGzip = true
	return api.StreamFilterContinue
}

func (f *streamGzipFilter) SetSenderFilterHandler(handler api.StreamSenderFilterHandler) {
	f.sendHandler = handler
}

func (f *streamGzipFilter) Append(ctx context.Context, headers api.HeaderMap, buf buffer.IoBuffer, trailers api.HeaderMap) api.StreamFilterStatus {
	if log.Proxy.GetLogLevel() >= log.DEBUG {
		log.Proxy.Debugf(ctx, "[stream filter] [gzip] gzip filter do append headers")
	}

	if buf == nil || buf.Len() <= 0 {
		return api.StreamFilterContinue
	}

	_, gziped := headers.Get(strContentEncoding)
	// client don't need gzip or the body is already compressed
	if !f.needGzip || gziped {
		return api.StreamFilterContinue
	}

	// There is no sense in spending CPU time on small body compression,
	// since there is a very high probability that the compressed
	// body size will be bigger than the original body size.
	if uint32(buf.Len()) < f.config.minCompressLen {
		return api.StreamFilterContinue
	}

	// check gzip contentType
	// default gzip contentTypes is "text/html"
	if !f.isCompressibleContentType(headers) {
		return api.StreamFilterContinue
	}

	// set gzip response header
	headers.Set(strContentEncoding, "gzip")

	// usually gzip compression ratio is 3-10 times
	outBuf := buffer.GetIoBuffer(buf.Len() / 3)

	fasthttp.WriteGzipLevel(outBuf, buf.Bytes(), int(f.config.gzipLevel))
	f.sendHandler.SetResponseData(outBuf)

	buffer.PutIoBuffer(outBuf)

	return api.StreamFilterContinue
}

func (f *streamGzipFilter) OnDestroy() {
}

// check request need gzip
func (f *streamGzipFilter) checkGzip(ctx context.Context, headers types.HeaderMap) bool {
	// check gzip switch
	if gzipSwitch, _ := variable.GetString(ctx, types.VarProxyGzipSwitch); gzipSwitch == "off" {
		return false
	}

	ae, _ := headers.Get(strAcceptEncoding)
	if v, ok := gzipCheck[ae]; ok {
		return v
	}

	return false

}

// check response content type
func (f *streamGzipFilter) isCompressibleContentType(headers api.HeaderMap) bool {
	contentType, _ := headers.Get(strContentType)
	if _, ok := f.config.contentType[strings.ToLower(contentType)]; ok {
		return true
	}

	return false
}
