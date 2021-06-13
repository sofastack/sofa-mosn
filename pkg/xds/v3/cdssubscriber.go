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
	"errors"

	envoy_config_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_service_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/golang/protobuf/ptypes"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/types"
)

func (c *ADSClient) reqClusters(streamClient envoy_service_discovery_v3.AggregatedDiscoveryService_StreamAggregatedResourcesClient) error {
	if streamClient == nil {
		return errors.New("stream client is nil")
	}

	rs := getResponseRequestInfo(EnvoyCluster)
	err := streamClient.Send(&envoy_service_discovery_v3.DiscoveryRequest{
		VersionInfo:   rs.VersionInfo,
		ResourceNames: rs.ResourceNames,
		TypeUrl:       EnvoyCluster,
		ResponseNonce: rs.ResponseNonce,
		ErrorDetail:   nil,
		Node: &envoy_config_core_v3.Node{
			Id:       types.GetGlobalXdsInfo().ServiceNode,
			Cluster:  types.GetGlobalXdsInfo().ServiceCluster,
			Metadata: types.GetGlobalXdsInfo().Metadata,
		}})

	if err != nil {
		log.DefaultLogger.Infof("get clusters fail: %v", err)
		return err
	}
	return nil
}

func (c *ADSClient) handleClustersResp(resp *envoy_service_discovery_v3.DiscoveryResponse) []*envoy_config_cluster_v3.Cluster {
	clusters := make([]*envoy_config_cluster_v3.Cluster, 0, len(resp.Resources))
	for _, res := range resp.Resources {
		cluster := &envoy_config_cluster_v3.Cluster{}
		if err := ptypes.UnmarshalAny(res, cluster); err != nil {
			log.DefaultLogger.Errorf("ADSClient unmarshal cluster fail: %v", err)
		}
		clusters = append(clusters, cluster)
	}
	return clusters
}
