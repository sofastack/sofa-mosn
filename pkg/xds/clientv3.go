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

/* Package xds can be used to create an grpc (just support grpc and v2 api) client communicated with pilot
   and fetch config in cycle
*/

package xds

import (
	"errors"
	"sync"

	envoy_config_bootstrap_v3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	envoy_config_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes/duration"
	jsoniter "github.com/json-iterator/go"
	mv2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/log"
	v3 "mosn.io/mosn/pkg/xds/v3"
	"mosn.io/mosn/pkg/xds/v3/conv"
)

// clientv3 provide an ADS client with xDS v3
type clientv3 struct {
	adsClient *v3.ADSClient
}

// unmarshalResourcesV3 used in order to convert bootstrap_v2 json to pb struct (go-control-plane), some fields must be exchanged format
func unmarshalResourcesV3(config *mv2.MOSNConfig) (dynamicResources *envoy_config_bootstrap_v3.Bootstrap_DynamicResources, staticResources *envoy_config_bootstrap_v3.Bootstrap_StaticResources, err error) {

	if len(config.RawDynamicResources) > 0 {
		dynamicResources = &envoy_config_bootstrap_v3.Bootstrap_DynamicResources{}
		resources := map[string]jsoniter.RawMessage{}
		err = json.Unmarshal(config.RawDynamicResources, &resources)
		if err != nil {
			log.DefaultLogger.Errorf("fail to unmarshal dynamic_resources: %v", err)
			return nil, nil, err
		}
		if adsConfigRaw, ok := resources["ads_config"]; ok {
			var b []byte
			adsConfig := map[string]jsoniter.RawMessage{}
			err = json.Unmarshal([]byte(adsConfigRaw), &adsConfig)
			if err != nil {
				log.DefaultLogger.Errorf("fail to unmarshal ads_config: %v", err)
				return nil, nil, err
			}
			if refreshDelayRaw, ok := adsConfig["refresh_delay"]; ok {
				refreshDelay := duration.Duration{}
				err = json.Unmarshal([]byte(refreshDelayRaw), &refreshDelay)
				if err != nil {
					log.DefaultLogger.Errorf("fail to unmarshal refresh_delay: %v", err)
					return nil, nil, err
				}

				d := duration2String(&refreshDelay)
				b, err = json.Marshal(&d)
				adsConfig["refresh_delay"] = jsoniter.RawMessage(b)
			}
			b, err = json.Marshal(&adsConfig)
			if err != nil {
				log.DefaultLogger.Errorf("fail to marshal refresh_delay: %v", err)
				return nil, nil, err
			}
			resources["ads_config"] = jsoniter.RawMessage(b)
			b, err = json.Marshal(&resources)
			if err != nil {
				log.DefaultLogger.Errorf("fail to marshal ads_config: %v", err)
				return nil, nil, err
			}

			err = jsonpb.UnmarshalString(string(b), dynamicResources)
			if err != nil {
				log.DefaultLogger.Errorf("fail to unmarshal dynamic_resources: %v", err)
				return nil, nil, err
			}
			err = dynamicResources.Validate()
			if err != nil {
				log.DefaultLogger.Errorf("invalid dynamic_resources: %v", err)
				return nil, nil, err
			}
		} else {
			log.DefaultLogger.Errorf("ads_config not found")
			err = errors.New("lack of ads_config")
			return nil, nil, err
		}
	}

	if len(config.RawStaticResources) > 0 {
		staticResources = &envoy_config_bootstrap_v3.Bootstrap_StaticResources{}
		var b []byte
		resources := map[string]jsoniter.RawMessage{}
		err = json.Unmarshal([]byte(config.RawStaticResources), &resources)
		if err != nil {
			log.DefaultLogger.Errorf("fail to unmarshal static_resources: %v", err)
			return nil, nil, err
		}
		if clustersRaw, ok := resources["clusters"]; ok {
			clusters := []jsoniter.RawMessage{}
			err = json.Unmarshal([]byte(clustersRaw), &clusters)
			if err != nil {
				log.DefaultLogger.Errorf("fail to unmarshal clusters: %v", err)
				return nil, nil, err
			}
			for i, clusterRaw := range clusters {
				cluster := map[string]jsoniter.RawMessage{}
				err = json.Unmarshal([]byte(clusterRaw), &cluster)
				if err != nil {
					log.DefaultLogger.Errorf("fail to unmarshal cluster: %v", err)
					return nil, nil, err
				}
				cb := envoy_config_cluster_v3.CircuitBreakers{}
				b, err = json.Marshal(&cb)
				if err != nil {
					log.DefaultLogger.Errorf("fail to marshal circuit_breakers: %v", err)
					return nil, nil, err
				}
				cluster["circuit_breakers"] = jsoniter.RawMessage(b)

				b, err = json.Marshal(&cluster)
				if err != nil {
					log.DefaultLogger.Errorf("fail to marshal cluster: %v", err)
					return nil, nil, err
				}
				clusters[i] = jsoniter.RawMessage(b)
			}
			b, err = json.Marshal(&clusters)
			if err != nil {
				log.DefaultLogger.Errorf("fail to marshal clusters: %v", err)
				return nil, nil, err
			}
		}
		resources["clusters"] = jsoniter.RawMessage(b)
		b, err = json.Marshal(&resources)
		if err != nil {
			log.DefaultLogger.Errorf("fail to marshal resources: %v", err)
			return nil, nil, err
		}

		err = jsonpb.UnmarshalString(string(b), staticResources)
		if err != nil {
			log.DefaultLogger.Errorf("fail to unmarshal static_resources: %v", err)
			return nil, nil, err
		}

		err = staticResources.Validate()
		if err != nil {
			log.DefaultLogger.Errorf("Invalid static_resources: %v", err)
			return nil, nil, err
		}
	}
	return dynamicResources, staticResources, nil
}

// Start used to fetch listeners/clusters/clusterloadassignment config from pilot in cycle,
// usually called when mosn start
func (c *clientv3) Start(config *mv2.MOSNConfig) error {
	log.DefaultLogger.Infof("xds client start")

	conv.InitStats()

	dynamicResources, staticResources, err := unmarshalResourcesV3(config)
	if err != nil {
		log.DefaultLogger.Warnf("fail to unmarshal xds resources, skip xds: %v", err)
		return errors.New("fail to unmarshal xds resources")
	}

	xdsConfig := v3.XDSConfig{}
	err = xdsConfig.Init(dynamicResources, staticResources)
	if err != nil {
		log.DefaultLogger.Warnf("fail to init xds config, skip xds: %v", err)
		return errors.New("fail to init xds config")
	}

	stopChan := make(chan int)
	sendControlChan := make(chan int)
	recvControlChan := make(chan int)
	adsClient := &v3.ADSClient{
		AdsConfig:         xdsConfig.ADSConfig,
		StreamClientMutex: sync.RWMutex{},
		StreamClient:      nil,
		MosnConfig:        config,
		SendControlChan:   sendControlChan,
		RecvControlChan:   recvControlChan,
		StopChan:          stopChan,
	}
	adsClient.Start()
	c.adsClient = adsClient
	return nil
}

// Stop used to stop fetch listeners/clusters/clusterloadassignment config from pilot,
// usually called when mosn quit
func (c *clientv3) Stop() {
	if c.adsClient != nil {
		log.DefaultLogger.Infof("prepare to stop xds client")
		c.adsClient.Stop()
		log.DefaultLogger.Infof("xds client stop")
	}
}
