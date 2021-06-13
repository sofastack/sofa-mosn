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

package cluster

import (
	"net"
	"regexp"
	"strings"
	"sync"

	"mosn.io/api"
	v2 "mosn.io/mosn/pkg/config/v2"
	mosnctx "mosn.io/mosn/pkg/context"
	"mosn.io/mosn/pkg/types"
)

func init() {
	RegisterLBType(types.ORIGINAL_DST, newOriginalDstLoadBalancer)
}

// LoadBalancer Implementations
type OriginalDstLoadBalancer struct {
	mutex sync.Mutex
	hosts types.HostSet
	host  map[string]types.Host
}

func newOriginalDstLoadBalancer(info types.ClusterInfo, hosts types.HostSet) types.LoadBalancer {
	return &OriginalDstLoadBalancer{
		hosts: hosts,
		host:  make(map[string]types.Host),
	}
}

var (
	ipPattern = regexp.MustCompile("^(?:[0-9]{1,3}\\.){3}[0-9]{1,3}")
)

const (
	localhost = "127.0.0.1"
)

func (lb *OriginalDstLoadBalancer) ChooseHost(lbCtx types.LoadBalancerContext) types.Host {

	var dstAdd string
	ctx := lbCtx.DownstreamContext()
	cluster := lbCtx.DownstreamCluster()
	headers := lbCtx.DownstreamHeaders()
	lbOriDstInfo := cluster.LbOriDstInfo()

	// Check if host header is present, if yes use it otherwise use OriRemoteAddr.
	if lbOriDstInfo.IsEnabled() && headers != nil {
		headername := lbOriDstInfo.GetHeader()
		// default use host header
		if headername == "" {
			headername = "host"
		}
		if hostHeader, ok := headers.Get(headername); ok {
			dstAdd = hostHeader
		}
	}

	if dstAdd == "" {
		oriRemoteAddr := mosnctx.Get(ctx, types.ContextOriRemoteAddr)
		if oriRemoteAddr == nil {
			return nil
		}

		ori := oriRemoteAddr.(*net.TCPAddr)
		dstAdd = ori.String()

	}

	// if does not specify a port and default useing 80.
	if ok := strings.Contains(dstAdd, ":"); !ok {
		dstAdd = dstAdd + ":80"
	}

	if mosnctx.Get(ctx, types.ContextKeyListenerType) == v2.INGRESS {
		dstAdd = ipPattern.ReplaceAllString(dstAdd, localhost)
	}

	var config v2.Host
	config.Address = dstAdd
	config.Hostname = dstAdd

	lb.mutex.Lock()
	defer lb.mutex.Unlock()
	host, ok := lb.host[dstAdd]
	if !ok {
		host = NewSimpleHost(config, cluster)
		lb.host[dstAdd] = host
	}

	return host
}

func (lb *OriginalDstLoadBalancer) IsExistsHosts(metadata api.MetadataMatchCriteria) bool {
	return true
}

func (lb *OriginalDstLoadBalancer) HostNum(metadata api.MetadataMatchCriteria) int {
	return 1
}

type LBOriDstInfoImpl struct {
	useHeader  bool
	headerName string
}

func (info *LBOriDstInfoImpl) IsEnabled() bool {
	return info.useHeader
}

func (info *LBOriDstInfoImpl) GetHeader() string {
	return info.headerName
}

func NewLBOriDstInfo(oridstCfg *v2.LBOriDstConfig) types.LBOriDstInfo {
	dstInfo := &LBOriDstInfoImpl{
		useHeader:  oridstCfg.UseHeader,
		headerName: oridstCfg.HeaderName,
	}

	return dstInfo
}
