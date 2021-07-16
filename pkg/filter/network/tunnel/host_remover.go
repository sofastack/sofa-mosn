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

package tunnel

import (
	"mosn.io/api"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/upstream/cluster"
)

// HostRemover is the implementation of ConnectionEventListener, which removes the disconnected
// connection from ClusterManager by listening to the Connection-Closed event
type HostRemover struct {
	address string
	cluster string
}

func NewHostRemover(address string, cluster string) *HostRemover {
	return &HostRemover{address: address, cluster: cluster}
}

func (h *HostRemover) OnEvent(event api.ConnectionEvent) {
	if event.IsClose() {
		log.DefaultLogger.Infof("try to remove host: %v in cluster: %v", h.address, h.cluster)
		tunnelHostMutex.Lock()
		cluster.GetClusterMngAdapterInstance().RemoveClusterHosts(h.cluster, []string{h.address})
		tunnelHostMutex.Unlock()
		cluster.GetClusterMngAdapterInstance().ShutdownConnectionPool("", h.address)
	}
}
