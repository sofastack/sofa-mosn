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

package mosn

import (
	"syscall"

	admin "mosn.io/mosn/pkg/admin/server"
	"mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/configmanager"
	"mosn.io/mosn/pkg/featuregate"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/server/keeper"
	"mosn.io/mosn/pkg/types"
)

// Default Init Stage wrappers. if more initialize needs to extend.
// modify it in main function
func DefaultInitStage(c *v2.MOSNConfig) {
	types.InitDefaultPath(configmanager.GetConfigPath())
	types.InitXdsFromMOSNConfig(c, log.StartLogger.Fatalf)
	InitDebugServe(c)
	InitializePidFile(c)
	InitializeTracing(c)
	InitializePlugin(c)
	InitializeWasm(c)
	InitializeThirdPartCodec(c)
	InitializeMetrics(c)
}

// Default Pre-start Stage wrappers
func DefaultPreStartStage(m *Mosn) {
	// the signals SIGKILL and SIGSTOP may not be caught by a program,
	// so we need other ways to ensure that resources are safely cleaned up
	keeper.AddSignalCallback(func() {
		log.DefaultLogger.Infof("[mosn] [close] mosn closed by sys signal")
		m.Close()
	}, syscall.SIGINT, syscall.SIGTERM)
	// start xds client
	m.StartXdsClient()
	featuregate.FinallyInitFunc()
	m.HandleExtendConfig()
}

// Default Start Stage wrappers
func DefaultStartStage(m *Mosn) {
	// register admin server
	// admin server should registered after all prepares action ready
	srv := admin.Server{}
	srv.Start(m.Config)
	//  transfer connection used in smooth upgrade in mosn
	m.TransferConnection()
	// clean upgrade finish the smooth upgrade datas
	m.CleanUpgrade()
}
