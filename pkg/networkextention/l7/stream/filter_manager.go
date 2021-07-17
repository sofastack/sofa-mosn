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
	"context"
	"os"

	"mosn.io/mosn/pkg/streamfilter"
)

const (
	// TODO using mapping port stuct to dynamic get
	DefaultFilterChainName string = "2990"

	AsyncEnvName string = "ENABLEMOEASYNCMOD"
)

var enableFilterAsyncMode bool

func init() {
	parseFilterAsyncMod(AsyncEnvName)
}

// TODO support auto config
func parseFilterAsyncMod(envName string) {
	enableFilterAsyncMode = false
	asyncEnable := os.Getenv(envName)
	if asyncEnable != "" {
		enableFilterAsyncMode = true
	}
}

// TODO remove it or as a global switch for check async?
// GetFilterAsyncMod is check filter enables asynchronous running mode.
func GetFilterAsyncMod() bool {
	return enableFilterAsyncMode
}

func AddOrUpdateFilter(filterChainName string, streamFiltersConfig streamfilter.StreamFiltersConfig) {
	streamfilter.GetStreamFilterManager().AddOrUpdateStreamFilterConfig(filterChainName, streamFiltersConfig)
}

func CreateStreamFilter(ctx context.Context, filterChainName string) *streamfilter.DefaultStreamFilterChainImpl {
	fm := streamfilter.GetDefaultStreamFilterChain()
	streamFilterFactory := streamfilter.GetStreamFilterManager().GetStreamFilterFactory(filterChainName)
	streamFilterFactory.CreateFilterChain(ctx, fm)
	return fm
}

func DestoryStreamFilter(fm *streamfilter.DefaultStreamFilterChainImpl) {
	if fm != nil {
		streamfilter.PutStreamFilterChain(fm)
	}
}
