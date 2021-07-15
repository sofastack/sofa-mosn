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
	"time"

	"github.com/c2h5oh/datasize"
	xdsboot "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v2"
	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc/keepalive"
)

// MOSNConfig make up mosn to start the mosn project
// Servers contains the listener, filter and so on
// ClusterManager used to manage the upstream
type MOSNConfig struct {
	Servers                      []ServerConfig       `json:"servers,omitempty"`                //server config
	ClusterManager               ClusterManagerConfig `json:"cluster_manager,omitempty"`        //cluster config
	CloseGraceful                bool                 `json:"close_graceful,omitempty"`         // graceful switch, default false
	InheritOldMosnconfig         bool                 `json:"inherit_old_mosnconfig,omitempty"` // inherit old mosn config switch, default false
	Tracing                      TracingConfig        `json:"tracing,omitempty"`
	Metrics                      MetricsConfig        `json:"metrics,omitempty"`
	RawDynamicResources          json.RawMessage      `json:"dynamic_resources,omitempty"` //dynamic_resources raw message
	RawStaticResources           json.RawMessage      `json:"static_resources,omitempty"`  //static_resources raw message
	RawAdmin                     json.RawMessage      `json:"admin,omitempty"`             // admin raw message
	Debug                        PProfConfig          `json:"pprof,omitempty"`
	Pid                          string               `json:"pid,omitempty"`                 // pid file
	Plugin                       PluginConfig         `json:"plugin,omitempty"`              // plugin config
	ThirdPartCodec               ThirdPartCodecConfig `json:"third_part_codec,omitempty"`    // third part codec config
	Extends                      []ExtendConfig       `json:"extends,omitempty"`             // extend config
	Wasms                        []WasmPluginConfig   `json:"wasm_global_plugins,omitempty"` // wasm config
	BranchTransactionServicePort int                  `json:"branchTransactionServicePort,omitempty"`
	EnforcementPolicy            struct {
		MinTime             time.Duration `json:"minTime"`
		PermitWithoutStream bool          `json:"permitWithoutStream"`
	} `json:"enforcementPolicy"`

	ServerParameters struct {
		MaxConnectionIdle     time.Duration `json:"maxConnectionIdle"`
		MaxConnectionAge      time.Duration `json:"maxConnectionAge"`
		MaxConnectionAgeGrace time.Duration `json:"maxConnectionAgeGrace"`
		Time                  time.Duration `json:"time"`
		Timeout               time.Duration `json:"timeout"`
	} `json:"serverParameters"`
}

// GetEnforcementPolicy used to config grpc connection keep alive
func (c *MOSNConfig) GetEnforcementPolicy() keepalive.EnforcementPolicy {
	ep := keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second,
		PermitWithoutStream: true,
	}
	if c.EnforcementPolicy.MinTime > 0 {
		ep.MinTime = c.EnforcementPolicy.MinTime
	}
	ep.PermitWithoutStream = c.EnforcementPolicy.PermitWithoutStream
	return ep
}

// GetServerParameters used to config grpc connection keep alive
func (c *MOSNConfig) GetServerParameters() keepalive.ServerParameters {
	sp := keepalive.ServerParameters{
		MaxConnectionIdle:     15 * time.Second,
		MaxConnectionAge:      30 * time.Second,
		MaxConnectionAgeGrace: 5 * time.Second,
		Time:                  5 * time.Second,
		Timeout:               time.Second,
	}
	if c.ServerParameters.MaxConnectionIdle > 0 {
		sp.MaxConnectionIdle = c.ServerParameters.MaxConnectionIdle
	}
	if c.ServerParameters.MaxConnectionAge > 0 {
		sp.MaxConnectionAge = c.ServerParameters.MaxConnectionAge
	}
	if c.ServerParameters.MaxConnectionAgeGrace > 0 {
		sp.MaxConnectionAgeGrace = c.ServerParameters.MaxConnectionAgeGrace
	}
	if c.ServerParameters.Time > 0 {
		sp.Time = c.ServerParameters.Time
	}
	if c.ServerParameters.Timeout > 0 {
		sp.Timeout = c.ServerParameters.Timeout
	}
	return sp
}

// PProfConfig is used to start a pprof server for debug
type PProfConfig struct {
	StartDebug bool `json:"debug"`      // If StartDebug is true, start a pprof, default is false
	Port       int  `json:"port_value"` // If port value is 0, will use 9090 as default
}

// Tracing configuration for a server
type TracingConfig struct {
	Enable bool                   `json:"enable,omitempty"`
	Tracer string                 `json:"tracer,omitempty"` // DEPRECATED
	Driver string                 `json:"driver,omitempty"`
	Config map[string]interface{} `json:"config,omitempty"`
}

// MetricsConfig for metrics sinks
type MetricsConfig struct {
	SinkConfigs  []Filter          `json:"sinks"`
	StatsMatcher StatsMatcher      `json:"stats_matcher"`
	ShmZone      string            `json:"shm_zone"`
	ShmSize      datasize.ByteSize `json:"shm_size"`
	FlushMosn    bool              `json:"flush_mosn"`
	LazyFlush    bool              `json:"lazy_flush"`
}

// PluginConfig for plugin config
type PluginConfig struct {
	LogBase string `json:"log_base"`
}

// ThirdPartCodecType represents type of a third part codec
type ThirdPartCodecType string

// Third part codec consts
const (
	GoPlugin ThirdPartCodecType = "go-plugin"
	Wasm     ThirdPartCodecType = "wasm"
)

// ThirdPartCodec represents configuration for a third part codec
type ThirdPartCodec struct {
	Enable         bool                   `json:"enable,omitempty"`
	Type           ThirdPartCodecType     `json:"type,omitempty"`
	Path           string                 `json:"path,omitempty"`
	LoaderFuncName string                 `json:"loader_func_name,omitempty"`
	Config         map[string]interface{} `json:"config,omitempty"`
}

// ThirdPartCodecConfig represents configurations for third part codec
type ThirdPartCodecConfig struct {
	Codecs []ThirdPartCodec `json:"codecs"`
}

// ExtendConfig for any extends
type ExtendConfig struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

// StatsMatcher is a configuration for disabling stat instantiation.
// TODO: support inclusion_list
// TODO: support exclusion list/inclusion_list as pattern
type StatsMatcher struct {
	RejectAll       bool     `json:"reject_all,omitempty"`
	ExclusionLabels []string `json:"exclusion_labels,omitempty"`
	ExclusionKeys   []string `json:"exclusion_keys,omitempty"`
}

// Mode is mosn's starting type
type Mode uint8

// File means start from config file
// Xds means start from xds
// Mix means start both from file and Xds
const (
	File Mode = iota
	Xds
	Mix
)

func (c *MOSNConfig) Mode() Mode {
	if len(c.Servers) > 0 {
		if len(c.RawStaticResources) == 0 || len(c.RawDynamicResources) == 0 {
			return File
		}

		return Mix
	}
	if len(c.RawStaticResources) > 0 && len(c.RawDynamicResources) > 0 {
		return Xds
	}

	return File
}

func (c *MOSNConfig) GetAdmin() *xdsboot.Admin {
	if len(c.RawAdmin) > 0 {
		adminConfig := &xdsboot.Admin{}
		err := jsonpb.UnmarshalString(string(c.RawAdmin), adminConfig)
		if err == nil {
			return adminConfig
		}
	}
	return nil
}
