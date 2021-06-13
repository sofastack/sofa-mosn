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

package http

import (
	"github.com/gogo/protobuf/proto"
	"mosn.io/mosn/pkg/istio/control"
	"mosn.io/mosn/pkg/mixer/v1/config/client"
)

// ServiceContext hold service config for HTTP
type ServiceContext struct {
	clientContext *ClientContext

	// service config come from per filter config key "mixer"
	serviceConfig *client.ServiceConfig
}

// NewServiceContext return ServiceContext
func NewServiceContext(clientContext *ClientContext) *ServiceContext {
	return &ServiceContext{
		clientContext: clientContext,
	}
}

// GetClientContext return ClientContext
func (s *ServiceContext) GetClientContext() *ClientContext {
	return s.clientContext
}

// SetServiceConfig set ServiceConfig
func (s *ServiceContext) SetServiceConfig(config *client.ServiceConfig) {
	s.serviceConfig = config
}

// AddStaticAttributes add static mixer attributes.
func (s *ServiceContext) AddStaticAttributes(requestContext *control.RequestContext) {
	s.clientContext.AddLocalNodeAttributes(&requestContext.Attributes)

	if s.clientContext.HasMixerConfig() {
		proto.Merge(&requestContext.Attributes, s.clientContext.MixerAttributes())
	}
	if s.HasMixerConfig() {
		proto.Merge(&requestContext.Attributes, s.serviceConfig.MixerAttributes)
	}
}

// HasMixerConfig return if or not has mixer config
func (s *ServiceContext) HasMixerConfig() bool {
	return s.serviceConfig != nil && s.serviceConfig.MixerAttributes != nil && len(s.serviceConfig.MixerAttributes.Attributes) > 0
}
