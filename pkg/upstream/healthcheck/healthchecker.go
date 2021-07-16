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

package healthcheck

import (
	"math/rand"
	"sync/atomic"
	"time"

	"mosn.io/pkg/utils"

	v2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/types"
)

var firstInterval = time.Second

const (
	DefaultTimeout  = time.Second
	DefaultInterval = 15 * time.Second
)

// TODO: move healthcheck package to cluster package

// healthChecker is a basic implementation of a health checker.
// we use different implementations of types.Session to implement different health checker
type healthChecker struct {
	//
	sessionConfig       map[string]interface{}
	sessionFactory      types.HealthCheckSessionFactory
	checkers            map[string]*sessionChecker
	localProcessHealthy int64
	hosts               []types.Host
	stats               *healthCheckStats
	// check config
	timeout            time.Duration
	intervalBase       time.Duration
	intervalJitter     time.Duration
	initialDelay       time.Duration
	healthyThreshold   uint32
	unhealthyThreshold uint32
	rander             *rand.Rand
	hostCheckCallbacks []types.HealthCheckCb
}

func newHealthChecker(cfg v2.HealthCheck, f types.HealthCheckSessionFactory) types.HealthChecker {
	timeout := DefaultTimeout
	if cfg.Timeout != 0 {
		timeout = cfg.Timeout
	}
	interval := DefaultInterval
	if cfg.Interval != 0 {
		interval = cfg.Interval
	}
	hc := &healthChecker{
		// cfg
		sessionConfig:      cfg.SessionConfig,
		timeout:            timeout,
		intervalBase:       interval,
		intervalJitter:     cfg.IntervalJitter,
		healthyThreshold:   cfg.HealthyThreshold,
		unhealthyThreshold: cfg.UnhealthyThreshold,
		//runtime and stats
		rander:             rand.New(rand.NewSource(time.Now().UnixNano())),
		hostCheckCallbacks: []types.HealthCheckCb{},
		sessionFactory:     f,
		checkers:           make(map[string]*sessionChecker),
		stats:              newHealthCheckStats(cfg.ServiceName),
	}

	if cfg.InitialDelayConfig != nil {
		hc.initialDelay = cfg.InitialDelayConfig.Duration
	} else {
		hc.initialDelay = firstInterval
	}

	// Add common callbacks when create
	// common callbacks should be registered and configured
	for _, name := range cfg.CommonCallbacks {
		v, exists := commonCallbacks.Load(name)
		if exists {
			if cb, ok := v.(types.HealthCheckCb); ok {
				hc.AddHostCheckCompleteCb(cb)
			}

		}
	}
	return hc
}

// only called in cluster, lock in cluster
func (hc *healthChecker) Start() {
	hc.start()
}

func (hc *healthChecker) start() {
	for _, h := range hc.hosts {
		hc.startCheck(h)
	}
	hc.stats.healthy.Update(atomic.LoadInt64(&hc.localProcessHealthy))

}

// only called in cluster, lock in cluster
func (hc *healthChecker) Stop() {
	hc.stop()
}

func (hc *healthChecker) stop() {
	for _, h := range hc.hosts {
		hc.stopCheck(h)
	}
}

func (hc *healthChecker) AddHostCheckCompleteCb(cb types.HealthCheckCb) {
	hc.hostCheckCallbacks = append(hc.hostCheckCallbacks, cb)
}

// only called in cluster, lock in cluster
// SetHealthCheckerHostSet reset the healthchecker's hosts
func (hc *healthChecker) SetHealthCheckerHostSet(hostSet types.HostSet) {
	deleteHosts, newHosts := findNewAndDeleteHost(hc.hosts, hostSet.Hosts())
	for _, newHost := range newHosts {
		hc.startCheck(newHost)
	}
	for _, deleteHost := range deleteHosts {
		hc.stopCheck(deleteHost)
	}
	hc.hosts = hostSet.Hosts()
	hc.stats.healthy.Update(atomic.LoadInt64(&hc.localProcessHealthy))
}

// findNewAndDeleteHost Find deleted and new host in the updated hostSet
func findNewAndDeleteHost(old, new []types.Host) ([]types.Host, []types.Host) {
	newHostsMap := make(map[string]types.Host, len(new))
	for _, newHost := range new {
		newHostsMap[newHost.AddressString()] = newHost
	}
	// find delete host
	deleteHosts := make([]types.Host, 0)
	for _, oldHost := range old {
		_, ok := newHostsMap[oldHost.AddressString()]
		if ok {
			delete(newHostsMap, oldHost.AddressString())
		} else {
			deleteHosts = append(deleteHosts, oldHost)
		}
	}

	// find new host
	newHosts := make([]types.Host, 0, len(newHostsMap))
	for _, newHost := range newHostsMap {
		newHosts = append(newHosts, newHost)
	}

	return deleteHosts, newHosts
}

func (hc *healthChecker) startCheck(host types.Host) {
	addr := host.AddressString()
	if _, ok := hc.checkers[addr]; !ok {
		s := hc.sessionFactory.NewSession(hc.sessionConfig, host)
		if s == nil {
			log.DefaultLogger.Alertf("healthcheck.session", "[upstream] [health check] Create Health Check Session Error, Remote Address = %s", addr)
			return
		}
		c := newChecker(s, host, hc)
		hc.checkers[addr] = c
		utils.GoWithRecover(func() {
			c.Start()
		}, nil)
		atomic.AddInt64(&hc.localProcessHealthy, 1) // default host is healthy
		if log.DefaultLogger.GetLogLevel() >= log.INFO {
			log.DefaultLogger.Infof("[upstream] [health check] create a health check session for %s", addr)
		}
	}
}

func (hc *healthChecker) stopCheck(host types.Host) {
	addr := host.AddressString()
	if c, ok := hc.checkers[addr]; ok {
		c.Stop()
		delete(hc.checkers, addr)
		// hc.localProcessHealthy--
		atomic.AddInt64(&hc.localProcessHealthy, ^int64(0)) // deleted check is unhealthy
		if log.DefaultLogger.GetLogLevel() >= log.INFO {
			log.DefaultLogger.Infof("[upstream] [health check] remove a health check session for %s", addr)
		}
	}
}

func (hc *healthChecker) runCallbacks(host types.Host, changed bool, isHealthy bool) {
	hc.stats.healthy.Update(atomic.LoadInt64(&hc.localProcessHealthy))
	for _, cb := range hc.hostCheckCallbacks {
		cb(host, changed, isHealthy)
	}
}

func (hc *healthChecker) getCheckInterval() time.Duration {
	interval := hc.intervalBase
	if hc.intervalJitter > 0 {
		interval += time.Duration(hc.rander.Int63n(int64(hc.intervalJitter)))
	}
	// TODO: support jitter percentage
	return interval
}

func (hc *healthChecker) incHealthy(host types.Host, changed bool) {
	hc.stats.success.Inc(1)
	if changed {
		if log.DefaultLogger.GetLogLevel() >= log.INFO {
			log.DefaultLogger.Infof("[upstream] [health check] host %s is healthy", host.AddressString())
		}
		atomic.AddInt64(&hc.localProcessHealthy, 1)
	}
	hc.runCallbacks(host, changed, true)
}

func (hc *healthChecker) decHealthy(host types.Host, reason types.FailureType, changed bool) {
	hc.stats.failure.Inc(1)
	if changed {
		// hc.localProcessHealthy--
		if log.DefaultLogger.GetLogLevel() >= log.INFO {
			log.DefaultLogger.Infof("[upstream] [health check] host %s is unhealthy", host.AddressString())
		}
		atomic.AddInt64(&hc.localProcessHealthy, ^int64(0))
	}
	switch reason {
	case types.FailureActive:
		hc.stats.activeFailure.Inc(1)
	case types.FailureNetwork:
		hc.stats.networkFailure.Inc(1)
	case types.FailurePassive: //TODO: not support yet
		hc.stats.passiveFailure.Inc(1)
	}
	hc.runCallbacks(host, changed, false)

}
