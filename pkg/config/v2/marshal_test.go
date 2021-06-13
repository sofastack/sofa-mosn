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

package v2

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	envoy_api_v2_auth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	envoy_api_v2_core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_extensions_transport_sockets_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
)

func TestFaultInjectMarshal(t *testing.T) {
	f := &FaultInject{
		DelayDuration: uint64(time.Second),
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"delay_duration":"1s"`) {
		t.Fatalf("unexpected output: %s", string(b))
	}
}

func TestDelayInjectMarshal(t *testing.T) {
	i := &DelayInject{
		Delay: time.Second,
	}
	b, err := json.Marshal(i)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"fixed_delay":"1s"`) {
		t.Fatalf("unexpected output: %s", string(b))
	}
}

func TestSecretConfigWrapperMarshalXDSV2(t *testing.T) {
	sw := &SecretConfigWrapper{
		Config: &envoy_extensions_transport_sockets_tls_v3.SdsSecretConfig{},
		ConfigDeprecated: &envoy_api_v2_auth.SdsSecretConfig{
			Name:      "sds",
			SdsConfig: &envoy_api_v2_core.ConfigSource{},
		},
	}
	b, err := json.Marshal(sw)
	if err != nil {
		t.Fatal(err)
		if !strings.Contains(string(b), "sdsConfig") {
			t.Fatalf("unexpected output: %s", string(b))
		}
	}
}

func TestSecretConfigWrapperMarshalXDSV3(t *testing.T) {
	sw := &SecretConfigWrapper{
		Config: &envoy_extensions_transport_sockets_tls_v3.SdsSecretConfig{
			Name:      "sds",
			SdsConfig: &envoy_config_core_v3.ConfigSource{},
		},
	}
	b, err := json.Marshal(sw)
	if err != nil {
		t.Fatal(err)
		if !strings.Contains(string(b), "sdsConfig") {
			t.Fatalf("unexpected output: %s", string(b))
		}
	}
}
