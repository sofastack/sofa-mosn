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
	"bytes"
	"fmt"

	envoy_api_v2_auth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	envoy_extensions_transport_sockets_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/golang/protobuf/jsonpb"
)

// TLSConfig is a configuration of tls context
type TLSConfig struct {
	Status            bool                   `json:"status,omitempty"`
	Type              string                 `json:"type,omitempty"`
	ServerName        string                 `json:"server_name,omitempty"`
	CACert            string                 `json:"ca_cert,omitempty"`
	CertChain         string                 `json:"cert_chain,omitempty"`
	PrivateKey        string                 `json:"private_key,omitempty"`
	VerifyClient      bool                   `json:"verify_client,omitempty"`
	RequireClientCert bool                   `json:"require_client_cert,omitempty"`
	InsecureSkip      bool                   `json:"insecure_skip,omitempty"`
	CipherSuites      string                 `json:"cipher_suites,omitempty"`
	EcdhCurves        string                 `json:"ecdh_curves,omitempty"`
	MinVersion        string                 `json:"min_version,omitempty"`
	MaxVersion        string                 `json:"max_version,omitempty"`
	ALPN              string                 `json:"alpn,omitempty"`
	Ticket            string                 `json:"ticket,omitempty"`
	Fallback          bool                   `json:"fall_back,omitempty"`
	ExtendVerify      map[string]interface{} `json:"extend_verify,omitempty"`
	SdsConfig         *SdsConfig             `json:"sds_source,omitempty"`
}

type SdsConfig struct {
	CertificateConfig *SecretConfigWrapper
	ValidationConfig  *SecretConfigWrapper
}

type SecretConfigWrapper struct {
	Config           *envoy_extensions_transport_sockets_tls_v3.SdsSecretConfig
	ConfigDeprecated *envoy_api_v2_auth.SdsSecretConfig
}

func (sc SecretConfigWrapper) MarshalJSON() (b []byte, err error) {
	newData := &bytes.Buffer{}
	marshaler := &jsonpb.Marshaler{}
	err = marshaler.Marshal(newData, sc.Config)
	return newData.Bytes(), err
}

func (sc *SecretConfigWrapper) UnmarshalJSON(b []byte) error {
	secretConfigV3 := &envoy_extensions_transport_sockets_tls_v3.SdsSecretConfig{}
	err1 := jsonpb.Unmarshal(bytes.NewReader(b), secretConfigV3)
	if err1 == nil {
		sc.Config = secretConfigV3
		// first Unmarshal with v3, if no err will return fast.
		return nil
	}

	secretConfigV2 := &envoy_api_v2_auth.SdsSecretConfig{}
	err2 := jsonpb.Unmarshal(bytes.NewBuffer(b), secretConfigV2)
	if err2 == nil {
		sc.ConfigDeprecated = secretConfigV2
		// try UnmarshalJSON with v2, if no err will return fast.
		return nil
	}

	// neither of v2 and v3
	return fmt.Errorf("Unmarshal SdsSecretConfig with V3 failed: {%s}, with V2 failed: {%s}", err1, err2)
}

// Valid checks the whether the SDS Config is valid or not
func (c *SdsConfig) Valid() bool {
	return c != nil && c.CertificateConfig != nil && c.ValidationConfig != nil
}
