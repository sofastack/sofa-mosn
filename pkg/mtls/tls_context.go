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
	"crypto/x509"
	"fmt"
	"strings"

	"mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/mtls/crypto/tls"
	"mosn.io/mosn/pkg/types"
)

type secretInfo struct {
	Certificate string
	PrivateKey  string
	Validation  string // root ca
}

// full returns whether the secret info is full enough for a tls config
func (info *secretInfo) full() bool {
	return info.Certificate != "" && info.PrivateKey != "" && info.Validation != ""
}

// tlsContext is an implmentation of basic provider
type tlsContext struct {
	matches map[string]struct{}
	client  *types.TLSConfigContext
	server  *types.TLSConfigContext
}

func (ctx *tlsContext) buildMatch(tlsConfig *tls.Config) {
	if tlsConfig == nil {
		return
	}
	matches := make(map[string]struct{})
	certs := tlsConfig.Certificates
	for i := range certs {
		cert := certs[i]
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			continue
		}
		if len(x509Cert.Subject.CommonName) > 0 {
			matches[x509Cert.Subject.CommonName] = struct{}{}
		}
		for _, san := range x509Cert.DNSNames {
			matches[san] = struct{}{}
		}
	}
	for _, protocol := range tlsConfig.NextProtos {
		matches[protocol] = struct{}{}
	}
	matches[tlsConfig.ServerName] = struct{}{}
	ctx.matches = matches
}

func (ctx *tlsContext) setServerConfig(tmpl *tls.Config, cfg *v2.TLSConfig, hooks ConfigHooks) {
	tlsConfig := tmpl.Clone()
	// no certificate should be set no server tls config
	if len(tlsConfig.Certificates) == 0 {
		return
	}
	tlsConfig.ClientAuth = hooks.GetClientAuth(cfg)
	tlsConfig.VerifyPeerCertificate = hooks.ServerHandshakeVerify(tlsConfig)

	ctx.server = types.NewTLSConfigContext(tlsConfig, hooks.GenerateHashValue)
	// build matches
	ctx.buildMatch(tlsConfig)
}

func (ctx *tlsContext) setClientConfig(tmpl *tls.Config, cfg *v2.TLSConfig, hooks ConfigHooks) {
	tlsConfig := tmpl.Clone()
	tlsConfig.ServerName = cfg.ServerName
	tlsConfig.VerifyPeerCertificate = hooks.ClientHandshakeVerify(tlsConfig)
	if tlsConfig.VerifyPeerCertificate != nil {
		// use self verify, skip normal verify
		tlsConfig.InsecureSkipVerify = true
	}
	if cfg.InsecureSkip {
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyPeerCertificate = nil
	}
	ctx.client = types.NewTLSConfigContext(tlsConfig, hooks.GenerateHashValue)
}

func (ctx *tlsContext) MatchedServerName(sn string) bool {
	name := strings.ToLower(sn)
	// e.g. www.example.com will be first matched against www.example.com, then *.example.com, then *.com
	for len(name) > 0 && name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	if _, ok := ctx.matches[name]; ok {
		return true
	}
	labels := strings.Split(name, ".")
	for i := 0; i < len(labels)-1; i++ {
		labels[i] = "*"
		candidate := strings.Join(labels[i:], ".")
		if _, ok := ctx.matches[candidate]; ok {
			return true
		}
	}
	return false
}

func (ctx *tlsContext) MatchedALPN(protos []string) bool {
	for _, protocol := range protos {
		protocol = strings.ToLower(protocol)
		if _, ok := ctx.matches[protocol]; ok {
			return true
		}
	}
	return false
}

func (ctx *tlsContext) GetTLSConfigContext(client bool) *types.TLSConfigContext {
	if client {
		return ctx.client
	} else {
		if ctx.server == nil {
			return nil
		}
		return ctx.server
	}
}

func newTLSContext(cfg *v2.TLSConfig, secret *secretInfo) (*tlsContext, error) {
	// basic template
	tmpl, err := tlsConfigTemplate(cfg)
	if err != nil {
		return nil, err
	}
	// extension config
	factory := getFactory(cfg.Type)
	hooks := factory.CreateConfigHooks(cfg.ExtendVerify)
	// pool can be nil, if it is nil, TLS uses the host's root CA set.
	pool, err := hooks.GetX509Pool(secret.Validation)
	if err != nil {
		return nil, err
	}
	tmpl.RootCAs = pool
	tmpl.ClientCAs = pool
	// set tls context
	ctx := &tlsContext{}
	cert, err := hooks.GetCertificate(secret.Certificate, secret.PrivateKey)
	switch err {
	case ErrorNoCertConfigure:
		// no certificate
	case nil:
		tmpl.Certificates = append(tmpl.Certificates, cert)
	default:
		// get certificate failed, if fallback is configured, it is ok
		if !cfg.Fallback {
			return nil, err
		} else {
			log.DefaultLogger.Alertf(types.ErrorKeyTLSFallback, "get certificate failed: %v, fallback tls", err)
		}
	}

	// needs copy template config
	if len(tmpl.Certificates) > 0 {
		ctx.setServerConfig(tmpl, cfg, hooks)
	}
	ctx.setClientConfig(tmpl, cfg, hooks)
	return ctx, nil

}

func tlsConfigTemplate(c *v2.TLSConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	// prefer server cipher suites by default
	tlsConfig.PreferServerCipherSuites = true

	if c.CipherSuites != "" {
		ciphers := strings.Split(c.CipherSuites, ":")
		for _, s := range ciphers {
			cipher, ok := ciphersMap[s]
			if !ok {
				log.DefaultLogger.Warnf("[mtls] cipher %s is not supported", s)
				continue
			}
			tlsConfig.CipherSuites = append(tlsConfig.CipherSuites, cipher)
		}
		if len(tlsConfig.CipherSuites) == 0 {
			tlsConfig.CipherSuites = defaultCiphers
		}
	}
	if c.EcdhCurves != "" {
		curves := strings.Split(c.EcdhCurves, ",")
		for _, s := range curves {
			curve, ok := allCurves[strings.ToLower(s)]
			if !ok {
				return nil, fmt.Errorf("curve %s is not supported", s)
			}
			tlsConfig.CurvePreferences = append(tlsConfig.CurvePreferences, curve)
		}
		if len(tlsConfig.CurvePreferences) == 0 {
			tlsConfig.CurvePreferences = defaultCurves
		}
	}
	tlsConfig.MaxVersion = maxProtocols
	if c.MaxVersion != "" {
		protocol, ok := version[strings.ToLower(c.MaxVersion)]
		if !ok {
			return nil, fmt.Errorf("tls protocol %s is not supported", c.MaxVersion)
		}
		if protocol != 0 {
			tlsConfig.MaxVersion = protocol
		}
	}
	tlsConfig.MinVersion = minProtocols
	if c.MinVersion != "" {
		protocol, ok := version[strings.ToLower(c.MinVersion)]
		if !ok {
			return nil, fmt.Errorf("tls protocol %s is not supported", c.MaxVersion)
		}
		if protocol != 0 {
			tlsConfig.MinVersion = protocol
		}
	}
	if c.ALPN != "" {
		protocols := strings.Split(c.ALPN, ",")
		for _, p := range protocols {
			_, ok := alpn[strings.ToLower(p)]
			if !ok {
				log.DefaultLogger.Errorf("[mtls] ALPN %s is not supported", p)
				continue
			}
			tlsConfig.NextProtos = append(tlsConfig.NextProtos, p)
		}
	}
	tlsConfig.ServerName = c.ServerName
	return tlsConfig, nil
}
