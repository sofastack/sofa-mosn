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

package types

import (
	"crypto/sha256"
	"fmt"
	"net"

	envoy_api_v2_auth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	envoy_api_v2_core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_extensions_transport_sockets_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"mosn.io/mosn/pkg/mtls/crypto/tls"
)

type HashValue struct {
	value string
}

func NewHashValue(v [sha256.Size]byte) *HashValue {
	return &HashValue{
		value: fmt.Sprintf("%x", v),
	}
}

func (v *HashValue) Equal(hash *HashValue) bool {
	return v.String() == hash.String()
}

func (v *HashValue) String() string {
	if v == nil {
		return ""
	}
	return v.value
}

// TLSContextManager manages the listener tls config
type TLSContextManager interface {
	// Conn handles the connection, makes a connection as tls connection
	// or keep it as a non-tls connection
	Conn(net.Conn) (net.Conn, error)
	// Enabled returns true means the context manager can make a connection as tls connection
	Enabled() bool
}

// TLSClientContextManager manages the cluster tls config
type TLSClientContextManager interface {
	TLSContextManager
	// HashValue returns the tls context manager's config hash value
	// If tls enabled is false, the hash value returns nil.
	HashValue() *HashValue
	// Fallback represents the config fallback
	Fallback() bool
}

// TLSConfigContext contains a tls.Config and a HashValue represents the tls.Config
type TLSConfigContext struct {
	config *tls.Config
	hash   *HashValue
}

func NewTLSConfigContext(cfg *tls.Config, f func(cfg *tls.Config) *HashValue) *TLSConfigContext {
	return &TLSConfigContext{
		config: cfg,
		hash:   f(cfg),
	}
}

// Config returns a tls.Config's copy in config context
func (ctx *TLSConfigContext) Config() *tls.Config {
	if ctx == nil {
		return nil
	}
	return ctx.config.Clone()
}

// HashValue returns a hash value's copy in config context
func (ctx *TLSConfigContext) HashValue() *HashValue {
	if ctx == nil {
		return nil
	}
	return ctx.hash
}

// TLSProvider provides a tls config for connection
// the matched function is used for check whether the connection should use this provider
type TLSProvider interface {
	// GetTLSConfigContext returns the configcontext used in connection
	// if client is true, return the client mode config, or returns the server mode config
	GetTLSConfigContext(client bool) *TLSConfigContext
	// MatchedServerName checks whether the server name is matched the stored tls certificate
	MatchedServerName(sn string) bool
	// MatchedALPN checks whether the ALPN is matched the stored tls certificate
	MatchedALPN(protos []string) bool
	// Ready checks whether the provider is inited.
	// the static provider should be always ready.
	Ready() bool
	// Empty represent whether the provider contains a certificate or not.
	// A Ready Provider maybe empty too.
	// the sds provider should be always not empty.
	Empty() bool
}

type SdsSecret struct {
	Name           string
	CertificatePEM string
	PrivateKeyPEM  string
	ValidationPEM  string
}

type SdsUpdateCallbackFunc func(name string, secret *SdsSecret)

type SdsClientV3 interface {
	AddUpdateCallback(sdsConfig *envoy_extensions_transport_sockets_tls_v3.SdsSecretConfig, callback SdsUpdateCallbackFunc) error
	DeleteUpdateCallback(sdsConfig *envoy_extensions_transport_sockets_tls_v3.SdsSecretConfig) error
	SecretProviderV3
}

type SecretProviderV3 interface {
	SetSecret(name string, secret *envoy_extensions_transport_sockets_tls_v3.Secret)
}

func SecretConvertV3(raw *envoy_extensions_transport_sockets_tls_v3.Secret) *SdsSecret {
	secret := &SdsSecret{
		Name: raw.Name,
	}
	if validateSecret, ok := raw.Type.(*envoy_extensions_transport_sockets_tls_v3.Secret_ValidationContext); ok {
		ds := validateSecret.ValidationContext.TrustedCa.Specifier.(*envoy_config_core_v3.DataSource_InlineBytes)
		secret.ValidationPEM = string(ds.InlineBytes)
	}
	if tlsCert, ok := raw.Type.(*envoy_extensions_transport_sockets_tls_v3.Secret_TlsCertificate); ok {
		certSpec, _ := tlsCert.TlsCertificate.CertificateChain.Specifier.(*envoy_config_core_v3.DataSource_InlineBytes)
		priKey, _ := tlsCert.TlsCertificate.PrivateKey.Specifier.(*envoy_config_core_v3.DataSource_InlineBytes)
		secret.CertificatePEM = string(certSpec.InlineBytes)
		secret.PrivateKeyPEM = string(priKey.InlineBytes)
	}
	return secret
}

//////
///// Deprecated with xDS v2
/////

type SdsClientV2 interface {
	AddUpdateCallback(sdsConfig *envoy_api_v2_auth.SdsSecretConfig, callback SdsUpdateCallbackFunc) error
	DeleteUpdateCallback(sdsConfig *envoy_api_v2_auth.SdsSecretConfig) error
	SecretProviderV2
}

type SecretProviderV2 interface {
	SetSecret(name string, secret *envoy_api_v2_auth.Secret)
}

func SecretConvertV2(raw *envoy_api_v2_auth.Secret) *SdsSecret {
	secret := &SdsSecret{
		Name: raw.Name,
	}
	if validateSecret, ok := raw.Type.(*envoy_api_v2_auth.Secret_ValidationContext); ok {
		ds := validateSecret.ValidationContext.TrustedCa.Specifier.(*envoy_api_v2_core.DataSource_InlineBytes)
		secret.ValidationPEM = string(ds.InlineBytes)
	}
	if tlsCert, ok := raw.Type.(*envoy_api_v2_auth.Secret_TlsCertificate); ok {
		certSpec, _ := tlsCert.TlsCertificate.CertificateChain.Specifier.(*envoy_api_v2_core.DataSource_InlineBytes)
		priKey, _ := tlsCert.TlsCertificate.PrivateKey.Specifier.(*envoy_api_v2_core.DataSource_InlineBytes)
		secret.CertificatePEM = string(certSpec.InlineBytes)
		secret.PrivateKeyPEM = string(priKey.InlineBytes)
	}
	return secret
}
