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

package server

import (
	"os"
	"time"

	v2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/configmanager"
	mlog "mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/network"
	"mosn.io/mosn/pkg/types"
	"mosn.io/pkg/buffer"
	"mosn.io/pkg/log"
)

// GetServer currently, only one server supported
func GetServer() Server {
	if len(servers) == 0 {
		log.DefaultLogger.Errorf("[server] Server is nil and hasn't been initiated at this time")
		return nil
	}

	return servers[0]
}

var servers []*server

type server struct {
	serverName string
	stopChan   chan struct{}
	handler    types.ConnectionHandler
}

// NewConfig get Config by ServerConfig
func NewConfig(c *v2.ServerConfig) *Config {
	return &Config{
		ServerName:      c.ServerName,
		LogPath:         c.DefaultLogPath,
		LogLevel:        configmanager.ParseLogLevel(c.DefaultLogLevel),
		LogRoller:       c.GlobalLogRoller,
		GracefulTimeout: c.GracefulTimeout.Duration,
		UseNetpollMode:  c.UseNetpollMode,
	}
}

// NewServer get a new server
func NewServer(config *Config, cmFilter types.ClusterManagerFilter, clMng types.ClusterManager) Server {
	if config != nil {
		//graceful timeout setting
		if config.GracefulTimeout != 0 {
			GracefulTimeout = config.GracefulTimeout
		}

		if config.UseNetpollMode {
			network.UseNetpollMode = config.UseNetpollMode
			log.DefaultLogger.Infof("[server] [reconfigure] [new server] Netpoll mode enabled.")
		}
	}

	server := &server{
		serverName: config.ServerName,
		stopChan:   make(chan struct{}),
		handler:    NewHandler(cmFilter, clMng),
	}

	initListenerAdapterInstance(server.serverName, server.handler)

	servers = append(servers, server)

	return server
}

// add a listener to server handler
func (srv *server) AddListener(lc *v2.Listener) (types.ListenerEventListener, error) {

	return srv.handler.AddOrUpdateListener(lc)
}

// Start the server
func (srv *server) Start() {
	// TODO: handle main thread panic @wugou

	srv.handler.StartListeners(nil)

	<-srv.stopChan
}

// Restart the server
func (srv *server) Restart() {
	// TODO
}

// Close the server
func (srv *server) Close() {
	// stop listener and connections
	srv.handler.StopListeners(nil, true)

	close(srv.stopChan)
}

// Handler get server's handler
func (srv *server) Handler() types.ConnectionHandler {
	return srv.handler
}

// Stop the server
func Stop() {
	for _, server := range servers {
		server.Close()
	}
}

// StopAccept stops all listeners in servers
func StopAccept() {
	for _, server := range servers {
		server.handler.StopListeners(nil, false)
	}
}

// StopConnection stops all connections in servers
func StopConnection() {
	for _, server := range servers {
		server.handler.StopConnection()
	}
}

// ListListenersFile returns all server listener's fds
func ListListenersFile() []*os.File {
	var files []*os.File
	for _, server := range servers {
		files = append(files, server.handler.ListListenersFile(nil)...)
	}
	return files
}

// WaitConnectionsDone Wait for all connections to be finished
func WaitConnectionsDone(duration time.Duration) {
	// one duration wait for connection to active close
	// two duration wait for connection to transfer
	// DefaultConnReadTimeout wait for read timeout
	timeout := time.NewTimer(2*duration + 2*buffer.ConnReadTimeout)
	StopConnection()
	log.DefaultLogger.Infof("[server] StopConnection")

	<-timeout.C
}

// InitDefaultLogger init default logger
func InitDefaultLogger(config *Config) {

	var logPath string
	var logLevel log.Level

	if config != nil {
		logPath = config.LogPath
		logLevel = config.LogLevel
	}

	//use default log path
	if logPath == "" {
		logPath = types.MosnLogDefaultPath
	}

	if config.LogRoller != "" {
		err := log.InitGlobalRoller(config.LogRoller)
		if err != nil {
			log.DefaultLogger.Fatalf("[server] [init] initialize default logger Roller failed : %v", err)
		}
	}

	err := mlog.InitDefaultLogger(logPath, logLevel)
	if err != nil {
		mlog.StartLogger.Fatalf("[server] [init] initialize default logger failed : %v", err)
	}
}
