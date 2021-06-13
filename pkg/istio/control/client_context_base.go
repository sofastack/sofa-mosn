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

package control

import (
	v2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/istio/mixerclient"
	v1 "mosn.io/mosn/pkg/mixer/v1"
)

// ClientContextBase hold mixer client
type ClientContextBase struct {
	config *v2.Mixer
}

// NewClientContextBase for create ClientContextBase
func NewClientContextBase(config *v2.Mixer) *ClientContextBase {
	return &ClientContextBase{
		config: config,
	}
}

// SendReport for send report attributes
func (c *ClientContextBase) SendReport(context *RequestContext) {
	mixerclient.Report(c.config.Transport.ReportCluster, &context.Attributes)
}

// HasMixerConfig return if or not has mixer config
func (c *ClientContextBase) HasMixerConfig() bool {
	return c.config != nil && c.config.MixerAttributes != nil && len(c.config.MixerAttributes.Attributes) > 0
}

// MixerAttributes return mixer attributes
func (c *ClientContextBase) MixerAttributes() *v1.Attributes {
	return c.config.MixerAttributes
}

// AddLocalNodeAttributes for add local node info into attributes
// TODO
func (c *ClientContextBase) AddLocalNodeAttributes(attributes *v1.Attributes) {

}
