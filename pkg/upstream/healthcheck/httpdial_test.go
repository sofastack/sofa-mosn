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

package healthcheck

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type handler struct {
	duration time.Duration
}

func (h handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	time.Sleep(h.duration)
	writer.WriteHeader(http.StatusOK)
}

func TestHTTPDial(t *testing.T) {
	s := httptest.NewServer(&handler{})
	host := &mockHost{
		addr: s.URL,
	}
	dialfactory := &HTTPDialSessionFactory{}
	session := dialfactory.NewSession(nil, host)
	if !session.CheckHealth() {
		t.Error("http dial check health failed")
	}
	s.Close()
	if session.CheckHealth() {
		t.Error("http dial a closed server, but returns ok")
	}
}

func TestHTTPDialTimeout(t *testing.T) {
	s := httptest.NewServer(&handler{time.Second * 3})
	host := &mockHost{
		addr: s.URL,
	}
	dialfactory := &HTTPDialSessionFactory{}
	session := dialfactory.NewSession(map[string]interface{}{
		TimeoutCfgKey: uint32(2), // timeout after 2 second
	}, host)
	if session.CheckHealth() {
		t.Error("http dial check health succeed, should failed")
	}
	s.Close()
	if session.CheckHealth() {
		t.Error("http dial a closed server, but returns ok")
	}
}

func TestHTTPDialNonStanderURL(t *testing.T) {
	s := httptest.NewServer(&handler{})
	host := &mockHost{
		addr: s.URL[len("http://"):],
	}
	dialfactory := &HTTPDialSessionFactory{}
	session := dialfactory.NewSession(nil, host)
	if !session.CheckHealth() {
		t.Error("http dial check health failed")
	}
	s.Close()
	if session.CheckHealth() {
		t.Error("http dial a closed server, but returns ok")
	}
}
