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

package mixerclient

import (
	"runtime"
	"sync"

	v1 "mosn.io/mosn/pkg/mixer/v1"
	"mosn.io/pkg/utils"
)

const (
	reportChannelSize = 1024
)

type mixerClientArrayInfo struct {
	clients []MixerClient
	index   int32
}

type reportInfo struct {
	reportCluster string
	attributes    *v1.Attributes
}

type mixerClientManager struct {
	clientMap      map[string]*mixerClientArrayInfo
	cpuNum         int32
	reportInfoChan chan *reportInfo
}

var gMixerClientManager *mixerClientManager

func init() {
	gMixerClientManager = newMixerClientManager()
}

func newMixerClientManager() *mixerClientManager {
	mg := &mixerClientManager{
		clientMap:      make(map[string]*mixerClientArrayInfo, 0),
		cpuNum:         int32(runtime.NumCPU()),
		reportInfoChan: make(chan *reportInfo, reportChannelSize),
	}

	var wg sync.WaitGroup
	wg.Add(1)
	utils.GoWithRecover(func() {
		mg.mainLoop(&wg)
	}, nil)
	wg.Wait()

	return mg
}

func (m *mixerClientManager) mainLoop(wg *sync.WaitGroup) {
	wg.Done()

	for {
		select {
		case reportInfo := <-m.reportInfoChan:
			m.doReport(reportInfo.reportCluster, reportInfo.attributes)
		}
	}
}

func (m *mixerClientManager) newClientArray(reportCluster string) *mixerClientArrayInfo {
	clientArray := &mixerClientArrayInfo{
		clients: make([]MixerClient, 0),
		index:   0,
	}

	var i int32
	for i = 0; i < m.cpuNum; i++ {
		clientArray.clients = append(clientArray.clients, NewMixerClient(reportCluster))
	}

	return clientArray
}

func (m *mixerClientManager) doReport(reportCluster string, attributes *v1.Attributes) {
	clientArray, exist := m.clientMap[reportCluster]
	if !exist {
		clientArray = m.newClientArray(reportCluster)
		m.clientMap[reportCluster] = clientArray
	}

	index := clientArray.index
	clientArray.index = (clientArray.index + 1) % m.cpuNum
	clientArray.clients[index].Report(attributes)
}

func (m *mixerClientManager) report(reportCluster string, attributes *v1.Attributes) {
	m.reportInfoChan <- &reportInfo{
		reportCluster: reportCluster,
		attributes:    attributes,
	}
}

// Report attributes to mixer server
func Report(reportCluster string, attributes *v1.Attributes) {
	gMixerClientManager.report(reportCluster, attributes)
}
