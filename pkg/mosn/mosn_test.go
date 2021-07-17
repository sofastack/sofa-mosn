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

package mosn

import (
	"encoding/json"
	"testing"
	"time"

	"mosn.io/mosn/pkg/config/v2"
	_ "mosn.io/mosn/pkg/filter/network/connectionmanager"
	_ "mosn.io/mosn/pkg/filter/network/proxy"
	"mosn.io/mosn/pkg/router"
	logger "mosn.io/pkg/log"
)

// test compatible
const mosnConfigOld = `{
	"servers":[{
		"default_log_path": "stdout",
		"listeners":[{
			"name":"serverListener",
			"address": "127.0.0.1:8080",
			"bind_port": true,
			"filter_chains": [{
				"filters": [
					{
						"type": "proxy",
						"config": {
							"downstream_protocol": "Http1",
							"upstream_protocol": "Http1",
							"router_config_name":"server_router"
						}
					},
					{
						"type": "connection_manager",
						"config":{
							"router_config_name":"server_router",
							"virtual_hosts":[{
								"domains": ["*"],
								"routers": [{
									"match":{"prefix":"/"},
									"route":{"cluster_name":"serverCluster"}
								}]
							}]
						}
					}
				]
			}]
		}]
	}],
	"cluster_manager":{
		"clusters":[{
			"name":"serverCluster",
			"type": "SIMPLE",
			"lb_type": "LB_RANDOM",
			"hosts":[]
		}]
	}
}`

// test new config
const mosnConfigNew = `{
	"servers":[{
		"default_log_path": "stdout",
		"listeners":[{
			"name":"serverListener",
			"address": "127.0.0.1:8080",
			"bind_port": true,
			"filter_chains": [{
				"filters": [
					{
						"type": "proxy",
						"config": {
							"downstream_protocol": "Http1",
							"upstream_protocol": "Http1",
							"router_config_name":"server_router"
						}
					},
					{
						"type": "connection_manager",
						"config":{
							"router_config_name":"server_router",
							"virtual_hosts":[{
								"domains": ["*"],
								"routers": [{
									"match":{"prefix":"/"},
									"route":{"cluster_name":"serverCluster"}
								}]
							}]
						}
					}
				]
			}]
		}],
		"routers": [
			{
				"router_config_name":"server_router",
				"virtual_hosts":[
					{
						"name": "virtualhost00",
						"domains": ["www.server.com"],
						"routers": [{
							"match":{"prefix":"/"},
						 	"route":{"cluster_name":"serverCluster"}
						}]
					},
					{
						"name": "virtualhost01",
						 "domains": ["www.test.com"],
						 "routers": [{
							 "match":{"prefix":"/"},
							 "route":{"cluster_name":"serverCluster"}
						 }]
					}
				]
			},
			{
				 "router_config_name":"test_router",
				 "virtual_hosts":[{
					 "domains": ["*"],
					 "routers": [{
						 "match":{"prefix":"/"},
						 "route":{"cluster_name":"serverCluster"}
					 }]
				 }]
			}
		]
	}],
	"cluster_manager":{
		"clusters":[{
			"name":"serverCluster",
			"type": "SIMPLE",
			"lb_type": "LB_RANDOM",
			"hosts":[]
		}]
	}
}`

func TestNewMosn(t *testing.T) {
	// comatible for old version config, routers in connection_manager
	t.Run("comatible for old version config, routers in connection_manager", func(t *testing.T) {
		cfg := &v2.MOSNConfig{}
		content := []byte(mosnConfigOld)
		if err := json.Unmarshal(content, cfg); err != nil {
			t.Fatal(err)
		}
		DefaultInitStage(cfg)
		NewMosn(cfg)
		// verify routers
		routerMng := router.GetRoutersMangerInstance()
		rw := routerMng.GetRouterWrapperByName("server_router")
		if rw == nil {
			t.Fatal("no router parsed success")
		}
		rcfg := rw.GetRoutersConfig()
		if len(rcfg.VirtualHosts) != 1 {
			t.Fatal("router config is not expected")
		}
	})
	t.Run("new version config, use routers instead of connection_manager", func(t *testing.T) {
		cfg := &v2.MOSNConfig{}
		content := []byte(mosnConfigNew)
		if err := json.Unmarshal(content, cfg); err != nil {
			t.Fatal(err)
		}
		DefaultInitStage(cfg)
		m := NewMosn(cfg)
		routerMng := router.GetRoutersMangerInstance()
		rw0 := routerMng.GetRouterWrapperByName("server_router")
		if rw0 == nil {
			t.Fatal("no router parsed success")
		}
		if rcfg := rw0.GetRoutersConfig(); len(rcfg.VirtualHosts) != 2 {
			t.Fatal("router config is not expected")
		}
		rw1 := routerMng.GetRouterWrapperByName("test_router")
		if rw1 == nil {
			t.Fatal("no router parsed success")
		}
		if rcfg := rw1.GetRoutersConfig(); len(rcfg.VirtualHosts) != 1 {
			t.Fatal("router config is not expected")
		}

		// start mosn
		m.Start()
		time.Sleep(time.Second * 3)
		//go test must ClearAll logger because they're going to disrupt other unit tests;  more #1654
		logger.ClearAll()
		time.Sleep(time.Second * 3)
		// stop mosn
		m.Close()
	})
}
