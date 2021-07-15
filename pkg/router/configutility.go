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

package router

import (
	"context"
	"regexp"
	"sort"

	"mosn.io/api"
	v2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/types"
	"mosn.io/mosn/pkg/variable"
)

// StringMatch describes hwo to match a given string.
// support regex-based match or exact string match (case-sensitive)
type StringMatch struct {
	Value        string
	IsRegex      bool
	RegexPattern *regexp.Regexp
}

func (sm StringMatch) Matches(s string) bool {
	if !sm.IsRegex {
		return s == sm.Value
	}
	if sm.RegexPattern != nil {
		return sm.RegexPattern.MatchString(s)
	}
	return false
}

// KeyValueData represents a key-value pairs.
// The value is a StringMatch
// used in HeaderMatch and QueryParamsMatch
type KeyValueData struct {
	Name  string // name should be lower case in router headerdata
	Value StringMatch
}

func (k *KeyValueData) Key() string {
	return k.Name
}

func (k *KeyValueData) MatchType() api.KeyValueMatchType {
	if k.Value.IsRegex {
		return api.ValueRegex
	}
	return api.ValueExact
}

func (k *KeyValueData) Matcher() string {
	return k.Value.Value
}

func NewKeyValueData(header v2.HeaderMatcher) (*KeyValueData, error) {
	kvData := &KeyValueData{
		Name: header.Name,
		Value: StringMatch{
			Value:   header.Value,
			IsRegex: header.Regex,
		},
	}
	if header.Regex {
		p, err := regexp.Compile(header.Value)
		if err != nil {
			log.DefaultLogger.Errorf("parse route header macther config failed, ignore it, error: %v", err)
			return nil, err
		}
		kvData.Value.RegexPattern = p
	}
	return kvData, nil
}

// commonHeaderMatcherImpl implements a simple types.HeaderMacther
type commonHeaderMatcherImpl []*KeyValueData

func (m commonHeaderMatcherImpl) Get(i int) api.KeyValueMatchCriterion {
	return m[i]
}

func (m commonHeaderMatcherImpl) Len() int {
	return len(m)
}

func (m commonHeaderMatcherImpl) Range(f func(api.KeyValueMatchCriterion) bool) {
	for _, kv := range m {
		// stop if f return false
		if !f(kv) {
			break
		}
	}
}

func (m commonHeaderMatcherImpl) HeaderMatchCriteria() api.KeyValueMatchCriteria {
	return m
}

func (m commonHeaderMatcherImpl) Matches(_ context.Context, headers api.HeaderMap) bool {
	if log.DefaultLogger.GetLogLevel() >= log.DEBUG {
		log.DefaultLogger.Debugf(RouterLogFormat, "config utility", "try match header", headers)
	}
	for _, headerData := range m {
		cfgName := headerData.Name
		// if a condition is not matched, return false
		// ll condition matched, return true
		value, exists := headers.Get(cfgName)
		if !exists {
			return false
		}
		if !headerData.Value.Matches(value) {
			return false
		}
	}
	return true
}

func CreateCommonHeaderMatcher(headers []v2.HeaderMatcher) types.HeaderMatcher {
	hm := make(commonHeaderMatcherImpl, 0, len(headers))
	for _, header := range headers {
		if kv, err := NewKeyValueData(header); err == nil {
			hm = append(hm, kv)
		}
	}
	return hm
}

// http header matcher is quite different from common header matcher.
// some keys in the header will be matched in variables and support exact match only
type httpHeaderMatcherImpl struct {
	variables map[string]string
	headers   commonHeaderMatcherImpl
}

func (m *httpHeaderMatcherImpl) HeaderMatchCriteria() api.KeyValueMatchCriteria {
	if m.headers != nil {
		return m.headers.HeaderMatchCriteria()
	}
	return nil
}

func (m *httpHeaderMatcherImpl) Matches(ctx context.Context, headers api.HeaderMap) bool {
	if log.DefaultLogger.GetLogLevel() >= log.DEBUG {
		log.DefaultLogger.Debugf(RouterLogFormat, "config utility", "try match http header", headers)
	}
	// check http variables
	for vkey, vvalue := range m.variables {
		value, err := variable.GetString(ctx, vkey)
		if err != nil {
			return false
		}
		if value != vvalue {
			return false
		}
	}
	return m.headers.Matches(ctx, headers)
}

// CreateHTTPHeaderMatcher creates a http header matcher as a types.HeaderMatcher
func CreateHTTPHeaderMatcher(headers []v2.HeaderMatcher) types.HeaderMatcher {
	matcher := &httpHeaderMatcherImpl{
		variables: make(map[string]string, 1),
		headers:   make(commonHeaderMatcherImpl, 0, len(headers)),
	}
	for _, header := range headers {
		switch header.Name {
		case "method":
			matcher.variables[types.VarMethod] = header.Value
		default:
			if kv, err := NewKeyValueData(header); err == nil {
				matcher.headers = append(matcher.headers, kv)
			}
		}
	}
	return matcher

}

// TODO: support query params match
// queryParameterMatcherImpl implements a types.QueryParamsMatcher
// nolint: unused
type queryParameterMatcherImpl []*KeyValueData

func (m queryParameterMatcherImpl) Matches(ctx context.Context, queryParams types.QueryParams) bool {
	if log.DefaultLogger.GetLogLevel() >= log.DEBUG {
		log.DefaultLogger.Debugf(RouterLogFormat, "config utility", "try match query params", queryParams)
	}
	for _, configQueryParam := range m {
		cfgName := configQueryParam.Name
		value, ok := queryParams[cfgName]
		if !ok {
			return false
		}
		if !configQueryParam.Value.Matches(value) {
			return false
		}
	}
	return true
}

// NewConfigImpl return an configImpl instance contains requestHeadersParser and responseHeadersParser
func NewConfigImpl(routerConfig *v2.RouterConfiguration) *configImpl {
	return &configImpl{
		requestHeadersParser:  getHeaderParser(routerConfig.RequestHeadersToAdd, nil),
		responseHeadersParser: getHeaderParser(routerConfig.ResponseHeadersToAdd, routerConfig.ResponseHeadersToRemove),
	}
}

// Implementation of Config that reads from a proto file.
// TODO: more action
type configImpl struct {
	requestHeadersParser  *headerParser
	responseHeadersParser *headerParser
}

// NewMetadataMatchCriteriaImpl
func NewMetadataMatchCriteriaImpl(metadataMatches map[string]string) *MetadataMatchCriteriaImpl {

	metadataMatchCriteriaImpl := &MetadataMatchCriteriaImpl{}
	metadataMatchCriteriaImpl.merge(nil, metadataMatches)

	return metadataMatchCriteriaImpl
}

// MetadataMatchCriteriaImpl class wrapper MatchCriteriaArray
// which contains MatchCriteria in dictionary sorted
type MetadataMatchCriteriaImpl struct {
	MatchCriteriaArray []api.MetadataMatchCriterion
}

// MetadataMatchCriteria
func (mmcti *MetadataMatchCriteriaImpl) MetadataMatchCriteria() []api.MetadataMatchCriterion {
	return mmcti.MatchCriteriaArray
}

// MergeMatchCriteria
// No usage currently
func (mmcti *MetadataMatchCriteriaImpl) MergeMatchCriteria(metadataMatches map[string]string) api.MetadataMatchCriteria {
	mmcti.merge(mmcti, metadataMatches)
	return mmcti
}

func (mmcti *MetadataMatchCriteriaImpl) Len() int {
	return len(mmcti.MatchCriteriaArray)
}

func (mmcti *MetadataMatchCriteriaImpl) Less(i, j int) bool {
	return mmcti.MatchCriteriaArray[i].MetadataKeyName() < mmcti.MatchCriteriaArray[j].MetadataKeyName()
}

func (mmcti *MetadataMatchCriteriaImpl) Swap(i, j int) {
	mmcti.MatchCriteriaArray[i], mmcti.MatchCriteriaArray[j] = mmcti.MatchCriteriaArray[j],
		mmcti.MatchCriteriaArray[i]
}

// Used to generate metadata match criteria from config
func (mmcti *MetadataMatchCriteriaImpl) merge(parent *MetadataMatchCriteriaImpl, metadataMatches map[string]string) {

	var mdMatchCriteria []api.MetadataMatchCriterion
	// used to record key and its index for o(1) searching
	var existingMap = make(map[string]uint32)

	// get from parent
	if nil != parent {
		for _, v := range parent.MetadataMatchCriteria() {
			existingMap[v.MetadataKeyName()] = uint32(len(mdMatchCriteria))
			mdMatchCriteria = append(mdMatchCriteria, v)
		}
	}

	// get from metadatamatch
	for k, v := range metadataMatches {
		mmci := &MetadataMatchCriterionImpl{
			Name:  k,
			Value: v,
		}

		if index, ok := existingMap[k]; ok {
			// update value
			mdMatchCriteria[index] = mmci
		} else {
			// append
			mdMatchCriteria = append(mdMatchCriteria, mmci)
		}
	}

	mmcti.MatchCriteriaArray = mdMatchCriteria
	// sorting in lexically by name
	sort.Sort(mmcti)
}

// MetadataMatchCriterionImpl class contains the name and value of the metadata match criterion
// Implement types.MetadataMatchCriterion
type MetadataMatchCriterionImpl struct {
	Name  string
	Value string
}

// MetadataKeyName return name
func (mmci *MetadataMatchCriterionImpl) MetadataKeyName() string {
	return mmci.Name
}

// MetadataValue return value
func (mmci *MetadataMatchCriterionImpl) MetadataValue() string {
	return mmci.Value
}
