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

package v2

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"mosn.io/api"
	"mosn.io/pkg/utils"
)

type HealthCheckConfig struct {
	Protocol             string                 `json:"protocol,omitempty"`
	TimeoutConfig        api.DurationConfig     `json:"timeout,omitempty"`
	IntervalConfig       api.DurationConfig     `json:"interval,omitempty"`
	IntervalJitterConfig api.DurationConfig     `json:"interval_jitter,omitempty"`
	InitialDelayConfig   *api.DurationConfig    `json:"initial_delay,omitempty"`
	HealthyThreshold     uint32                 `json:"healthy_threshold,omitempty"`
	UnhealthyThreshold   uint32                 `json:"unhealthy_threshold,omitempty"`
	ServiceName          string                 `json:"service_name,omitempty"`
	SessionConfig        map[string]interface{} `json:"check_config,omitempty"`
	CommonCallbacks      []string               `json:"common_callbacks,omitempty"` // HealthCheck support register some common callbacks that are not related to specific cluster
}

type HostConfig struct {
	Address        string          `json:"address,omitempty"`
	Hostname       string          `json:"hostname,omitempty"`
	Weight         uint32          `json:"weight,omitempty"`
	MetaDataConfig *MetadataConfig `json:"metadata,omitempty"`
	TLSDisable     bool            `json:"tls_disable,omitempty"`
}

// ClusterType
type ClusterType string

// Group of cluster type
const (
	STATIC_CLUSTER      ClusterType = "STATIC"
	SIMPLE_CLUSTER      ClusterType = "SIMPLE"
	DYNAMIC_CLUSTER     ClusterType = "DYNAMIC"
	EDS_CLUSTER         ClusterType = "EDS"
	ORIGINALDST_CLUSTER ClusterType = "ORIGINAL_DST"
	STRICT_DNS_CLUSTER  ClusterType = "STRICT_DNS"
)

// LbType
type LbType string

// Group of load balancer type
const (
	LB_RANDOM        LbType = "LB_RANDOM"
	LB_ROUNDROBIN    LbType = "LB_ROUNDROBIN"
	LB_ORIGINAL_DST  LbType = "LB_ORIGINAL_DST"
	LB_LEAST_REQUEST LbType = "LB_LEAST_REQUEST"
	LB_MAGLEV        LbType = "LB_MAGLEV"
)

type DnsLookupFamily string

const (
	V4Only DnsLookupFamily = "V4_ONLY"
	V6Only DnsLookupFamily = "V6_ONLY"
)

// Cluster represents a cluster's information
type Cluster struct {
	Name                 string              `json:"name,omitempty"`
	ClusterType          ClusterType         `json:"type,omitempty"`
	SubType              string              `json:"sub_type,omitempty"` //not used yet
	LbType               LbType              `json:"lb_type,omitempty"`
	MaxRequestPerConn    uint32              `json:"max_request_per_conn,omitempty"`
	ConnBufferLimitBytes uint32              `json:"conn_buffer_limit_bytes,omitempty"`
	CirBreThresholds     CircuitBreakers     `json:"circuit_breakers,omitempty"`
	HealthCheck          HealthCheck         `json:"health_check,omitempty"`
	Spec                 ClusterSpecInfo     `json:"spec,omitempty"`
	LBSubSetConfig       LBSubsetConfig      `json:"lb_subset_config,omitempty"`
	LBOriDstConfig       LBOriDstConfig      `json:"original_dst_lb_config,omitempty"`
	ClusterManagerTLS    bool                `json:"cluster_manager_tls,omitempty"`
	TLS                  TLSConfig           `json:"tls_context,omitempty"`
	Hosts                []Host              `json:"hosts,omitempty"`
	ConnectTimeout       *api.DurationConfig `json:"connect_timeout,omitempty"`
	LbConfig             IsCluster_LbConfig  `json:"lbconfig,omitempty"`
	DnsRefreshRate       *api.DurationConfig `json:"dns_refresh_rate,omitempty"`
	RespectDnsTTL        bool                `json:"respect_dns_ttl,omitempty"`
	DnsLookupFamily      DnsLookupFamily     `json:"dns_lookup_family,omitempty"`
	DnsResolverConfig    DnsResolverConfig   `json:"dns_resolvers,omitempty"`
	DnsResolverFile      string              `json:"dns_resolver_file,omitempty"`
	DnsResolverPort      string              `json:"dns_resolver_port,omitempty"`
}

type DnsResolverConfig struct {
	Servers  []string `json:"servers,omitempty"`
	Search   []string `json:"search,omitempty"`
	Port     string   `json:"port,omitempty"`
	Ndots    int      `json:"ndots,omitempty"`
	Timeout  int      `json:"timeout,omitempty"`
	Attempts int      `json:"attempts,omitempty"`
}

// HealthCheck is a configuration of health check
// use DurationConfig to parse string to time.Duration
type HealthCheck struct {
	HealthCheckConfig
	Timeout        time.Duration `json:"-"`
	Interval       time.Duration `json:"-"`
	IntervalJitter time.Duration `json:"-"`
}

// Marshal implement a json.Marshaler
func (hc HealthCheck) MarshalJSON() (b []byte, err error) {
	hc.HealthCheckConfig.IntervalConfig.Duration = hc.Interval
	hc.HealthCheckConfig.IntervalJitterConfig.Duration = hc.IntervalJitter
	hc.HealthCheckConfig.TimeoutConfig.Duration = hc.Timeout
	return json.Marshal(hc.HealthCheckConfig)
}

func (hc *HealthCheck) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &hc.HealthCheckConfig); err != nil {
		return err
	}
	hc.Timeout = hc.TimeoutConfig.Duration
	hc.Interval = hc.IntervalConfig.Duration
	hc.IntervalJitter = hc.IntervalJitterConfig.Duration
	return nil
}

// Host represenets a host information
type Host struct {
	HostConfig
	MetaData api.Metadata `json:"-"`
}

func (h Host) MarshalJSON() (b []byte, err error) {
	h.HostConfig.MetaDataConfig = metadataToConfig(h.MetaData)
	return json.Marshal(h.HostConfig)
}

func (h *Host) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &h.HostConfig); err != nil {
		return err
	}
	h.MetaData = configToMetadata(h.MetaDataConfig)
	return nil
}

// CircuitBreakers is a configuration of circuit breakers
// CircuitBreakers implements json.Marshaler and json.Unmarshaler
type CircuitBreakers struct {
	Thresholds []Thresholds
}

// CircuitBreakers's implements json.Marshaler and json.Unmarshaler
func (cb CircuitBreakers) MarshalJSON() (b []byte, err error) {
	return json.Marshal(cb.Thresholds)
}
func (cb *CircuitBreakers) UnmarshalJSON(b []byte) (err error) {
	return json.Unmarshal(b, &cb.Thresholds)
}

type Thresholds struct {
	MaxConnections     uint32 `json:"max_connections,omitempty"`
	MaxPendingRequests uint32 `json:"max_pending_requests,omitempty"`
	MaxRequests        uint32 `json:"max_requests,omitempty"`
	MaxRetries         uint32 `json:"max_retries,omitempty"`
}

// ClusterSpecInfo is a configuration of subscribe
type ClusterSpecInfo struct {
	Subscribes []SubscribeSpec `json:"subscribe,omitempty"`
}

// SubscribeSpec describes the subscribe server
type SubscribeSpec struct {
	Subscriber  string `json:"subscriber,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
}

// LBSubsetConfig is a configuration of load balance subset
type LBSubsetConfig struct {
	FallBackPolicy  uint8             `json:"fall_back_policy,omitempty"`
	DefaultSubset   map[string]string `json:"default_subset,omitempty"`
	SubsetSelectors [][]string        `json:"subset_selectors,omitempty"`
}

// LBOriDstConfig for OriDst load balancer.
type LBOriDstConfig struct {
	UseHeader  bool   `json:"use_header,omitempty"`
	HeaderName string `json:"header_name,omitempty"`
}

// ClusterManagerConfig for making up cluster manager
// Cluster is the global cluster of mosn
type ClusterManagerConfig struct {
	ClusterManagerConfigJson
	Clusters []Cluster `json:"-"`
}

type ClusterManagerConfigJson struct {
	TLSContext        TLSConfig `json:"tls_context,omitempty"`
	ClusterConfigPath string    `json:"clusters_configs,omitempty"`
	ClustersJson      []Cluster `json:"clusters,omitempty"`
}

func (cc *ClusterManagerConfig) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &cc.ClusterManagerConfigJson); err != nil {
		return err
	}
	// only one of the config should be exists
	if len(cc.ClustersJson) > 0 && cc.ClusterConfigPath != "" {
		return ErrDuplicateStaticAndDynamic
	}
	if len(cc.ClustersJson) > 0 {
		cc.Clusters = cc.ClustersJson
	}
	// Traversing path and parse the json
	// assume all the files in the path are available json file, and no sub path
	if cc.ClusterConfigPath != "" {
		files, err := ioutil.ReadDir(cc.ClusterConfigPath)
		if err != nil {
			return err
		}
		for _, f := range files {
			fileName := path.Join(cc.ClusterConfigPath, f.Name())
			cluster := Cluster{}
			e := utils.ReadJsonFile(fileName, &cluster)
			switch e {
			case nil:
				cc.Clusters = append(cc.Clusters, cluster)
			case utils.ErrIgnore:
			// do nothing
			default:
				return e
			}
		}
	}
	return nil
}

// Marshal memory config into json, if dynamic mode is configured, write json file
func (cc ClusterManagerConfig) MarshalJSON() (b []byte, err error) {
	if cc.ClusterConfigPath == "" {
		cc.ClustersJson = cc.Clusters
		return json.Marshal(cc.ClusterManagerConfigJson)
	}
	// dynamic mode, should write file
	// first, get all the files in the directory
	// try to make dir if not exists
	os.MkdirAll(cc.ClusterConfigPath, 0755)
	files, err := ioutil.ReadDir(cc.ClusterConfigPath)
	if err != nil {
		return nil, err
	}
	allFiles := make(map[string]struct{}, len(files))
	for _, f := range files {
		allFiles[f.Name()] = struct{}{}
	}
	// file name is virtualhost name, if not exists, use {unixnano}.json
	for _, cluster := range cc.Clusters {
		fileName := cluster.Name
		if fileName == "" {
			fileName = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		data, err := json.MarshalIndent(cluster, "", " ")
		if err != nil {
			return nil, err
		}
		if len(fileName) > MaxFilePath {
			fileName = fileName[:MaxFilePath]
		}
		fileName = strings.ReplaceAll(fileName, sep, "_")
		fileName = fileName + ".json"
		delete(allFiles, fileName)
		fileName = path.Join(cc.ClusterConfigPath, fileName)
		if err := utils.WriteFileSafety(fileName, data, 0644); err != nil {
			return nil, err
		}
	}
	// delete invalid files
	for f := range allFiles {
		os.Remove(path.Join(cc.ClusterConfigPath, f))
	}
	return json.Marshal(cc.ClusterManagerConfigJson)
}
