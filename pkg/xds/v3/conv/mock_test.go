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

package conv

import (
	"context"

	"mosn.io/api"
	"mosn.io/mosn/pkg/types"
)

type mockCMF struct{}

func (cmf *mockCMF) OnCreated(cccb types.ClusterConfigFactoryCb, chcb types.ClusterHostFactoryCb) {}

type mockNetworkFilterFactory struct{}

func (ff *mockNetworkFilterFactory) CreateFilterChain(context context.Context, callbacks api.NetWorkFilterChainFactoryCallbacks) {
}

func CreateMockFilerFactory(conf map[string]interface{}) (api.NetworkFilterChainFactory, error) {
	return &mockNetworkFilterFactory{}, nil
}

func init() {
	api.RegisterNetwork("proxy", CreateMockFilerFactory)
}
