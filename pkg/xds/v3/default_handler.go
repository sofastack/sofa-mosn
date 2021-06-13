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

package v3

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"

	envoy_config_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_service_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	resource "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/golang/protobuf/proto"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/types"
	"mosn.io/mosn/pkg/xds/v3/conv"
)

// default type url mosn will handle
const (
	EnvoyListener = resource.ListenerType
	EnvoyCluster  = resource.ClusterType
	EnvoyEndpoint = resource.EndpointType
	EnvoyRoute    = resource.RouteType
)

var (
	responseInfoMap = make(map[string]responseInfo, 10)
	mutex           sync.Mutex
	clusterNames    []string
	routerNames     []string
)

func init() {
	RegisterTypeURLHandleFunc(EnvoyListener, HandleEnvoyListener)
	RegisterTypeURLHandleFunc(EnvoyCluster, HandleEnvoyCluster)
	RegisterTypeURLHandleFunc(EnvoyEndpoint, HandleEnvoyEndpoint)
	RegisterTypeURLHandleFunc(EnvoyRoute, HandleEnvoyRoute)
}

// HandleEnvoyListener parse envoy data to mosn listener config
func HandleEnvoyListener(client *ADSClient, resp *envoy_service_discovery_v3.DiscoveryResponse) {
	// go saveProtobufToFile("lds", protobufPath, resp)
	log.DefaultLogger.Tracef("get lds resp,handle it")
	listeners := client.handleListenersResp(resp)
	log.DefaultLogger.Infof("get %d listeners from LDS", len(listeners))

	conv.ConvertAddOrUpdateListeners(listeners)

	AckResponse(client.StreamClient, resp)
	if err := client.reqRoutes(client.StreamClient); err != nil {
		log.DefaultLogger.Warnf("send thread request rds fail!auto retry next period")
	}
}

// HandleEnvoyCluster parse envoy data to mosn cluster config
func HandleEnvoyCluster(client *ADSClient, resp *envoy_service_discovery_v3.DiscoveryResponse) {
	// go saveProtobufToFile("cds", protobufPath, resp)
	log.DefaultLogger.Tracef("get cds resp,handle it")
	clusters := client.handleClustersResp(resp)
	log.DefaultLogger.Infof("get %d clusters from CDS", len(clusters))
	conv.ConvertUpdateClusters(clusters)

	AckResponse(client.StreamClient, resp)

	clusterNames = make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		if cluster.GetType() == envoy_config_cluster_v3.Cluster_EDS {
			clusterNames = append(clusterNames, cluster.Name)
		}
	}

	if len(clusterNames) != 0 {
		if err := client.reqEndpoints(client.StreamClient, clusterNames); err != nil {
			log.DefaultLogger.Warnf("send thread request eds fail!auto retry next period")
		}
	} else {
		if err := client.reqListeners(client.StreamClient); err != nil {
			log.DefaultLogger.Warnf("send thread request lds fail!auto retry next period")
		}
	}
}

// HandleEnvoyEndpoint parse envoy data to mosn endpoint config
func HandleEnvoyEndpoint(client *ADSClient, resp *envoy_service_discovery_v3.DiscoveryResponse) {
	//go saveProtobufToFile("eds", protobufPath, resp)
	log.DefaultLogger.Tracef("get eds resp,handle it ")
	endpoints := client.handleEndpointsResp(resp)
	log.DefaultLogger.Infof("get %d endpoints from EDS", len(endpoints))
	conv.ConvertUpdateEndpoints(endpoints)

	AckResponse(client.StreamClient, resp)

	if err := client.reqListeners(client.StreamClient); err != nil {
		log.DefaultLogger.Warnf("send thread request lds fail!auto retry next period")
	}
}

// HandleEnvoyRoute parse envoy data to mosn route config
func HandleEnvoyRoute(client *ADSClient, resp *envoy_service_discovery_v3.DiscoveryResponse) {
	//go saveProtobufToFile("rds", protobufPath, resp)
	log.DefaultLogger.Tracef("get rds resp,handle it")
	routes := client.handleRoutesResp(resp)
	log.DefaultLogger.Infof("get %d routes from RDS", len(routes))
	conv.ConvertAddOrUpdateRouters(routes)

	routerNames = make([]string, 0, len(routes))
	for _, rt := range routes {
		routerNames = append(routerNames, rt.Name)
	}

	AckResponse(client.StreamClient, resp)
}

// getResponseRequestInfo get response nonce with request type
func getResponseRequestInfo(reqType string) responseInfo {
	mutex.Lock()
	defer mutex.Unlock()

	rs, ok := responseInfoMap[reqType]
	if ok {
		return rs
	}
	return responseInfo{}
}

// AckResponse response resource nonce
func AckResponse(streamClient envoy_service_discovery_v3.AggregatedDiscoveryService_StreamAggregatedResourcesClient, resp *envoy_service_discovery_v3.DiscoveryResponse) {

	resource := []string{}
	switch resp.TypeUrl {
	case EnvoyEndpoint:
		resource = clusterNames
	case EnvoyRoute:
		resource = routerNames
	}

	mutex.Lock()
	responseInfoMap[resp.TypeUrl] = responseInfo{
		ResponseNonce: resp.Nonce,
		VersionInfo:   resp.VersionInfo,
		ResourceNames: resource,
	}
	mutex.Unlock()

	err := streamClient.Send(&envoy_service_discovery_v3.DiscoveryRequest{
		VersionInfo:   resp.VersionInfo,
		ResourceNames: resource,
		TypeUrl:       resp.TypeUrl,
		ResponseNonce: resp.Nonce,
		ErrorDetail:   nil,
		Node: &envoy_config_core_v3.Node{
			Id:       types.GetGlobalXdsInfo().ServiceNode,
			Cluster:  types.GetGlobalXdsInfo().ServiceCluster,
			Metadata: types.GetGlobalXdsInfo().Metadata,
		},
	})
	if err != nil {
		log.DefaultLogger.Errorf("ack %s fail: %v", resp.TypeUrl, err)
		return
	}

	return
}

func saveProtobufToFile(prefix, path string, messge proto.Message) {
	data, err := proto.Marshal(messge)
	if err != nil {
		if _, err := fmt.Fprintf(os.Stderr, "marshal protobuf failed, %s", err); err != nil {
			println(err)
		}
	}
	if err := ioutil.WriteFile(
		fmt.Sprintf("%s/%s-%d.protobuf", path, prefix, time.Now().Unix()), data, 0644); err != nil {
		if _, err := fmt.Fprintf(os.Stderr, "write protobuf to file failed, %s", err); err != nil {
			println(err)
		}
	}
}

const (
	protobufPath = "/etc/istio/proxy"
)
