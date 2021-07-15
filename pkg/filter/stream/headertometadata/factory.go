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

package headertometadata

import (
	"context"
	"encoding/json"
	"errors"

	"mosn.io/api"
)

func init() {
	api.RegisterStream(HeaderToMetadata, CreateFilterFactory)
}

// Stream Filter's Name
const (
	HeaderToMetadata = "header_to_metadata"
)

var (
	ErrEmptyHeader           = errors.New("header must not be empty")
	ErrBothPresentAndMissing = errors.New("cannot specify both on_header_present and on_header_missing")
	ErrNeedPresentOrMissing  = errors.New("one of on_header_present or on_header_missing needs to be specified")
)

type FilterFactory struct {
	Rules []Rule `json:"request_rules"`
}

var _ api.StreamFilterChainFactory = (*FilterFactory)(nil)

type Rule struct {
	Header    string  `json:"header"`
	OnPresent *KVPair `json:"on_header_present"`
	OnMissing *KVPair `json:"on_header_missing"`
	Remove    bool    `json:"remove"`
}

type KVPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CreateFilterChain for create HeaderToMetadata filter
func (f *FilterFactory) CreateFilterChain(context context.Context, callbacks api.StreamFilterChainFactoryCallbacks) {
	filter := NewFilter(f)
	callbacks.AddStreamReceiverFilter(filter, api.AfterRoute)
}

// CreateFilterFactory for create HeaderToMetadata filter factory
func CreateFilterFactory(conf map[string]interface{}) (api.StreamFilterChainFactory, error) {
	data, err := json.Marshal(conf)
	if err != nil {
		return nil, err
	}

	factory := &FilterFactory{}
	err = json.Unmarshal(data, factory)
	if err != nil {
		return nil, err
	}

	// check config
	for _, rule := range factory.Rules {
		if rule.Header == "" {
			return nil, ErrEmptyHeader
		}

		if rule.OnPresent != nil && rule.OnMissing != nil {
			return nil, ErrBothPresentAndMissing
		}

		if rule.OnPresent == nil && rule.OnMissing == nil {
			return nil, ErrNeedPresentOrMissing
		}
	}

	return factory, nil
}
