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

package rbac

import (
	"go.uber.org/atomic"

	"mosn.io/mosn/pkg/config/v2"
)

/*
   Metric:
	  "filter.rbac.<version>": {
		  "engine.allowed":                         Counter, 	 // total allowed count in engine
		  "engine.denied":                          Counter, 	 // total denied count in engine
		  "engine.hits.${policy1_name}":            Counter,   // policy1 hit count in engine
		  "engine.hits.${policy2_name}":            Counter,   // policy2 hit count in engine
		  "engine.hits.${policy3_name}":            Counter,   // policy3 hit count in engine
		  "shadow_engine.allowed":                  Counter,   // total allowed count in shadow engine
		  "shadow_engine.denied":                   Counter,   // total denied count in shadow engine
		  "shadow_engine.hits.${policy1_name}":     Counter,   // policy1 hit count in shadow engine
		  "shadow_engine.hits.${policy2_name}":     Counter,   // policy2 hit count in shadow engine
		  "shadow_engine.hits.${policy3_name}":     Counter,   // policy3 hit count in shadow engine
	  },
*/

const (
	FilterMetricsType           = "filter.rbac"
	EngineAllowedTotal          = "engine.allowed"
	EngineDeniedTotal           = "engine.denied"
	EnginePoliciesMetrics       = "engine.hits.%s"
	ShadowEngineAllowedTotal    = "shadow_engine.allowed"
	ShadowEngineDeniedTotal     = "shadow_engine.denied"
	ShadowEnginePoliciesMetrics = "shadow_engine.hits.%s"
)

// Status contains the metric logs for rbac filter
type Status struct {
	EngineAllowedTotal          *atomic.Int64
	EngineDeniedTotal           *atomic.Int64
	EnginePoliciesMetrics       map[string]*atomic.Int64
	ShadowEngineAllowedTotal    *atomic.Int64
	ShadowEngineDeniedTotal     *atomic.Int64
	ShadowEnginePoliciesMetrics map[string]*atomic.Int64
}

// NewStatus return the instance of RbacStatus
func NewStatus(config *v2.RBACConfig) *Status {
	status := &Status{}

	status.EngineAllowedTotal = new(atomic.Int64)
	status.EngineDeniedTotal = new(atomic.Int64)
	status.ShadowEngineAllowedTotal = new(atomic.Int64)
	status.ShadowEngineDeniedTotal = new(atomic.Int64)

	status.EnginePoliciesMetrics = make(map[string]*atomic.Int64)

	if config.Rules != nil {
		for name := range config.Rules.Policies {
			status.EnginePoliciesMetrics[name] = new(atomic.Int64)
		}
	}

	if config.ShadowRules != nil {
		status.ShadowEnginePoliciesMetrics = make(map[string]*atomic.Int64)
		for name := range config.ShadowRules.Policies {
			status.ShadowEnginePoliciesMetrics[name] = new(atomic.Int64)
		}
	}

	return status
}
