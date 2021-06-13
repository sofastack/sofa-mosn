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

package common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"testing"

	"mosn.io/api"
)

func genRoleBasedAccessControlEngine(confPath string) (*RoleBasedAccessControlEngine, *RoleBasedAccessControlEngine, error) {
	// config
	conf, err := ioutil.ReadFile(confPath)
	if err != nil {
		return nil, nil, err
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(conf, &cfg); err != nil {
		return nil, nil, err
	}

	rbacConfig, err := ParseRbacFilterConfig(cfg)
	if err != nil {
		return nil, nil, err
	}

	// engine
	engine, err := NewRoleBasedAccessControlEngine(rbacConfig.GetRules())
	if err != nil {
		return nil, nil, err
	}

	// shadow engine
	shadowEngine, err := NewRoleBasedAccessControlEngine(rbacConfig.GetShadowRules())
	if err != nil {
		return nil, nil, err
	}

	return engine, shadowEngine, nil
}

// Mock StreamReceiverFilterHandler
type mockStreamReceiverFilterHandler struct {
	api.StreamReceiverFilterHandler
	conn *mockConn
}

func (cb *mockStreamReceiverFilterHandler) Connection() api.Connection {
	return cb.conn
}

type mockConn struct {
	api.Connection
	remoteAddr *mockAddr
	localAddr  *mockAddr
}

func (conn *mockConn) RemoteAddr() net.Addr {
	return conn.remoteAddr
}

func (conn *mockConn) LocalAddr() net.Addr {
	return conn.localAddr
}

type mockAddr struct {
	IP   string
	Port int
}

func (addr *mockAddr) Network() string {
	return "tcp"
}

func (addr *mockAddr) String() string {
	return fmt.Sprintf("%s:%d", addr.IP, addr.Port)
}

type mockHeaderMap struct {
	api.HeaderMap
	headers map[string]string
}

func (headers *mockHeaderMap) Get(key string) (string, bool) {
	if value, ok := headers.headers[key]; ok {
		return value, true
	} else {
		return "", false
	}
}

func TestGetPoliciesSize(t *testing.T) {
	engine, shadowEngine, _ := genRoleBasedAccessControlEngine("./test_conf/deny-all.json")
	if engine.GetPoliciesSize() != 1 {
		t.Error("TestGetPoliciesSize failed")
	}
	if shadowEngine.GetPoliciesSize() != 1 {
		t.Error("TestGetPoliciesSize failed")
	}
}

func TestShadowEngine(t *testing.T) {
	_, shadowEngine, _ := genRoleBasedAccessControlEngine("./test_conf/deny-all.json")
	allowed, _ := shadowEngine.Allowed(nil, nil, nil)
	if allowed {
		t.Error("TestShadowEngine failed")
		return
	}
}
