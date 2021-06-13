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

package common

import (
	envoy_config_rabc_v3 "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v3"
	"mosn.io/api"
	"mosn.io/mosn/pkg/log"
)

type InheritPolicy struct {
	// The set of permissions that define a role.
	// Each permission is matched with OR semantics.
	// To match all actions for this policy, a single Permission with the `any` field set to true should be used.
	InheritPermissions []InheritPermission
	// The set of principals that are assigned/denied the role based on “action”.
	// Each principal is matched with OR semantics.
	// To match all downstreams for this policy, a single Principal with the `any` field set to true should be used.
	InheritPrincipals []InheritPrincipal
}

// Receive the v2alpha.Policy input and convert it to mosn rbac policy
func NewInheritPolicy(policy *envoy_config_rabc_v3.Policy) (*InheritPolicy, error) {
	inheritPolicy := new(InheritPolicy)
	inheritPolicy.InheritPermissions = make([]InheritPermission, 0)
	inheritPolicy.InheritPrincipals = make([]InheritPrincipal, 0)

	// fill permission
	for _, permission := range policy.Permissions {
		if inheritPermission, err := NewInheritPermission(permission); err != nil {
			log.DefaultLogger.Errorf("[NewInheritPolicy] convert permission failed, error: %v", err)
			return nil, err
		} else {
			inheritPolicy.InheritPermissions = append(inheritPolicy.InheritPermissions, inheritPermission)
		}
	}

	// fill principal
	for _, principal := range policy.Principals {
		if inheritPrincipal, err := NewInheritPrincipal(principal); err != nil {
			log.DefaultLogger.Errorf("[NewInheritPolicy] convert principal failed, error: %v", err)
			return nil, err
		} else {
			inheritPolicy.InheritPrincipals = append(inheritPolicy.InheritPrincipals, inheritPrincipal)
		}
	}

	return inheritPolicy, nil
}

// A policy matches if and only if at least one of its permissions match the action taking place
// AND at least one of its principals match the downstream.
func (inheritPolicy *InheritPolicy) Match(cb api.StreamReceiverFilterHandler, headers api.HeaderMap) bool {
	permissionMatch, principalMatch := false, false
	for _, permission := range inheritPolicy.InheritPermissions {
		if permission.Match(cb, headers) {
			permissionMatch = true
			break
		}
	}

	for _, principal := range inheritPolicy.InheritPrincipals {
		if principal.Match(cb, headers) {
			principalMatch = true
			break
		}
	}

	return permissionMatch && principalMatch
}
