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
	"fmt"
	"io/ioutil"
	"sync"
	"testing"
	"time"

	envoy_extensions_transport_sockets_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	v2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/types"
)

var testInit sync.Once

func createSdsTLSConfigV3() *v2.TLSConfig {
	testInit.Do(func() {
		getSdsClientFuncV3 = getMockSdsClientV3
	})
	return &v2.TLSConfig{
		Status:       true,
		VerifyClient: true,
		SdsConfig: &v2.SdsConfig{
			CertificateConfig: &v2.SecretConfigWrapper{
				Config: &envoy_extensions_transport_sockets_tls_v3.SdsSecretConfig{
					Name: "default",
				},
			},
			ValidationConfig: &v2.SecretConfigWrapper{
				Config: &envoy_extensions_transport_sockets_tls_v3.SdsSecretConfig{
					Name: "rootCA",
				},
			},
		},
	}
}

func resetTest() {
	secretManagerInstance = &secretManager{
		validations: make(map[string]*validation),
	}
	sdsCallbacks = []func(*v2.TLSConfig){}
	mockSdsClientInstance = &mockSdsClientV3{
		callback: make(map[string]types.SdsUpdateCallbackFunc),
	}
}

func mockSetSecret() *secretInfo {
	info := &certInfo{
		CommonName: "sds",
		Curve:      "RSA",
	}
	secret, _ := info.CreateSecret()
	//
	sRoot := &types.SdsSecret{
		Name:          "rootCA",
		ValidationPEM: secret.Validation,
	}
	sDefault := &types.SdsSecret{
		Name:           "default",
		CertificatePEM: secret.Certificate,
		PrivateKeyPEM:  secret.PrivateKey,
	}

	//
	mockSdsClientInstance.setSecret("rootCA", sRoot)
	mockSdsClientInstance.setSecret("default", sDefault)
	return secret
}

// Test a simple sds scenario
// server listen a sds tls config
// before the certificate is setted, cannot support tls request
// after the certificate is setted, support tls request
func TestSimpleSdsTLS(t *testing.T) {
	resetTest()
	cfg := createSdsTLSConfigV3()
	filterChains := []v2.FilterChain{
		{
			TLSContexts: []v2.TLSConfig{
				*cfg,
			},
		},
	}
	lc := &v2.Listener{}
	lc.Inspector = true
	lc.FilterChains = filterChains
	ctxMng, err := NewTLSServerContextManager(lc)
	if err != nil {
		t.Errorf("create context manager failed %v", err)
		return
	}
	server := MockServer{
		Mng: ctxMng,
		t:   t,
	}
	server.GoListenAndServe(t)
	defer server.Close()
	time.Sleep(time.Second) //wait server start
	// before certificet set, server support non-tls
	inspectorCall := func() {
		resp, err := MockClient(t, server.Addr, nil)
		if err != nil {
			t.Errorf("inspector call sds listener failed: %v", err)
			return
		}
		defer resp.Body.Close()
		ioutil.ReadAll(resp.Body)
	}
	// no certificate in server, tls call should failed
	unsupportCall := func() {
		cfg := &v2.TLSConfig{
			Status:       true,
			InsecureSkip: true,
		}
		cltMng, err := NewTLSClientContextManager(cfg)
		if err != nil {
			t.Fatalf("create tls context manager failed: %v", err)
		}
		resp, err := MockClient(t, server.Addr, cltMng)
		if err == nil {
			t.Error("expected request server failed, but success")
			ioutil.ReadAll(resp.Body)
			resp.Body.Close()
		}
	}
	inspectorCall()
	unsupportCall()
	secret := mockSetSecret()
	// after certificate set
	clientConfigs := []*v2.TLSConfig{
		{
			Status:     true,
			CACert:     secret.Validation,
			ServerName: "127.0.0.1",
		},
		{
			Status:       true,
			InsecureSkip: true,
		},
	}
	for i, cfg := range clientConfigs {
		cltMng, err := NewTLSClientContextManager(cfg)
		if err != nil {
			t.Errorf("#%d create client context manager failed %v", i, err)
			continue
		}

		resp, err := MockClient(t, server.Addr, cltMng)
		if err != nil {
			t.Errorf("#%d request server error %v", i, err)
			continue
		}
		ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	}

}

// Test sds tls config with extension, after certificate is setted
// If the client request non-tls. it is ok
// If the client request tls without certificate. it is ok
// If the client request tls with certificate, the server will verify the client's certificate
func TestSdsWithExtension(t *testing.T) {
	resetTest()
	cfg := createSdsTLSConfigV3()
	// Add extension
	cfg.Type = testType
	extendVerify := map[string]interface{}{
		"name": "server",
		"cn":   "client",
	}
	cfg.ExtendVerify = extendVerify

	filterChains := []v2.FilterChain{
		{
			TLSContexts: []v2.TLSConfig{
				*cfg,
			},
		},
	}
	lc := &v2.Listener{}
	lc.Inspector = true
	lc.FilterChains = filterChains
	ctxMng, err := NewTLSServerContextManager(lc)
	if err != nil {
		t.Errorf("create context manager failed %v", err)
		return
	}
	server := MockServer{
		Mng: ctxMng,
		t:   t,
	}
	server.GoListenAndServe(t)
	defer server.Close()
	time.Sleep(time.Second) //wait server start
	mockSetSecret()
	// 1. client request non-tls
	inspectorCall := func() {
		resp, err := MockClient(t, server.Addr, nil)
		if err != nil {
			t.Errorf("inspector call sds listener failed: %v", err)
			return
		}
		defer resp.Body.Close()
		ioutil.ReadAll(resp.Body)
	}
	inspectorCall()
	// 2. client request tls without certificate
	noCertCall := func() {
		cfg := &v2.TLSConfig{
			Status:       true,
			InsecureSkip: true,
		}
		cltMng, _ := NewTLSClientContextManager(cfg)
		resp, err := MockClient(t, server.Addr, cltMng)
		if err != nil {
			t.Errorf("client request server without certificate failed: %v", err)
			return
		}
		defer resp.Body.Close()
		ioutil.ReadAll(resp.Body)

	}
	noCertCall()
	// 3. requets with valid cert, verify pass
	certPass := func() {
		info := &certInfo{
			CommonName: extendVerify["cn"].(string),
			Curve:      "RSA",
		}
		cfg, _ := info.CreateCertConfig()
		cfg.ServerName = "127.0.0.1"
		cltMng, err := NewTLSClientContextManager(cfg)
		if err != nil {
			t.Errorf("create tls context manager failed: %v", err)
			return
		}
		resp, err := MockClient(t, server.Addr, cltMng)
		if err != nil {
			t.Errorf("client request server with valid certificate failed: %v", err)
			return
		}
		defer resp.Body.Close()
		ioutil.ReadAll(resp.Body)
	}
	certPass()
	// 4. request with invalid cert,  verify failed
	certFail := func() {
		info := &certInfo{
			CommonName: "invalid",
			Curve:      "RSA",
		}
		cfg, _ := info.CreateCertConfig()
		cfg.ServerName = "127.0.0.1"
		cltMng, err := NewTLSClientContextManager(cfg)
		if err != nil {
			t.Errorf("create tls context manager failed: %v", err)
			return
		}
		resp, err := MockClient(t, server.Addr, cltMng)
		if err == nil {
			t.Errorf("client request server with invalid certificate passed")
			defer resp.Body.Close()
			ioutil.ReadAll(resp.Body)
		}
	}
	certFail()
}

func TestSdsProviderUpdate(t *testing.T) {
	resetTest()
	cfg := createSdsTLSConfigV3()
	prd := getOrCreateProvider(cfg)
	if prd.Ready() {
		t.Fatal("provider ready without certificate")
	}
	mockSetSecret()
	if !prd.Ready() {
		t.Fatal("provider note ready with certificate")
	}
	h1 := prd.GetTLSConfigContext(true).HashValue()
	// try to get again
	prd2 := getOrCreateProvider(cfg)
	prdAddr := fmt.Sprintf("%p", prd)
	prd2Addr := fmt.Sprintf("%p", prd2)
	if prdAddr != prd2Addr {
		t.Fatal("sds provider reuse failed")
	}
	// update tls config
	cfg2 := createSdsTLSConfigV3()
	cfg2.CipherSuites = "RSA-AES256-CBC-SHA:RSA-3DES-EDE-CBC-SHA"
	prd3 := getOrCreateProvider(cfg2)
	prd3Addr := fmt.Sprintf("%p", prd3)
	if prdAddr != prd3Addr {
		t.Fatal("sds provider reuse failed")
	}
	h2 := prd.GetTLSConfigContext(true).HashValue()
	if h1 == h2 {
		t.Fatal("update tls config failed")
	}
}
