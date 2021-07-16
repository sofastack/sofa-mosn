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
	"errors"
	"fmt"
	"net"
	"time"

	"go.uber.org/atomic"
	"mosn.io/api"
	"mosn.io/mosn/pkg/filter/network/tunnel/ext"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/types"
	"mosn.io/pkg/buffer"
	"mosn.io/pkg/utils"
)

var (
	defaultReconnectBaseDuration  = time.Second * 3
	defaultConnectTimeoutDuration = time.Second * 15
	defaultConnectMaxRetryTimes   = -1
)

type AgentRawConnection struct {
	ConnectionConfig
	readBuffer buffer.IoBuffer
	rawc       net.Conn
	listener   types.Listener
	close      *atomic.Bool
	closeChan  chan struct{}
	initInfo   *ConnectionInitInfo
}

func NewConnection(config ConnectionConfig, listener types.Listener) *AgentRawConnection {
	if config.Network == "" {
		config.Network = "tcp"
	}
	if config.ReconnectBaseDuration == 0 {
		config.ReconnectBaseDuration = defaultReconnectBaseDuration
	}
	if config.ConnectRetryTimes == 0 {
		config.ConnectRetryTimes = defaultConnectMaxRetryTimes
	}
	if config.ConnectTimeoutDuration == 0 {
		config.ConnectTimeoutDuration = defaultConnectTimeoutDuration
	}

	initInfo := &ConnectionInitInfo{
		ClusterName:      config.ClusterName,
		Weight:           config.Weight,
		CredentialPolicy: config.CredentialPolicy,
	}

	if config.CredentialPolicy != "" {
		credentialGetter := ext.GetConnectionCredentialGetter(config.CredentialPolicy)
		if credentialGetter == nil {
			log.DefaultLogger.Fatalf("[agent] credential %v getter not found", config.CredentialPolicy)
		}
		initInfo.Credential = credentialGetter(config.ClusterName)
	}

	return &AgentRawConnection{
		ConnectionConfig: config,
		listener:         listener,
		initInfo:         initInfo,
		readBuffer:       buffer.GetIoBuffer(1024),
		close:            atomic.NewBool(false),
	}
}

func (a *AgentRawConnection) Stop() error {
	if a.close.CAS(false, true) {
		close(a.closeChan)
		if a.rawc == nil {
			return nil
		}
		return a.rawc.Close()
	}
	return nil
}

func (a *AgentRawConnection) doConnect() (net.Conn, error) {
	rawc, err := net.DialTimeout(a.Network, a.Address, a.ConnectTimeoutDuration)
	if err != nil {
		return nil, err
	}
	rawc.SetReadDeadline(time.Now().Add(a.ConnectTimeoutDuration))
	b, err := Encode(a.initInfo)
	if err != nil {
		return nil, err
	}
	// Write connection init request
	_, err = rawc.Write(b.Bytes())
	for {
		select {
		case <-a.closeChan:
			return nil, errors.New("agent connection closed")
		default:
			_, err = a.readBuffer.ReadOnce(rawc)
			// Timout or EOF
			if err != nil {
				log.DefaultLogger.Errorf("[agent] read response failed, err: %+v", a.Address, err)
				a.Stop()
				return nil, err
			}
			ret, err := DecodeFromBuffer(a.readBuffer)
			if err != nil {
				log.DefaultLogger.Warnf("[agent] decode from buffer failed, err: %+v", err)
				return nil, err
			}
			// Data is not enough, continue to read
			if ret == nil {
				continue
			}
			resp := ret.(*ConnectionInitResponse)
			if resp.Status != ConnectSuccess {
				// Reconnect and write again
				log.DefaultLogger.Errorf("[agent] failed to write connection info to remote server, address: %v, status: %v", a.Address, resp.Status)
				// Close connection and reconnect again
				return nil, a.Stop()
			}
			return rawc, nil
		}
	}
}
func (a *AgentRawConnection) connectAndInit() error {
	var rawc net.Conn
	var err error
	backoffConnectDuration := a.ReconnectBaseDuration

	for i := 0; i < a.ConnectRetryTimes || a.ConnectRetryTimes == -1; i++ {
		if a.close.Load() {
			return fmt.Errorf("connection closed, don't attempt to connect, address: %v", a.ConnectionConfig.Address)
		}
		rawc, err = a.doConnect()
		if err == nil {
			a.rawc = rawc
			break
		}
		log.DefaultLogger.Errorf("[agent] failed to connect remote server, try again after %v seconds, address: %v, err: %+v", backoffConnectDuration, a.Address, err)
		time.Sleep(backoffConnectDuration)
		backoffConnectDuration *= 2
	}
	if err != nil {
		return err
	}
	// Hosting new connection
	utils.GoWithRecover(func() {
		ch := make(chan api.Connection, 1)
		a.listener.GetListenerCallbacks().OnAccept(rawc, a.listener.UseOriginalDst(), nil, ch, a.readBuffer.Bytes(), []api.ConnectionEventListener{a})
	}, nil)

	return nil
}

func (a *AgentRawConnection) OnEvent(event api.ConnectionEvent) {
	switch {
	case event.IsClose(), event.ConnectFailure():
		break
	default:
		if log.DefaultLogger.GetLogLevel() >= log.DEBUG {
			log.DefaultLogger.Debugf("[agent] receive %s event, ignore it", event)
		}
		return
	}

	utils.GoWithRecover(func() {
		log.DefaultLogger.Infof("[agent] receive reconnect event, and try to reconnect remote server %v", a.Address)
		err := a.connectAndInit()
		if err != nil {
			log.DefaultLogger.Errorf("[agent] failed to reconnect remote server: %v", a.Address)
			return
		}
		log.DefaultLogger.Infof("[agent] reconnect remote server: %v success", a.Address)
	}, nil)
}
