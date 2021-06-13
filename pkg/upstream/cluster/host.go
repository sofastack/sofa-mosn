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
	"context"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"mosn.io/api"
	v2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/network"
	"mosn.io/mosn/pkg/types"
	"mosn.io/pkg/utils"
)

// simpleHost is an implement of types.Host and types.HostInfo
type simpleHost struct {
	hostname      string
	addressString string
	clusterInfo   atomic.Value // store types.ClusterInfo
	stats         types.HostStats
	metaData      api.Metadata
	tlsDisable    bool
	weight        uint32
	healthFlags   *uint64
}

func NewSimpleHost(config v2.Host, clusterInfo types.ClusterInfo) types.Host {
	// clusterInfo should not be nil
	// pre resolve address
	GetOrCreateAddr(config.Address)
	h := &simpleHost{
		hostname:      config.Hostname,
		addressString: config.Address,
		stats:         newHostStats(clusterInfo.Name(), config.Address),
		metaData:      config.MetaData,
		tlsDisable:    config.TLSDisable,
		weight:        config.Weight,
		healthFlags:   GetHealthFlagPointer(config.Address),
	}
	h.clusterInfo.Store(clusterInfo)
	return h
}

// types.HostInfo Implement
func (sh *simpleHost) Hostname() string {
	return sh.hostname
}

func (sh *simpleHost) Metadata() api.Metadata {
	return sh.metaData
}

func (sh *simpleHost) ClusterInfo() types.ClusterInfo {
	v := sh.clusterInfo.Load()
	info, _ := v.(types.ClusterInfo)
	return info

}

func (sh *simpleHost) SetClusterInfo(info types.ClusterInfo) {
	sh.clusterInfo.Store(info)
}

func (sh *simpleHost) Address() net.Addr {
	return GetOrCreateAddr(sh.addressString)
}

func (sh *simpleHost) UDPAddress() net.Addr {
	return GetOrCreateUDPAddr(sh.addressString)
}

func (sh *simpleHost) AddressString() string {
	return sh.addressString
}

func (sh *simpleHost) HostStats() types.HostStats {
	return sh.stats
}

func (sh *simpleHost) Weight() uint32 {
	return sh.weight
}

func (sh *simpleHost) Config() v2.Host {
	return v2.Host{
		HostConfig: v2.HostConfig{
			Address:    sh.addressString,
			Hostname:   sh.hostname,
			TLSDisable: sh.tlsDisable,
			Weight:     sh.weight,
		},
		MetaData: sh.metaData,
	}
}

func (sh *simpleHost) SupportTLS() bool {
	return IsSupportTLS() && !sh.tlsDisable && sh.ClusterInfo().TLSMng().Enabled()
}

func (sh *simpleHost) TLSHashValue() *types.HashValue {
	// check tls_disable config
	if sh.tlsDisable || !sh.ClusterInfo().TLSMng().Enabled() {
		return disableTLSHashValue
	}
	// check global tls
	if !IsSupportTLS() {
		return clientSideDisableHashValue
	}
	return sh.ClusterInfo().TLSMng().HashValue()
}

// types.Host Implement
func (sh *simpleHost) CreateConnection(context context.Context) types.CreateConnectionData {
	var tlsMng types.TLSClientContextManager
	if sh.SupportTLS() {
		tlsMng = sh.ClusterInfo().TLSMng()
	}
	if log.DefaultLogger.GetLogLevel() >= log.DEBUG {
		log.DefaultLogger.Debugf("[simpleHost] tlsMng: %v", tlsMng)
	}
	clientConn := network.NewClientConnection(sh.ClusterInfo().ConnectTimeout(), tlsMng, sh.Address(), nil)
	clientConn.SetBufferLimit(sh.ClusterInfo().ConnBufferLimitBytes())

	return types.CreateConnectionData{
		Connection: clientConn,
		Host:       sh,
	}
}

func (sh *simpleHost) CreateUDPConnection(context context.Context) types.CreateConnectionData {
	clientConn := network.NewClientConnection(sh.ClusterInfo().ConnectTimeout(), nil, sh.UDPAddress(), nil)
	clientConn.SetBufferLimit(sh.ClusterInfo().ConnBufferLimitBytes())

	return types.CreateConnectionData{
		Connection: clientConn,
		Host:       sh,
	}
}

func (sh *simpleHost) ClearHealthFlag(flag api.HealthFlag) {
	ClearHealthFlag(sh.healthFlags, flag)
}

func (sh *simpleHost) ContainHealthFlag(flag api.HealthFlag) bool {
	return atomic.LoadUint64(sh.healthFlags)&uint64(flag) > 0
}

func (sh *simpleHost) SetHealthFlag(flag api.HealthFlag) {
	SetHealthFlag(sh.healthFlags, flag)
}

func (sh *simpleHost) HealthFlag() api.HealthFlag {
	return api.HealthFlag(atomic.LoadUint64(sh.healthFlags))
}

func (sh *simpleHost) Health() bool {
	return atomic.LoadUint64(sh.healthFlags) == 0
}

// net.Addr reuse for same address, valid in simple type
// Update DNS cache using asynchronous mode
var AddrStore *utils.ExpiredMap = utils.NewExpiredMap(
	func(key interface{}) (interface{}, bool) {
		addr, err := net.ResolveTCPAddr("tcp", key.(string))
		if err == nil {
			return addr, true
		}
		return nil, false
	}, false)

func GetOrCreateAddr(addrstr string) net.Addr {

	var addr net.Addr
	var err error

	// Check DNS cache
	if r, _ := AddrStore.Get(addrstr); r != nil {
		switch v := r.(type) {
		case net.Addr:
			return v
		case error:
			return nil
		}
	}

	// resolve addr
	if addr, err = net.ResolveTCPAddr("tcp", addrstr); err != nil {
		// try to resolve addr by unix
		// as a UNIX-domain socket path specified after the “unix:” prefix.
		if strings.HasSuffix(addrstr, "unix:") {
			addr, err = net.ResolveUnixAddr("unix", addrstr)
			if err != nil {
				err = errors.New("failed to resolve address in tcp and unix model")
			}
		}
	}

	if err != nil {
		// If a DNS query fails then don't sent to DNS within 15 seconds and avoid flood
		AddrStore.Set(addrstr, err, 15*time.Second)
		log.DefaultLogger.Errorf("[upstream] resolve addr %s failed: %v", addrstr, err)
		return nil
	}

	// Save DNS cache
	// Unix Domain Socket always satisfies `addr.String() == addrstr`
	if addr.String() != addrstr {
		// TODO support config or depends on DNS TTL for expire time
		// now set default expire time == 15 s, Means that after 15 seconds, the new request will trigger domain resolve.
		AddrStore.Set(addrstr, addr, 15*time.Second)
	} else {
		// if addrsstr isn't domain and don't set expire time
		AddrStore.Set(addrstr, addr, utils.NeverExpire)
	}

	return addr
}

// store resolved UDP addr
var UDPAddrStore *utils.ExpiredMap = utils.NewExpiredMap(
	func(key interface{}) (interface{}, bool) {
		addr, err := net.ResolveUDPAddr("udp", key.(string))
		if err == nil {
			return addr, true
		}
		return nil, false
	}, false)

func GetOrCreateUDPAddr(addrstr string) net.Addr {
	var addr net.Addr
	var err error

	// Check DNS cache
	if r, _ := UDPAddrStore.Get(addrstr); r != nil {
		switch v := r.(type) {
		case net.Addr:
			return v
		case error:
			return nil
		}
	}

	addr, err = net.ResolveUDPAddr("udp", addrstr)
	if err != nil {
		// If a DNS query fails then don't sent to DNS within 15 seconds and avoid flood
		UDPAddrStore.Set(addrstr, err, 15*time.Second)
		log.DefaultLogger.Errorf("[upstream] resolve addr %s failed: %v", addrstr, err)
		return nil
	}

	if addr.String() != addrstr {
		// now set default expire time == 15 s, Means that after 15 seconds, the new request will trigger domain resolve.
		UDPAddrStore.Set(addrstr, addr, 15*time.Second)
	} else {
		// if addrsstr isn't domain and don't set expire time
		UDPAddrStore.Set(addrstr, addr, utils.NeverExpire)
	}

	return addr
}
