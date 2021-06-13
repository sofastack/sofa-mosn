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

package types

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"github.com/golang/protobuf/jsonpb"
	_struct "github.com/golang/protobuf/ptypes/struct"
	"github.com/rcrowley/go-metrics"
	"mosn.io/mosn/pkg/config/v2"
)

const serviceMetaSeparator = ":"

// XdsInfo The xds start parameters
type XdsInfo struct {
	ServiceCluster string
	ServiceNode    string
	Metadata       *_struct.Struct
}

type meta struct {
	// IstioVersion specifies the Istio version associated with the proxy
	IstioVersion string `json:"ISTIO_VERSION,omitempty"`

	// Labels specifies the set of workload instance (ex: k8s pod) labels associated with this node.
	Labels map[string]string `json:"LABELS,omitempty"`

	// InterceptionMode is the name of the metadata variable that carries info about
	// traffic interception mode at the proxy
	InterceptionMode TrafficInterceptionMode `json:"INTERCEPTION_MODE,omitempty"`

	// ClusterID defines the cluster the node belongs to.
	ClusterID string `json:"CLUSTER_ID,omitempty"`
}

var (
	// IstioVersion adapt istio version
	IstioVersion = "unknow"
	ClusterID    = "Kubernetes"

	defaultMeta = &meta{
		IstioVersion:     IstioVersion,
		Labels:           map[string]string{"istio": "ingressgateway"},
		InterceptionMode: InterceptionRedirect,
		ClusterID:        ClusterID,
	}

	// XdsVersion xds version
	XdsVersionV3 = "V3"
	XdsVersion   = XdsVersionV3
)

var globalXdsInfo = &XdsInfo{}

// GetGlobalXdsInfo returns pointer of globalXdsInfo
func GetGlobalXdsInfo() *XdsInfo {
	return globalXdsInfo
}

func InitXdsFromMOSNConfig(config *v2.MOSNConfig, log func(string, ...interface{})) {
	if node := config.Node; node == nil || len(node) <= 0 {
		return
	} else if err := InitXdsFromBootstrap(config.Node); err != nil {
		log("parse bootstrap node config failed, %s", err)
	}
}

func InitXdsFromBootstrap(raw []byte) (err error) {
	var node corev3.Node
	if err = (&jsonpb.Unmarshaler{AllowUnknownFields: true}).Unmarshal(bytes.NewReader(raw), &node); err != nil {
		return
	}
	globalXdsInfo.ServiceCluster = node.Cluster
	globalXdsInfo.ServiceNode = node.Id
	globalXdsInfo.Metadata = node.Metadata
	return
}

func InitXdsFlags(serviceCluster, serviceNode string, serviceMeta []string, labels []string) {
	globalXdsInfo.ServiceCluster = serviceCluster
	globalXdsInfo.ServiceNode = serviceNode
	globalXdsInfo.Metadata = &_struct.Struct{}

	if len(labels) > 0 {
		defaultMeta.Labels = make(map[string]string, len(labels))
		for _, keyValue := range labels {
			keyValueSep := strings.SplitN(keyValue, serviceMetaSeparator, 2)
			if len(keyValueSep) != 2 {
				continue
			}
			defaultMeta.Labels[keyValueSep[0]] = keyValueSep[1]
		}
	}

	for k, v := range GetPodLabels() {
		defaultMeta.Labels[k] = v
	}

	if len(serviceMeta) > 0 {
		for _, keyValue := range serviceMeta {
			keyValueSep := strings.SplitN(keyValue, serviceMetaSeparator, 2)
			if len(keyValueSep) != 2 {
				continue
			}

			f := reflect.ValueOf(defaultMeta).Elem().FieldByName(keyValueSep[0])
			switch f.Kind() {
			case reflect.String:
				f.SetString(keyValueSep[1])
			case reflect.Int:
				if i, e := strconv.ParseInt(keyValueSep[1], 10, 64); e == nil {
					f.SetInt(i)
				}
			}
		}
	}

	bs, _ := json.Marshal(defaultMeta)
	_ = jsonpb.Unmarshal(bytes.NewReader(bs), globalXdsInfo.Metadata)
}

type TrafficInterceptionMode string

const (
	// InterceptionNone indicates that the workload is not using IPtables for traffic interception
	InterceptionNone TrafficInterceptionMode = "NONE"

	// InterceptionTproxy implies traffic intercepted by IPtables with TPROXY mode
	InterceptionTproxy TrafficInterceptionMode = "TPROXY"

	// InterceptionRedirect implies traffic intercepted by IPtables with REDIRECT mode
	// This is our default mode
	InterceptionRedirect TrafficInterceptionMode = "REDIRECT"
)

type NodeType string

const (
	// SidecarProxy type is used for sidecar proxies in the application containers
	SidecarProxy NodeType = "sidecar"

	// Router type is used for standalone proxies acting as L7/L4 routers
	Router NodeType = "router"

	// AllPortsLiteral is the string value indicating all ports
	AllPortsLiteral = "*"
)

type XdsStats struct {
	CdsUpdateSuccess metrics.Counter
	CdsUpdateReject  metrics.Counter
	LdsUpdateSuccess metrics.Counter
	LdsUpdateReject  metrics.Counter
}

func IsApplicationNodeType(nType string) bool {
	switch NodeType(nType) {
	case SidecarProxy, Router:
		return true
	default:
		return false
	}
}
