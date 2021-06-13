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

package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitXdsFromBootstrap(t *testing.T) {
	if err := InitXdsFromBootstrap([]byte(`{
  "id": "sidecar~172.31.1.5~sleep-64d7d56698-6z8zw.default~default.svc.cluster.local",
  "cluster": "sleep.default",
  "locality": {
  },
  "metadata": {"APP_CONTAINERS":"sleep","CLUSTER_ID":"Kubernetes","INSTANCE_IPS":"172.31.1.5","INTERCEPTION_MODE":"REDIRECT","ISTIO_PROXY_SHA":"istio-proxy:298ff36b2d43794816f7d8cdc5461bf6eed71bba","ISTIO_VERSION":"1.9.0","LABELS":{"app":"sleep","istio.io/rev":"default","pod-template-hash":"64d7d56698","security.istio.io/tlsMode":"istio","service.istio.io/canonical-name":"sleep","service.istio.io/canonical-revision":"latest"},"MESH_ID":"cluster.local","NAME":"sleep-64d7d56698-6z8zw","NAMESPACE":"default","OWNER":"kubernetes://apis/apps/v1/namespaces/default/deployments/sleep","POD_PORTS":"[\n]","PROXY_CONFIG":{"binaryPath":"/usr/local/bin/envoy","concurrency":2,"configPath":"./etc/istio/proxy","controlPlaneAuthPolicy":"MUTUAL_TLS","discoveryAddress":"istiod.istio-system.svc:15012","drainDuration":"45s","parentShutdownDuration":"60s","proxyAdminPort":15000,"serviceCluster":"sleep.default","statNameLength":189,"statusPort":15020,"terminationDrainDuration":"5s","tracing":{"zipkin":{"address":"zipkin.istio-system:9411"}}},"SERVICE_ACCOUNT":"sleep","WORKLOAD_NAME":"sleep"}
}`)); err != nil {
		t.Fatalf("failed, %s", err)
	}
	t.Logf("ok")
}

func TestInitXdsFlags(t *testing.T) {
	InitXdsFlags("cluster", "node", []string{}, []string{})
	xdsInfo := GetGlobalXdsInfo()

	if !assert.Equal(t, "cluster", xdsInfo.ServiceCluster, "serviceCluster should be 'cluster'") {
		t.FailNow()
	}
	if !assert.Equal(t, "node", xdsInfo.ServiceNode, "serviceNode should be 'node'") {
		t.FailNow()
	}
	//if !assert.Equal(t, 3, len(xdsInfo.Metadata.GetFields()), "serviceMeta len default be three") {
	//	t.FailNow()
	//}

	InitXdsFlags("cluster", "node", []string{
		"IstioVersion:1.1",
		"Not_exist_key:1",
		"not_exist_value",
	}, []string{})
	if !assert.Equal(t, 4, len(xdsInfo.Metadata.GetFields()), "serviceMeta len should be one") {
		t.FailNow()
	}
	if !assert.Equal(t, "1.1", xdsInfo.Metadata.Fields["ISTIO_VERSION"].GetStringValue(), "serviceMeta len should be one") {
		t.FailNow()
	}
	if !assert.Equal(t, "Kubernetes", xdsInfo.Metadata.Fields["CLUSTER_ID"].GetStringValue(), "serviceMeta default specifying network is not Kubernetes") {
		t.FailNow()
	}
}
