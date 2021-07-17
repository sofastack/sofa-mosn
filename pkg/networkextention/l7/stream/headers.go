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

package stream

import (
	"mosn.io/api"
	"mosn.io/mosn/pkg/networkextention/utils"
	"mosn.io/mosn/pkg/protocol"
)

// TODO using xprotocol.Header and reuse by sync pool
type Headers struct {
	protocol.CommonHeader
	Update map[string]string
}

// Set key-value pair in header map, the previous pair will be replaced if exists
func (h *Headers) Set(key string, value string) {
	h.CommonHeader.Set(key, value)
	h.Update[key] = value
}

// Del delete pair of specified key
func (h *Headers) Del(key string) {
	h.CommonHeader.Del(key)
	h.Update[key] = ""
}

// Clone used to deep copy header's map
func (h *Headers) Clone() api.HeaderMap {
	clone := &Headers{}
	clone.CommonHeader = (h.CommonHeader.Clone()).(protocol.CommonHeader)
	copy := make(map[string]string)

	for k, v := range h.Update {
		copy[k] = v
	}
	clone.Update = copy
	return clone
}

// DirectDeepCopyKeyValue used to deep copy header's key and value
func (h *Headers) DirectDeepCopyKeyValue() {
	deepCopy := make(map[string]string)
	h.Range(func(k, v string) bool {
		deepCopy[utils.DeepCopyString(k)] = utils.DeepCopyString(v)
		return true
	})
	h.CommonHeader = protocol.CommonHeader(deepCopy)
	//Note: the member 'Update' of 'Headers' does not need deep copy.
}
