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
package msgconnpool

import (
	"errors"
	"math"
	"sync"
	"sync/atomic"

	"mosn.io/mosn/pkg/log"

	v2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/upstream/cluster"

	"mosn.io/api"
	"mosn.io/mosn/pkg/types"
	"mosn.io/pkg/buffer"
)

// the state for message conn
type State int

const (
	Available State = iota
	Connecting
	Destroyed
)

// Connection is abstraction for message conn
// which can be configured to auto reconnect when the under conn is broken
type Connection interface {
	Write(buf ...buffer.IoBuffer) error // write to current connection
	State() State                       // get the conn detailed state
	Destroy()                           // destroy the current conn
}

type connpool struct {
	client        *activeClient
	host          types.Host
	keepalive     KeepAlive
	connectingMux sync.Mutex // all the connect behavior should be serial

	autoReconnectWhenClose bool
	connTryTimes           int
	destroyed              uint64

	// when connection change, readFilter need to change, use creator to create new filter chain
	getReadFilterAndKeepalive func() ([]api.ReadFilter, KeepAlive)
}

// NewConn returns a simplified connpool
func NewConn(hostAddr string, connectTryTimes int,
	getReadFilterAndKeepalive func() ([]api.ReadFilter, KeepAlive), autoReconnectWhenClose bool) Connection {
	// use connData addr as cluster name, for the count of metrics
	cl := basicCluster(hostAddr, []string{hostAddr})
	host := cluster.NewSimpleHost(cl.Hosts[0], cluster.NewCluster(cl).Snapshot().ClusterInfo())

	// if user configure this to -1, then retry is unlimited
	if connectTryTimes == -1 {
		connectTryTimes = math.MaxInt32
	}

	p := &connpool{
		host:                      host,
		autoReconnectWhenClose:    autoReconnectWhenClose,
		connTryTimes:              connectTryTimes,
		getReadFilterAndKeepalive: getReadFilterAndKeepalive,
	}

	p.initActiveClient()

	return p
}

func (p *connpool) Host() types.Host {
	return p.host
}

///////////// Connection interface start

// Write to client
func (p *connpool) Write(buf ...buffer.IoBuffer) error {
	if h, state := p.getConnAndState(); state == Available {
		return h.Connection.Write(buf...)
	}
	return errors.New("[connpool] connection not ready, host" + p.Host().AddressString())
}

// State current available to send request
func (p *connpool) State() State {
	_, state := p.getConnAndState()
	return state
}

// Destroy the pool
func (p *connpool) Destroy() {
	if !atomic.CompareAndSwapUint64(&p.destroyed, 0, 1) {
		if log.DefaultLogger.GetLogLevel() >= log.WARN {
			log.DefaultLogger.Warnf("[connpool] duplicate destroy call, host: %v", p.Host().AddressString())
		}
		return
	}

	p.client.clearHeartBeater()
	p.client.getConnData().Connection.Close(api.NoFlush, api.LocalClose)
}

///////////// Connection interface end

// generate the client, and set it to the connpool
func (p *connpool) initActiveClient() {
	p.client = &activeClient{
		pool: p,
	}

	p.connectingMux.Lock()
	defer p.connectingMux.Unlock()

	p.client.initConnectionLocked("FirstConnect")
}

func (p *connpool) getConnAndState() (*types.CreateConnectionData, State) {
	// if pool was destroyed
	if atomic.LoadUint64(&p.destroyed) == 1 {
		return nil, Destroyed
	}

	h := p.client.getConnData()
	if h != nil && h.Connection.State() == api.ConnActive {
		return h, Available
	}

	return h, Connecting
}

// to adapt the host api
func basicCluster(name string, hosts []string) v2.Cluster {
	var vhosts []v2.Host
	for _, addr := range hosts {
		vhosts = append(vhosts, v2.Host{
			HostConfig: v2.HostConfig{
				Address: addr,
			},
		})
	}
	return v2.Cluster{
		Name:        name,
		ClusterType: v2.SIMPLE_CLUSTER,
		LbType:      v2.LB_ROUNDROBIN,
		Hosts:       vhosts,
	}
}
