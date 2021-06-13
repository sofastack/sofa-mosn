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

package mtls

import (
	"net"
	"reflect"
	"time"

	"mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/mtls/crypto/tls"
	"mosn.io/mosn/pkg/types"
)

type serverContextManager struct {
	// providers stored the certificates
	providers []types.TLSProvider
	// if inspector is true, the manager support non-tls connections
	inspector bool
	// config is a tls.config with GetConfigForClient
	config *tls.Config
}

// NewTLSServerContextManager returns a types.TLSContextManager used in TLS Server
// A Server Manager can contains multiple certificates in provider
func NewTLSServerContextManager(cfg *v2.Listener) (types.TLSContextManager, error) {
	mng := &serverContextManager{
		inspector: cfg.Inspector,
	}
	mng.config = &tls.Config{
		GetConfigForClient: mng.GetConfigForClient,
	}
	for _, c := range cfg.FilterChains {
		for _, tlsCfg := range c.TLSContexts {
			provider, err := NewProvider(&tlsCfg)
			if err != nil {
				return nil, err
			}
			// provider is an interface, needs to check by reflect
			if provider != nil && !reflect.ValueOf(provider).IsNil() {
				// if a server receive a empty provider and do not support fallback, it should be failed
				if provider.Empty() {
					if !tlsCfg.Fallback {
						return nil, ErrorNoCertConfigure
					}
					log.DefaultLogger.Alertf(types.ErrorKeyTLSFallback, "listener enable tls without certificate, fallback tls")
				} else {
					mng.providers = append(mng.providers, provider)
				}
			}
		}
	}
	return mng, nil
}

func (mng *serverContextManager) GetConfigForClient(info *tls.ClientHelloInfo) (*tls.Config, error) {
	var defaultProvider types.TLSProvider
	for _, provider := range mng.providers {
		if !provider.Ready() {
			continue
		}
		// the default provider is the first provider which is ready
		if defaultProvider == nil {
			defaultProvider = provider
		}
		if provider.MatchedServerName(info.ServerName) {
			return provider.GetTLSConfigContext(false).Config(), nil
		}
		if provider.MatchedALPN(info.SupportedProtos) {
			return provider.GetTLSConfigContext(false).Config(), nil
		}
	}
	if defaultProvider == nil {
		return nil, ErrorNoCertConfigure
	}
	return defaultProvider.GetTLSConfigContext(false).Config(), nil
}

func (mng *serverContextManager) Conn(c net.Conn) (net.Conn, error) {
	if _, ok := c.(*net.TCPConn); !ok {
		return c, nil
	}
	if !mng.Enabled() {
		return c, nil
	}
	if !mng.inspector {
		if log.DefaultLogger.GetLogLevel() >= log.DEBUG {
			log.DefaultLogger.Debugf("[mtls] connected as server, remote addr: %s", c.RemoteAddr())
		}
		return &TLSConn{
			tls.Server(c, mng.config.Clone()),
		}, nil
	}
	// inspector
	conn := &Conn{
		Conn: c,
	}
	buf, err := conn.Peek()
	if err != nil {
		return nil, err
	}
	switch buf[0] {
	// TLS handshake
	case 0x16:
		if log.DefaultLogger.GetLogLevel() >= log.DEBUG {
			log.DefaultLogger.Debugf("[mtls] connected as server, remote addr: %s", c.RemoteAddr())
		}
		return &TLSConn{
			tls.Server(conn, mng.config.Clone()),
		}, nil
	// Non TLS
	default:
		return conn, nil
	}
}

func (mng *serverContextManager) Enabled() bool {
	for _, p := range mng.providers {
		if !p.Empty() {
			return true
		}
	}
	return false
}

type clientContextManager struct {
	// client support only one certificate
	provider types.TLSProvider
	// fallback
	fallback bool
	// servername for tls verify
	serverName string
}

// NewTLSClientContextManager returns a types.TLSContextManager used in TLS Client
func NewTLSClientContextManager(cfg *v2.TLSConfig) (types.TLSClientContextManager, error) {
	provider, err := NewProvider(cfg)
	if err != nil {
		return nil, err
	}
	mng := &clientContextManager{
		provider:   provider,
		fallback:   cfg.Fallback,
		serverName: cfg.ServerName,
	}
	return mng, nil
}

var handshakeTimeout = types.DefaultConnReadTimeout

func (mng *clientContextManager) Conn(c net.Conn) (net.Conn, error) {
	if _, ok := c.(*net.TCPConn); !ok {
		return c, nil
	}
	if !mng.Enabled() {
		return c, nil
	}
	// make tls connection and try handshake
	tlsconn := tls.Client(c, mng.provider.GetTLSConfigContext(true).Config())
	tlsconn.SetServerName(mng.serverName)
	tlsconn.SetReadDeadline(time.Now().Add(handshakeTimeout))
	if err := tlsconn.Handshake(); err != nil {
		c.Close() // close the failed connection
		return nil, err
	}
	if log.DefaultLogger.GetLogLevel() >= log.DEBUG {
		log.DefaultLogger.Debugf("[mtls] connected as client, remote addr: %s", c.RemoteAddr())
	}

	return &TLSConn{
		tlsconn,
	}, nil
}

func (mng *clientContextManager) Enabled() bool {
	return mng != nil && mng.provider != nil && mng.provider.Ready()
}

// if the provider is not ready, the hash value returns nil.
func (mng *clientContextManager) HashValue() *types.HashValue {
	if mng == nil || mng.provider == nil {
		return nil
	}
	return mng.provider.GetTLSConfigContext(true).HashValue()

}

func (mng *clientContextManager) Fallback() bool {
	return mng.fallback
}
