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
	"runtime/debug"
	"sync/atomic"

	"mosn.io/api"
	"mosn.io/pkg/utils"

	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/types"
)

// sessionChecker is a wrapper of types.HealthCheckSession for health check
type sessionChecker struct {
	Session       types.HealthCheckSession
	Host          types.Host
	HealthChecker *healthChecker
	//
	resp          chan checkResponse
	timeout       chan bool
	checkID       uint64
	stop          chan struct{}
	checkTimer    *utils.Timer
	checkTimeout  *utils.Timer
	unHealthCount uint32
	healthCount   uint32
}

type checkResponse struct {
	ID      uint64
	Healthy bool
}

func newChecker(s types.HealthCheckSession, h types.Host, hc *healthChecker) *sessionChecker {
	c := &sessionChecker{
		Session:       s,
		Host:          h,
		HealthChecker: hc,
		resp:          make(chan checkResponse),
		timeout:       make(chan bool),
		stop:          make(chan struct{}),
	}
	return c
}

func (c *sessionChecker) Start() {
	defer func() {
		if r := recover(); r != nil {
			log.DefaultLogger.Alertf("healthcheck.session", "[upstream] [health check] [session checker] panic %v\n%s", r, string(debug.Stack()))
		}
		// stop all the timer when start is finished
		c.checkTimer.Stop()
		c.checkTimeout.Stop()
	}()

	c.checkTimer = utils.NewTimer(c.HealthChecker.initialDelay, c.OnCheck)
	for {
		select {
		case <-c.stop:
			return
		default:
			// prepare a check
			currentID := atomic.AddUint64(&c.checkID, 1)
			select {
			case <-c.stop:
				return
			case resp := <-c.resp:
				// if the ID is not equal, means we receive a timeout for this ID, ignore the response
				if resp.ID == currentID {
					c.checkTimeout.Stop()
					if resp.Healthy {
						c.HandleSuccess()
					} else {
						c.HandleFailure(types.FailureActive)
					}
					// next health checker
					c.checkTimer = utils.NewTimer(c.HealthChecker.getCheckInterval(), c.OnCheck)
					if log.DefaultLogger.GetLogLevel() >= log.DEBUG {
						log.DefaultLogger.Debugf("[upstream] [health check] [session checker] receive a response id: %d", resp.ID)
					}
				} else {
					if log.DefaultLogger.GetLogLevel() >= log.DEBUG {
						log.DefaultLogger.Debugf("[upstream] [health check] [session checker] receive a expired id response, response id: %d, currentID: %d", resp.ID, currentID)
					}
				}
			case <-c.timeout:
				c.checkTimer.Stop()
				c.Session.OnTimeout() // session timeout callbacks
				c.HandleFailure(types.FailureNetwork)
				// next health checker
				c.checkTimer = utils.NewTimer(c.HealthChecker.getCheckInterval(), c.OnCheck)
				if log.DefaultLogger.GetLogLevel() >= log.DEBUG {
					log.DefaultLogger.Debugf("[upstream] [health check] [session checker] receive a timeout response at id: %d", currentID)
				}
			}
		}
	}
}

func (c *sessionChecker) Stop() {
	close(c.stop)
}

func (c *sessionChecker) HandleSuccess() {
	c.unHealthCount = 0
	changed := false
	if c.Host.ContainHealthFlag(api.FAILED_ACTIVE_HC) {
		c.healthCount++
		// check the threshold
		if c.healthCount == c.HealthChecker.healthyThreshold {
			changed = true
			c.Host.ClearHealthFlag(api.FAILED_ACTIVE_HC)
		}
	}
	c.HealthChecker.incHealthy(c.Host, changed)
}

func (c *sessionChecker) HandleFailure(reason types.FailureType) {
	c.healthCount = 0
	changed := false
	if !c.Host.ContainHealthFlag(api.FAILED_ACTIVE_HC) {
		c.unHealthCount++
		// check the threshold
		if c.unHealthCount == c.HealthChecker.unhealthyThreshold {
			changed = true
			c.Host.SetHealthFlag(api.FAILED_ACTIVE_HC)
		}
	}
	c.HealthChecker.decHealthy(c.Host, reason, changed)
}

func (c *sessionChecker) OnCheck() {
	// record current id
	id := atomic.LoadUint64(&c.checkID)
	c.HealthChecker.stats.attempt.Inc(1)
	// start a timeout before check health
	c.checkTimeout.Stop()
	c.checkTimeout = utils.NewTimer(c.HealthChecker.timeout, c.OnTimeout)
	c.resp <- checkResponse{
		ID:      id,
		Healthy: c.Session.CheckHealth(),
	}
}

func (c *sessionChecker) OnTimeout() {
	c.timeout <- true
}
