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

package streamproxy

import (
	"context"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"

	"mosn.io/api"
	"mosn.io/mosn/pkg/config/v2"
	mosnctx "mosn.io/mosn/pkg/context"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/network"
	"mosn.io/mosn/pkg/types"
	"mosn.io/mosn/pkg/upstream/cluster"
	"mosn.io/pkg/buffer"
)

// ReadFilter
type proxy struct {
	config              ProxyConfig
	clusterManager      types.ClusterManager
	readCallbacks       api.ReadFilterCallbacks
	upstreamConnection  types.ClientConnection
	requestInfo         types.RequestInfo
	upstreamCallbacks   UpstreamCallbacks
	downstreamCallbacks DownstreamCallbacks
	network             string

	upstreamConnecting bool

	accessLogs []api.AccessLog
	ctx        context.Context
}

func NewProxy(ctx context.Context, config *v2.StreamProxy, net string) Proxy {
	p := &proxy{
		config:         NewProxyConfig(config),
		clusterManager: cluster.GetClusterMngAdapterInstance().ClusterManager,
		requestInfo:    network.NewRequestInfo(),
		accessLogs:     mosnctx.Get(ctx, types.ContextKeyAccessLogs).([]api.AccessLog),
		ctx:            ctx,
		network:        net,
	}

	p.upstreamCallbacks = &upstreamCallbacks{
		proxy: p,
	}
	p.downstreamCallbacks = &downstreamCallbacks{
		proxy: p,
	}

	return p
}

func (p *proxy) OnData(buffer buffer.IoBuffer) api.FilterStatus {
	if log.DefaultLogger.GetLogLevel() >= log.DEBUG {
		log.DefaultLogger.Debugf("[%s proxy] [ondata] read data , len = %v", p.network, buffer.Len())
	}
	bytesRecved := p.requestInfo.BytesReceived() + uint64(buffer.Len())
	p.requestInfo.SetBytesReceived(bytesRecved)

	p.upstreamConnection.Write(buffer.Clone())
	buffer.Drain(buffer.Len())
	return api.Stop
}

func (p *proxy) OnNewConnection() api.FilterStatus {
	if log.DefaultLogger.GetLogLevel() >= log.DEBUG {
		log.DefaultLogger.Debugf("[%s proxy] [new conn] accept new connection", p.network)
	}
	return p.initializeUpstreamConnection()
}

func (p *proxy) InitializeReadFilterCallbacks(cb api.ReadFilterCallbacks) {
	p.readCallbacks = cb

	p.readCallbacks.Connection().AddConnectionEventListener(p.downstreamCallbacks)
	p.requestInfo.SetDownstreamRemoteAddress(p.readCallbacks.Connection().RemoteAddr())
	p.requestInfo.SetDownstreamLocalAddress(p.readCallbacks.Connection().LocalAddr())

	p.readCallbacks.Connection().SetReadDisable(true)

	// TODO: set downstream connection stats
}

func (p *proxy) getUpstreamConnection(ctx types.LoadBalancerContext, snapshot types.ClusterSnapshot) types.CreateConnectionData {
	switch p.network {
	case "udp":
		return p.clusterManager.UDPConnForCluster(ctx, snapshot)
	default:
		return p.clusterManager.TCPConnForCluster(ctx, snapshot)
	}
}

func (p *proxy) initializeUpstreamConnection() api.FilterStatus {
	clusterName := p.getUpstreamCluster()

	clusterSnapshot := p.clusterManager.GetClusterSnapshot(context.Background(), clusterName)

	if clusterSnapshot == nil || reflect.ValueOf(clusterSnapshot).IsNil() {
		p.requestInfo.SetResponseFlag(api.NoRouteFound)
		p.onInitFailure(NoRoute)

		return api.Stop
	}

	clusterInfo := clusterSnapshot.ClusterInfo()
	clusterConnectionResource := clusterInfo.ResourceManager().Connections()

	if !clusterConnectionResource.CanCreate() {
		p.requestInfo.SetResponseFlag(api.UpstreamOverflow)
		p.onInitFailure(ResourceLimitExceeded)

		return api.Stop
	}

	ctx := &LbContext{
		conn:    p.readCallbacks,
		ctx:     p.ctx,
		cluster: clusterInfo,
	}
	connectionData := p.getUpstreamConnection(ctx, clusterSnapshot)
	if connectionData.Connection == nil {
		p.requestInfo.SetResponseFlag(api.NoHealthyUpstream)
		p.onInitFailure(NoHealthyUpstream)

		return api.Stop
	}
	p.readCallbacks.SetUpstreamHost(connectionData.Host)
	clusterConnectionResource.Increase()
	upstreamConnection := connectionData.Connection
	upstreamConnection.AddConnectionEventListener(p.upstreamCallbacks)
	upstreamConnection.FilterManager().AddReadFilter(p.upstreamCallbacks)
	p.upstreamConnection = upstreamConnection
	if err := upstreamConnection.Connect(); err != nil {
		log.DefaultLogger.Debugf("%s proxy connect to upstream failed", p.network)
		p.requestInfo.SetResponseFlag(api.NoHealthyUpstream)
		p.onInitFailure(NoHealthyUpstream)
		return api.Stop
	}

	p.requestInfo.OnUpstreamHostSelected(connectionData.Host)
	p.requestInfo.SetUpstreamLocalAddress(connectionData.Host.AddressString())

	// TODO: update upstream stats

	return api.Continue
}

func (p *proxy) closeUpstreamConnection() {
	// TODO: finalize upstream connection stats
	p.upstreamConnection.Close(api.NoFlush, api.LocalClose)
}

func (p *proxy) getUpstreamCluster() string {
	downstreamConnection := p.readCallbacks.Connection()

	return p.config.GetRouteFromEntries(downstreamConnection)
}

func (p *proxy) onInitFailure(reason UpstreamFailureReason) {
	p.readCallbacks.Connection().Close(api.NoFlush, api.LocalClose)
}

func (p *proxy) onUpstreamData(buffer types.IoBuffer) {
	log.DefaultLogger.Tracef("%s Proxy :: read upstream data , len = %v", p.network, buffer.Len())
	bytesSent := p.requestInfo.BytesSent() + uint64(buffer.Len())
	p.requestInfo.SetBytesSent(bytesSent)

	p.readCallbacks.Connection().Write(buffer.Clone())
	buffer.Drain(buffer.Len())
}

func (p *proxy) onUpstreamEvent(event api.ConnectionEvent) {
	switch event {
	case api.RemoteClose:
		p.finalizeUpstreamConnectionStats()
		p.readCallbacks.Connection().Close(api.FlushWrite, api.RemoteClose)

	case api.OnReadErrClose:
		p.finalizeUpstreamConnectionStats()
		p.readCallbacks.Connection().Close(api.NoFlush, api.LocalClose)

	case api.LocalClose:
		log.DefaultLogger.Debugf("upstream close write")

	case api.OnConnect:
	case api.Connected:
		p.readCallbacks.Connection().SetReadDisable(false)
		p.onConnectionSuccess()
	case api.ConnectTimeout:
		p.finalizeUpstreamConnectionStats()

		p.requestInfo.SetResponseFlag(api.UpstreamConnectionFailure)
		p.closeUpstreamConnection()
		p.initializeUpstreamConnection()
	case api.ConnectFailed:
		p.requestInfo.SetResponseFlag(api.UpstreamConnectionFailure)
	}
}

func (p *proxy) finalizeUpstreamConnectionStats() {
	hostInfo := p.readCallbacks.UpstreamHost()
	if host, ok := hostInfo.(types.Host); ok {
		host.ClusterInfo().ResourceManager().Connections().Decrease()
	}
}

func (p *proxy) onConnectionSuccess() {
	// In udp proxy, each upstream connection needs a idle checker
	if p.network == "udp" {
		p.upstreamConnection.SetIdleTimeout(p.config.GetReadTimeout("udp"), p.config.GetIdleTimeout("udp"))
	}
	log.DefaultLogger.Debugf("new upstream connection %d created", p.upstreamConnection.ID())
}

type writeCloser interface {
	CloseWrite(ccType api.ConnectionCloseType, eventType api.ConnectionEvent) (err error)
}

func (p *proxy) onDownstreamEvent(event api.ConnectionEvent) {
	if upstreamConnection := p.upstreamConnection; upstreamConnection != nil {
		if event == api.RemoteClose {
			if closer, ok := upstreamConnection.(writeCloser); ok {
				if err := closer.CloseWrite(api.FlushWrite, api.LocalClose); err != nil {
					log.DefaultLogger.Warnf("close upstream write failed, %s", err)
				}
			} else {
				p.upstreamConnection.Close(api.FlushWrite, api.LocalClose)
			}
		} else if event == api.LocalClose {
			p.upstreamConnection.Close(api.NoFlush, api.LocalClose)
		}
	}
}

func (p *proxy) ReadDisableUpstream(disable bool) {
	// TODO
}

func (p *proxy) ReadDisableDownstream(disable bool) {
	// TODO
}

func (p *proxy) ActiveStreamSize() int {
	// TODO
	return 0
}

type proxyConfig struct {
	statPrefix         string
	cluster            string
	idleTimeout        *time.Duration
	maxConnectAttempts uint32
	routes             []*route
}

type IpRangeList struct {
	cidrRanges []v2.CidrRange
}

func (ipList *IpRangeList) Contains(address net.Addr) bool {
	var ip net.IP
	switch address.Network() {
	case "tcp":
		if tcpAddr, ok := address.(*net.TCPAddr); ok {
			ip = tcpAddr.IP
		}
	case "udp":
		if udpAddr, ok1 := address.(*net.UDPAddr); ok1 {
			ip = udpAddr.IP
		}
	default:
		return false
	}

	log.DefaultLogger.Tracef("IpRangeList check ip = %v,address = %v", ip, address)
	if ip != nil {
		for _, cidrRange := range ipList.cidrRanges {
			log.DefaultLogger.Tracef("check CidrRange = %v,ip = %v", cidrRange, ip)
			if cidrRange.IsInRange(ip) {
				return true
			}
		}
	}
	return false
}

type PortRangeList struct {
	portList []PortRange
}

func (pr *PortRangeList) Contains(address net.Addr) bool {
	var port = 0

	switch address.Network() {
	case "tcp":
		if tcpAddr, ok := address.(*net.TCPAddr); ok {
			port = tcpAddr.Port
		}
	case "udp":
		if udpAddr, ok1 := address.(*net.UDPAddr); ok1 {
			port = udpAddr.Port
		}
	default:
		return false
	}

	if port != 0 {
		log.DefaultLogger.Tracef("PortRangeList check port = %v , address = %v", port, address)
		for _, portRange := range pr.portList {
			log.DefaultLogger.Tracef("check port range , port range = %v , port = %v", portRange, port)
			if port >= portRange.min && port <= portRange.max {
				return true
			}
		}
	}
	return false
}

type PortRange struct {
	min int
	max int
}

func ParsePortRangeList(ports string) PortRangeList {
	var portList []PortRange
	if ports == "" {
		return PortRangeList{portList}
	}
	for _, portItem := range strings.Split(ports, ",") {
		if strings.Contains(portItem, "-") {
			pieces := strings.Split(portItem, "-")
			min, err := strconv.Atoi(pieces[0])
			max, err := strconv.Atoi(pieces[1])
			if err != nil {
				log.DefaultLogger.Errorf("parse port range list fail, invalid port %v", portItem)
				continue
			}
			pRange := PortRange{min: min, max: max}
			portList = append(portList, pRange)
		} else {
			port, err := strconv.Atoi(portItem)
			if err != nil {
				log.DefaultLogger.Errorf("parse port range list fail, invalid port %v", portItem)
				continue
			}
			pRange := PortRange{min: port, max: port}
			portList = append(portList, pRange)
		}
	}
	return PortRangeList{portList}
}

type route struct {
	clusterName      string
	sourceAddrs      IpRangeList
	destinationAddrs IpRangeList
	sourcePort       PortRangeList
	destinationPort  PortRangeList
}

func NewProxyConfig(config *v2.StreamProxy) ProxyConfig {
	var routes []*route

	log.DefaultLogger.Tracef("Stream Proxy :: New Proxy Config = %v", config)
	for _, routeConfig := range config.Routes {
		route := &route{
			clusterName:      routeConfig.Cluster,
			sourceAddrs:      IpRangeList{routeConfig.SourceAddrs},
			destinationAddrs: IpRangeList{routeConfig.DestinationAddrs},
			sourcePort:       ParsePortRangeList(routeConfig.SourcePort),
			destinationPort:  ParsePortRangeList(routeConfig.DestinationPort),
		}
		log.DefaultLogger.Tracef("Stream Proxy add one route : %v", route)

		routes = append(routes, route)
	}

	return &proxyConfig{
		statPrefix:         config.StatPrefix,
		cluster:            config.Cluster,
		idleTimeout:        config.IdleTimeout,
		maxConnectAttempts: config.MaxConnectAttempts,
		routes:             routes,
	}
}

func (pc *proxyConfig) GetIdleTimeout(network string) time.Duration {
	if pc.idleTimeout != nil && *pc.idleTimeout > 0 {
		return *pc.idleTimeout
	}
	if network == "udp" {
		return types.DefaultUDPIdleTimeout
	}
	return types.DefaultIdleTimeout
}

func (pc *proxyConfig) GetReadTimeout(network string) time.Duration {
	switch network {
	case "udp":
		return types.DefaultUDPReadTimeout
	default:
		return types.DefaultConnReadTimeout
	}
}

func (pc *proxyConfig) GetRouteFromEntries(connection api.Connection) string {
	if pc.cluster != "" {
		log.DefaultLogger.Tracef("Stream Proxy get cluster from config , cluster name = %v", pc.cluster)
		return pc.cluster
	}

	log.DefaultLogger.Tracef("Stream Proxy get route from entries , connection = %v", connection)
	for _, r := range pc.routes {
		log.DefaultLogger.Tracef("Stream Proxy check one route = %v", r)
		if !r.sourceAddrs.Contains(connection.RemoteAddr()) {
			continue
		}
		if !r.sourcePort.Contains(connection.RemoteAddr()) {
			continue
		}
		if !r.destinationAddrs.Contains(connection.LocalAddr()) {
			continue
		}
		if !r.destinationPort.Contains(connection.LocalAddr()) {
			continue
		}
		return r.clusterName
	}
	log.DefaultLogger.Warnf("Stream Proxy find no cluster , connection = %v", connection)

	return ""
}

// ConnectionEventListener
// ReadFilter
type upstreamCallbacks struct {
	proxy *proxy
}

func (uc *upstreamCallbacks) OnEvent(event api.ConnectionEvent) {
	switch event {
	case api.Connected:
		uc.proxy.upstreamConnection.SetNoDelay(true)
		uc.proxy.upstreamConnection.SetReadDisable(false)
	}

	uc.proxy.onUpstreamEvent(event)
}

func (uc *upstreamCallbacks) OnData(buffer buffer.IoBuffer) api.FilterStatus {
	uc.proxy.onUpstreamData(buffer)
	return api.Stop
}

func (uc *upstreamCallbacks) OnNewConnection() api.FilterStatus {
	return api.Continue
}

func (uc *upstreamCallbacks) InitializeReadFilterCallbacks(cb api.ReadFilterCallbacks) {}

// ConnectionEventListener
type downstreamCallbacks struct {
	proxy *proxy
}

func (dc *downstreamCallbacks) OnEvent(event api.ConnectionEvent) {
	dc.proxy.onDownstreamEvent(event)
}

// LbContext is a types.LoadBalancerContext implementation
type LbContext struct {
	conn    api.ReadFilterCallbacks
	ctx     context.Context
	cluster types.ClusterInfo
}

func (c *LbContext) MetadataMatchCriteria() api.MetadataMatchCriteria {
	return nil
}

func (c *LbContext) DownstreamConnection() net.Conn {
	return c.conn.Connection().RawConn()
}

// Stream Proxy have no header
func (c *LbContext) DownstreamHeaders() api.HeaderMap {
	return nil
}

func (c *LbContext) DownstreamContext() context.Context {
	return c.ctx
}

func (c *LbContext) DownstreamCluster() types.ClusterInfo {
	return c.cluster
}

func (c *LbContext) DownstreamRoute() api.Route {
	return nil
}
