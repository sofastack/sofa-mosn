///*
// * Licensed to the Apache Software Foundation (ASF) under one or more
// * contributor license agreements.  See the NOTICE file distributed with
// * this work for additional information regarding copyright ownership.
// * The ASF licenses this file to You under the Apache License, Version 2.0
// * (the "License"); you may not use this file except in compliance with
// * the License.  You may obtain a copy of the License at
// *
// *     http://www.apache.org/licenses/LICENSE-2.0
// *
// * Unless required by applicable law or agreed to in writing, software
// * distributed under the License is distributed on an "AS IS" BASIS,
// * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// * See the License for the specific language governing permissions and
// * limitations under the License.
// */
//
package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	jsonp "github.com/golang/protobuf/jsonpb"
	"mosn.io/api"
	"mosn.io/mosn/pkg/config/v2"
)

const (
	PseudoHeaderMethod    = ":method"
	PseudoHeaderPath      = ":path" // indicate method name in rpc protocol
	PseudoHeaderScheme    = ":scheme"
	PseudoHeaderAuthority = ":authority"
)

// fetch target value from header, return "" if not found
func headerMapper(target string, headers api.HeaderMap) (string, bool) {
	// TODO: make sure pseudo-header parsing is correct
	switch strings.ToLower(target) {
	case PseudoHeaderMethod:
		return headers.Get("X-Mosn-Method")
	case PseudoHeaderPath:
		return headers.Get("X-Mosn-Path")
	case PseudoHeaderScheme:
		// TODO: parse `:scheme` here
		return "", false
	case PseudoHeaderAuthority:
		return headers.Get("Authority")
	default:
		return headers.Get(target)
	}
}

// parse rbac filter config to v2.RBAC struct
func ParseRbacFilterConfig(cfg map[string]interface{}) (*v2.RBACConfig, error) {
	filterConfig := new(v2.RBACConfig)

	version, ok := cfg["version"]
	if !ok {
		return nil, fmt.Errorf("parseing rbac filter configuration failed, err: missing version field")
	}

	if v, ok := version.(string); ok {
		filterConfig.Version = v
	} else {
		return nil, fmt.Errorf("rbac configuration version must be string")
	}

	jsonConf, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	// parse rules
	un := jsonp.Unmarshaler{
		AllowUnknownFields: true,
	}
	if err = un.Unmarshal(bytes.NewReader(jsonConf), &filterConfig.RBAC); err != nil {
		return nil, err
	}

	return filterConfig, nil
}
