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
	"fmt"
	"net"
	"sort"
	"sync/atomic"

	"mosn.io/api"
	"mosn.io/mosn/pkg/network"
	"mosn.io/mosn/pkg/types"
)

type mockHostSet struct {
	types.HostSet
	hosts                   []types.Host
	healthCheckVisitedCount int
}

func (hs *mockHostSet) Hosts() []types.Host {
	return hs.hosts
}

func getMockHostSet(count int) *mockHostSet {
	hosts := []types.Host{}
	hostCount := count
	set := &mockHostSet{}

	for i := 0; i < hostCount; i++ {
		h := &mockHost{
			name:    fmt.Sprintf("host-%d", i),
			addr:    fmt.Sprintf("127.0.0.%d", i),
			hostSet: set,
		}
		hosts = append(hosts, h)
	}
	set.hosts = hosts
	return set
}

type mockHost struct {
	name       string
	addr       string
	meta       api.Metadata
	w          uint32
	healthFlag *uint64
	types.Host
	stats   types.HostStats
	hostSet types.HostSet
}

func (h *mockHost) Hostname() string {
	return h.name
}

func (h *mockHost) AddressString() string {
	return h.addr
}

func (h *mockHost) Metadata() api.Metadata {
	return h.meta
}

func (h *mockHost) Health() bool {
	if h.healthFlag == nil {
		h.healthFlag = GetHealthFlagPointer(h.addr)
	}

	// increase hostSet's health check visited count, for testing
	if mhs, ok := h.hostSet.(*mockHostSet); ok {
		mhs.healthCheckVisitedCount++
	}
	return atomic.LoadUint64(h.healthFlag) == 0
}

func (h *mockHost) ClearHealthFlag(flag api.HealthFlag) {
	if h.healthFlag == nil {
		h.healthFlag = GetHealthFlagPointer(h.addr)
	}
	ClearHealthFlag(h.healthFlag, flag)
}

func (h *mockHost) SetHealthFlag(flag api.HealthFlag) {
	if h.healthFlag == nil {
		h.healthFlag = GetHealthFlagPointer(h.addr)
	}
	SetHealthFlag(h.healthFlag, flag)
}

func (h *mockHost) HealthFlag() api.HealthFlag {
	return api.HealthFlag(atomic.LoadUint64(h.healthFlag))
}
func (h *mockHost) HostStats() types.HostStats {
	return h.stats
}

func (h *mockHost) Weight() uint32 {
	return h.w
}

type ipPool struct {
	idx int
	ips []string
}

func (pool *ipPool) Get() string {
	ip := pool.ips[pool.idx]
	pool.idx++
	return ip
}

func (pool *ipPool) MakeHosts(size int, meta api.Metadata) []types.Host {
	hosts := make([]types.Host, size)
	for i := 0; i < size; i++ {
		host := &mockHost{
			addr: pool.Get(),
			meta: meta,
		}
		host.name = host.addr
		host.stats = newHostStats(meta["cluster"], host.addr)
		hosts[i] = host
	}
	return hosts
}

// makePool makes ${size} ips in a ipPool
func makePool(size int) *ipPool {
	var start int64 = 3221291264 // 192.1.1.0:80
	ips := make([]string, size)
	for i := 0; i < size; i++ {
		ip := start + int64(i)
		ips[i] = fmt.Sprintf("%d.%d.%d.%d:80", byte(ip>>24), byte(ip>>16), byte(ip>>8), byte(ip))
	}
	return &ipPool{
		ips: ips,
	}
}

type mockConnPool struct {
	host      atomic.Value
	hashvalue *types.HashValue
	types.ConnectionPool
}

const mockProtocol = types.ProtocolName("mock")

func (p *mockConnPool) Protocol() types.ProtocolName {
	return mockProtocol
}

func (p *mockConnPool) CheckAndInit(ctx context.Context) bool {
	return true
}

func (p *mockConnPool) TLSHashValue() *types.HashValue {
	return p.hashvalue
}

func (p *mockConnPool) Shutdown() {
}

func (p *mockConnPool) Close() {
}

func (p *mockConnPool) NewStream(ctx context.Context, receiver types.StreamReceiveListener) (types.Host, types.StreamSender, types.PoolFailureReason) {
	return p.Host(), nil, ""
}

func (p *mockConnPool) Host() types.Host {
	h := p.host.Load()
	if host, ok := h.(types.Host); ok {
		return host
	}

	return nil
}

func (p *mockConnPool) UpdateHost(h types.Host) {
	p.host.Store(h)
}

func init() {
	network.RegisterNewPoolFactory(mockProtocol, func(ctx context.Context, h types.Host) types.ConnectionPool {
		pool := &mockConnPool{
			hashvalue: h.TLSHashValue(),
		}
		pool.host.Store(h)
		return pool
	})
	types.RegisterConnPoolFactory(mockProtocol, true)
}

type mockLbContext struct {
	types.LoadBalancerContext
	mmc     api.MetadataMatchCriteria
	header  api.HeaderMap
	context context.Context
	route   api.Route
}
type mockConn struct {
	net.Conn
}

// NewMetadataMatchCriteriaImpl
func NewMockMetadataMatchCriteriaImpl(metadataMatches map[string]string) *MockMetadataMatchCriteriaImpl {

	metadataMatchCriteriaImpl := &MockMetadataMatchCriteriaImpl{}
	metadataMatchCriteriaImpl.extractMetadataMatchCriteria(nil, metadataMatches)

	return metadataMatchCriteriaImpl
}

// MetadataMatchCriteriaImpl class wrapper MatchCriteriaArray
// which contains MatchCriteria in dictionary sorted
type MockMetadataMatchCriteriaImpl struct {
	MatchCriteriaArray []api.MetadataMatchCriterion
}

// MetadataMatchCriteria
func (mmcti *MockMetadataMatchCriteriaImpl) MetadataMatchCriteria() []api.MetadataMatchCriterion {
	return mmcti.MatchCriteriaArray
}

func (mmcti *MockMetadataMatchCriteriaImpl) Len() int {
	return len(mmcti.MatchCriteriaArray)
}

func (mmcti *MockMetadataMatchCriteriaImpl) Less(i, j int) bool {
	return mmcti.MatchCriteriaArray[i].MetadataKeyName() < mmcti.MatchCriteriaArray[j].MetadataKeyName()
}

func (mmcti *MockMetadataMatchCriteriaImpl) Swap(i, j int) {
	mmcti.MatchCriteriaArray[i], mmcti.MatchCriteriaArray[j] = mmcti.MatchCriteriaArray[j],
		mmcti.MatchCriteriaArray[i]
}

// Used to generate metadata match criteria from config
func (mmcti *MockMetadataMatchCriteriaImpl) extractMetadataMatchCriteria(parent *MockMetadataMatchCriteriaImpl,
	metadataMatches map[string]string) {

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
		mmci := &MockMetadataMatchCriterionImpl{
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

// MergeMatchCriteria
// No usage currently
func (mmcti *MockMetadataMatchCriteriaImpl) MergeMatchCriteria(metadataMatches map[string]string) api.MetadataMatchCriteria {
	return nil
}

// MetadataMatchCriterionImpl class contains the name and value of the metadata match criterion
// Implement types.MetadataMatchCriterion
type MockMetadataMatchCriterionImpl struct {
	Name  string
	Value string
}

// MetadataKeyName return name
func (mmci *MockMetadataMatchCriterionImpl) MetadataKeyName() string {
	return mmci.Name
}

// MetadataValue return value
func (mmci *MockMetadataMatchCriterionImpl) MetadataValue() string {
	return mmci.Value
}

func newMockLbContext(m map[string]string) types.LoadBalancerContext {
	var mmc api.MetadataMatchCriteria
	if m != nil {
		mmc = NewMockMetadataMatchCriteriaImpl(m)
	}
	return &mockLbContext{
		mmc: mmc,
	}
}

func newMockLbContextWithHeader(m map[string]string, header types.HeaderMap) types.LoadBalancerContext {
	mmc := NewMockMetadataMatchCriteriaImpl(m)
	return &mockLbContext{
		mmc:    mmc,
		header: header,
	}
}

func newMockLbContextWithCtx(m map[string]string, ctx context.Context) types.LoadBalancerContext {
	mmc := NewMockMetadataMatchCriteriaImpl(m)
	return &mockLbContext{
		mmc:     mmc,
		context: ctx,
	}
}

func (ctx *mockLbContext) MetadataMatchCriteria() api.MetadataMatchCriteria {
	return ctx.mmc
}
func (ctx *mockLbContext) DownstreamHeaders() types.HeaderMap {
	return ctx.header
}
func (ctx *mockLbContext) DownstreamContext() context.Context {
	return ctx.context
}
func (ctx *mockLbContext) DownstreamConnection() net.Conn {
	return &mockConn{}
}

func (ctx *mockLbContext) DownstreamRoute() api.Route {
	return ctx.route
}

func (mc *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.IP([]byte{192, 168, 0, 100}),
		Port: 8080,
		Zone: "",
	}
}

type mockClusterInfo struct {
	name string
	types.ClusterInfo
}

func (ci *mockClusterInfo) Name() string {
	return ci.name
}

type mockRoute struct {
	api.Route
	routeRule api.RouteRule
}

func (mr *mockRoute) RouteRule() api.RouteRule {
	return mr.routeRule
}

type mockRouteRule struct {
	api.RouteRule
	policy api.Policy
}

func (mp *mockRouteRule) Policy() api.Policy {
	return mp.policy
}

type mockPolicy struct {
	api.Policy
	hashPolicy api.HashPolicy
}

func (mp *mockPolicy) HashPolicy() api.HashPolicy {
	return mp.hashPolicy
}

type mockHashPolicy struct {
	api.HashPolicy
}

func (mhp *mockHashPolicy) GenerateHash(context context.Context) uint64 {
	return 0
}
