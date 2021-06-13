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

package sds

import (
	"errors"
	"sync"

	envoy_extensions_transport_sockets_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/types"
	"mosn.io/pkg/utils"
)

type SdsClientImplV3 struct {
	SdsConfigMap   map[string]*envoy_extensions_transport_sockets_tls_v3.SdsSecretConfig
	SdsCallbackMap map[string]types.SdsUpdateCallbackFunc
	updatedLock    sync.Mutex
	sdsSubscriber  *SdsSubscriberV3
}

var (
	sdsClientV3     *SdsClientImplV3
	sdsClientLock   sync.Mutex
	sdsPostCallback func() = nil
)

var ErrSdsClientNotInit = errors.New("sds client not init")

// NewSdsClientSingletonV3 use by tls module , when get sds config from xds
func NewSdsClientSingletonV3(config *envoy_extensions_transport_sockets_tls_v3.SdsSecretConfig) types.SdsClientV3 {
	sdsClientLock.Lock()
	defer sdsClientLock.Unlock()
	if sdsClientV3 != nil {
		// update sds config
		sdsClientV3.sdsSubscriber.sdsConfig = config.SdsConfig
		return sdsClientV3
	}

	sdsClientV3 = &SdsClientImplV3{
		SdsConfigMap:   make(map[string]*envoy_extensions_transport_sockets_tls_v3.SdsSecretConfig),
		SdsCallbackMap: make(map[string]types.SdsUpdateCallbackFunc),
	}
	// For Istio , sds config should be the same
	// So we use first sds config to init sds subscriber
	sdsClientV3.sdsSubscriber = NewSdsSubscriberV3(sdsClientV3, config.SdsConfig, types.GetGlobalXdsInfo().ServiceNode, types.GetGlobalXdsInfo().ServiceCluster)
	utils.GoWithRecover(sdsClientV3.sdsSubscriber.Start, nil)
	return sdsClientV3
}

// CloseSdsClientV3 used only mosn exit
func CloseSdsClientV3() {
	sdsClientLock.Lock()
	defer sdsClientLock.Unlock()
	if sdsClientV3 != nil && sdsClientV3.sdsSubscriber != nil {
		sdsClientV3.sdsSubscriber.Stop()
		sdsClientV3.sdsSubscriber = nil
		sdsClientV3 = nil
	}
}

func (client *SdsClientImplV3) AddUpdateCallback(sdsConfig *envoy_extensions_transport_sockets_tls_v3.SdsSecretConfig, callback types.SdsUpdateCallbackFunc) error {
	client.updatedLock.Lock()
	defer client.updatedLock.Unlock()
	client.SdsConfigMap[sdsConfig.Name] = sdsConfig
	client.SdsCallbackMap[sdsConfig.Name] = callback
	client.sdsSubscriber.SendSdsRequest(sdsConfig.Name)
	return nil
}

// DeleteUpdateCallback ...
func (client *SdsClientImplV3) DeleteUpdateCallback(sdsConfig *envoy_extensions_transport_sockets_tls_v3.SdsSecretConfig) error {
	client.updatedLock.Lock()
	defer client.updatedLock.Unlock()
	delete(client.SdsConfigMap, sdsConfig.Name)
	delete(client.SdsCallbackMap, sdsConfig.Name)
	return nil
}

// SetSecret invoked when sds subscriber get secret response
func (client *SdsClientImplV3) SetSecret(name string, secret *envoy_extensions_transport_sockets_tls_v3.Secret) {
	client.updatedLock.Lock()
	defer client.updatedLock.Unlock()
	if fc, ok := client.SdsCallbackMap[name]; ok {
		log.DefaultLogger.Debugf("[xds] [sds client],set secret = %v", name)
		mosnSecret := types.SecretConvertV3(secret)
		fc(name, mosnSecret)
	}
}

// SetSdsPostCallback ..
func SetSdsPostCallback(fc func()) {
	sdsPostCallback = fc
}
