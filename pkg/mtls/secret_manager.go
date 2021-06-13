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
	"reflect"
	"sync"
	"sync/atomic"

	"mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/types"
)

var (
	secretManagerInstance = &secretManager{
		validations: make(map[string]*validation),
	}
	sdsCallbacks = []func(*v2.TLSConfig){}
)

func RegisterSdsCallback(f func(*v2.TLSConfig)) {
	sdsCallbacks = append(sdsCallbacks, f)
}

type validation struct {
	// pem stored the validation pem string
	pem string
	// certificates stored the certificates that are signed by the validation
	certificates map[string]*sdsProvider
}

type secretManager struct {
	mutex       sync.Mutex
	validations map[string]*validation
}

func getOrCreateProvider(cfg *v2.TLSConfig) *sdsProvider {
	return secretManagerInstance.getOrCreateProvider(cfg)
}

func ClearSecretManager() {
	secretManagerInstance.mutex.Lock()
	defer secretManagerInstance.mutex.Unlock()
	secretManagerInstance.validations = make(map[string]*validation)
	log.DefaultLogger.Infof("[mtls] [sds provider] clear all providers")
}

func (mng *secretManager) getOrCreateProvider(cfg *v2.TLSConfig) *sdsProvider {
	mng.mutex.Lock()
	defer mng.mutex.Unlock()
	validationName := cfg.SdsConfig.ValidationConfig.Config.Name
	v, ok := mng.validations[validationName]
	if !ok {
		// add a validation
		v = &validation{
			certificates: make(map[string]*sdsProvider),
		}
		mng.validations[validationName] = v
	}
	certName := cfg.SdsConfig.CertificateConfig.Config.Name
	p, ok := v.certificates[certName]
	if !ok {
		// new a provider
		p = &sdsProvider{
			info: &secretInfo{
				Validation: v.pem,
			},
		}
		p.config.Store(cfg)
		v.certificates[certName] = p
		// set a certificate callback
		client := GetSdsClientV3(cfg.SdsConfig.CertificateConfig.Config)
		client.AddUpdateCallback(cfg.SdsConfig.CertificateConfig.Config, p.setCertificate)
		// set a validation callback
		client.AddUpdateCallback(cfg.SdsConfig.ValidationConfig.Config, mng.setValidation)
		log.DefaultLogger.Infof("[mtls] [sds provider] add a new sds provider %s", certName)
	} else {
		// try to update if config is changed
		v := p.config.Load()
		old, ok := v.(*v2.TLSConfig)
		if ok {
			if reflect.DeepEqual(old, cfg) { // nothing changed
				return p
			}
		}
		log.DefaultLogger.Infof("[mtls] [sds provider] update sds provider %s", certName)
		// try to update provider
		// update tls config
		p.config.Store(cfg)
		// update sds config
		GetSdsClientV3(cfg.SdsConfig.CertificateConfig.Config)
		// update secret
		if p.info.full() {
			p.update()
		}
	}

	return p
}

// setValidation is called in sds client
func (mng *secretManager) setValidation(name string, secret *types.SdsSecret) {
	mng.mutex.Lock()
	defer mng.mutex.Unlock()
	v, ok := mng.validations[name]
	if !ok {
		return
	}
	if secret.ValidationPEM != "" {
		v.pem = secret.ValidationPEM
		log.DefaultLogger.Infof("[mtls] [sds provider] provider %s receive a validation set", name)
		// set the validation
		for _, cert := range v.certificates {
			cert.setValidation(v.pem)
		}
	}
}

// sdsProvider is an implementation of types.Provider
// sdsProvider stored a tls context that makes by sds
// do not support delete certificate for sds api
type sdsProvider struct {
	value  atomic.Value // stored tlsContext
	config atomic.Value // store *v2.TLSConfig
	info   *secretInfo
}

func (p *sdsProvider) setValidation(v string) {
	p.info.Validation = v
	if p.info.full() {
		p.update()
	}
}

func (p *sdsProvider) setCertificate(name string, secret *types.SdsSecret) {
	if secret.CertificatePEM != "" {
		p.info.Certificate = secret.CertificatePEM
		p.info.PrivateKey = secret.PrivateKeyPEM
		log.DefaultLogger.Infof("[mtls] [sds provider] provider %s receive a cerificate set", name)
	}
	if p.info.full() {
		p.update()
	}
}

func (p *sdsProvider) update() {
	v := p.config.Load()
	cfg, ok := v.(*v2.TLSConfig)
	if !ok {
		return
	}
	ctx, err := newTLSContext(cfg, p.info)
	if err != nil {
		log.DefaultLogger.Alertf("sds.update", "[mtls] [sds] update tls context failed: %v", err)
		return
	}
	p.value.Store(ctx)
	log.DefaultLogger.Infof("[mtls] [sds] update tls context success")
	// notify certificates updates
	for _, cb := range sdsCallbacks {
		cb(cfg)
	}
}

func (p *sdsProvider) GetTLSConfigContext(client bool) *types.TLSConfigContext {
	v := p.value.Load()
	ctx, ok := v.(*tlsContext)
	if !ok {
		return nil
	}
	return ctx.GetTLSConfigContext(client)
}

func (p *sdsProvider) MatchedServerName(sn string) bool {
	v := p.value.Load()
	ctx, ok := v.(*tlsContext)
	if !ok {
		return false
	}
	return ctx.MatchedServerName(sn)
}

func (p *sdsProvider) MatchedALPN(protos []string) bool {
	v := p.value.Load()
	ctx, ok := v.(*tlsContext)
	if !ok {
		return false
	}
	return ctx.MatchedALPN(protos)
}

func (p *sdsProvider) Ready() bool {
	v := p.value.Load()
	_, ok := v.(*tlsContext)
	return ok
}

func (p *sdsProvider) Empty() bool {
	return false
}
