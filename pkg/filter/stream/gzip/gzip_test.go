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
	"compress/gzip"
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
	"mosn.io/api"
	"mosn.io/mosn/pkg/variable"
	"mosn.io/pkg/buffer"

	"mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/protocol"
	_ "mosn.io/mosn/pkg/proxy"
	"mosn.io/mosn/pkg/types"
)

type mockSendHandler struct {
	api.StreamSenderFilterHandler
	downstreamRespDataBuf types.IoBuffer
}

func (f *mockSendHandler) SetResponseData(data types.IoBuffer) {
	// data is the original data. do nothing
	if f.downstreamRespDataBuf == data {
		return
	}
	if f.downstreamRespDataBuf == nil {
		f.downstreamRespDataBuf = buffer.NewIoBuffer(0)
	}
	f.downstreamRespDataBuf.Reset()
	f.downstreamRespDataBuf.ReadFrom(data)
}

func (f *mockSendHandler) setData(data types.IoBuffer) {
	f.downstreamRespDataBuf = data
}

func TestCreateGzipFilterFactory(t *testing.T) {
	m := map[string]interface{}{}
	if f, err := CreateGzipFilterFactory(m); f == nil || err != nil {
		t.Errorf("CreateGzipFilterFactory failed: %v.", err)
	}

	// invalid config
	m["gzip_level"] = 10
	if f, err := CreateGzipFilterFactory(m); f != nil || err == nil {
		t.Error("CreateGzipFilterFactory failed.")
	}
}

func TestGzipNewStreamFilter(t *testing.T) {
	rawbody := "123456"
	cfg := &v2.StreamGzip{
		ContentLength: uint32(len(rawbody)),
		ContentType:   []string{defaultContentType},
	}

	f := NewStreamFilter(context.Background(), cfg)

	sendHandler := &mockSendHandler{}

	f.SetSenderFilterHandler(sendHandler)
	reqHeaders := protocol.CommonHeader(map[string]string{
		strAcceptEncoding: "gzip",
	})

	resGzipHeaders := protocol.CommonHeader(map[string]string{
		strContentType:     defaultContentType,
		strContentEncoding: "gzip",
	})

	resHeaders := protocol.CommonHeader(map[string]string{
		strContentType: defaultContentType,
	})

	f.OnReceive(context.Background(), reqHeaders, nil, nil)

	if !f.needGzip {
		t.Error("client request need gzip.")
	}

	// check switch
	ctx := variable.NewVariableContext(context.Background())
	variable.SetString(ctx, types.VarProxyGzipSwitch, "off")
	f.OnReceive(ctx, reqHeaders, nil, nil)

	if f.needGzip {
		t.Error("gzip switch off.")
	}

	variable.SetString(ctx, types.VarProxyGzipSwitch, "on")
	f.OnReceive(ctx, reqHeaders, nil, nil)
	if !f.needGzip {
		t.Error("gzip switch on.")
	}

	// check responseHeader
	body := buffer.NewIoBufferString(rawbody)
	sendHandler.setData(body)
	f.Append(context.Background(), resGzipHeaders, body, nil)

	if body.String() != rawbody {
		t.Error("response body alerady gziped, shouldn't gzip.")
	}

	// check content type
	resHeaders.Del(strContentEncoding)
	body = buffer.NewIoBufferString(rawbody)
	sendHandler.setData(body)
	f.Append(context.Background(), resHeaders, body, nil)

	v, _ := resHeaders.Get(strContentEncoding)
	if body.String() == rawbody || v != "gzip" {
		t.Error("should gzip body.")
	}

	// check ungzip
	gr, _ := gzip.NewReader(body)
	buf := make([]byte, len(rawbody))
	gr.Read(buf)
	if string(buf) != rawbody {
		t.Errorf("ungzip body error.")
	}

	// check gzip minsize
	resHeaders.Del(strContentEncoding)
	f.config.minCompressLen = uint32(len(rawbody) + 1)
	body = buffer.NewIoBufferString(rawbody)
	sendHandler.setData(body)
	f.Append(context.Background(), resHeaders, body, nil)

	v, _ = resHeaders.Get(strContentEncoding)
	if body.String() != rawbody || v == "gzip" {
		t.Error("should not gzip body.")
	}
}

func BenchmarkGzip(b *testing.B) {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < b.N; i++ {
		buf := randomString(rand.Intn(1024)+4096, rand)
		outBuf := buffer.GetIoBuffer(len(buf) / 3)
		_, err := fasthttp.WriteGzipLevel(outBuf, []byte(buf), defaultGzipLevel)
		if err != nil {
			b.Error("get variable failed:", err)
		}
	}
}

func randomString(n int, rand *rand.Rand) string {
	b := randomBytes(n, rand)
	return string(b)
}

func randomBytes(n int, rand *rand.Rand) []byte {
	r := make([]byte, n)
	if _, err := rand.Read(r); err != nil {
		panic("rand.Read failed: " + err.Error())
	}
	return r
}
