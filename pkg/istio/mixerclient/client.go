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
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"mosn.io/mosn/pkg/log"
	v1 "mosn.io/mosn/pkg/mixer/v1"
	"mosn.io/mosn/pkg/upstream/cluster"
)

const (
	queryTimeout = time.Second * 2

	// flush time out
	flushTimeout = time.Second * 1

	// max entries before flush
	flushMaxEntries = 100
)

// MixerClient for communicate with mixer server
type MixerClient interface {
	// Report RPC
	Report(attributes *v1.Attributes)

	// SendReport
	SendReport(request *v1.ReportRequest) *v1.ReportResponse
}

type mixerClient struct {
	reportBatch         *reportBatch
	attributeCompressor *AttributeCompressor
	client              v1.MixerClient
	mixerAddress        string
	lastConnectTime     time.Time
	reportCluster       string
}

// NewMixerClient return MixerClient
func NewMixerClient(reportCluster string) MixerClient {
	attributeCompressor := NewAttributeCompressor()
	client := &mixerClient{
		attributeCompressor: attributeCompressor,
		reportCluster:       reportCluster,
		lastConnectTime:     time.Now(),
	}
	client.reportBatch = newReportBatch(client.attributeCompressor, newReportOptions(flushMaxEntries, flushTimeout), client)

	client.tryConnect(false)

	return client
}

func (c *mixerClient) tryConnect(retry bool) error {
	if retry {
		now := time.Now()
		diff := now.Sub(c.lastConnectTime)
		if diff < time.Second*10 {
			return fmt.Errorf("re-connect too often")
		}
		// update last connect time
		c.lastConnectTime = now
	}

	mngAdaper := cluster.GetClusterMngAdapterInstance()
	if mngAdaper == nil {
		return fmt.Errorf("mng adapter nil")
	}
	snapshot := mngAdaper.GetClusterSnapshot(context.Background(), c.reportCluster)
	if snapshot == nil {
		err := fmt.Errorf("get mixer server cluster config error, report cluster: %s", c.reportCluster)
		log.DefaultLogger.Errorf("%s", err.Error())
		return err
	}

	hosts := snapshot.HostSet().Hosts()
	if len(hosts) == 0 {
		return fmt.Errorf("no hosts for reportCluster %s", c.reportCluster)
	}

	// TODO: use lb
	mixerAddress := hosts[0].AddressString()
	conn, err := grpc.Dial(mixerAddress, grpc.WithInsecure())
	if err != nil {
		err := fmt.Errorf("grpc dial to mixer server %s error %v", mixerAddress, err)
		log.DefaultLogger.Errorf("%s", err.Error())
		return err
	}
	c.client = v1.NewMixerClient(conn)
	c.mixerAddress = mixerAddress
	return nil
}

// Report RPC call
func (c *mixerClient) Report(attributes *v1.Attributes) {
	if c.client == nil {
		err := c.tryConnect(true)
		if err != nil {
			log.DefaultLogger.Errorf("mixer client nil, retry error:%v")
			return
		}
	}

	c.reportBatch.report(attributes)
}

// SendReport send report request
func (c *mixerClient) SendReport(request *v1.ReportRequest) *v1.ReportResponse {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	response, err := c.client.Report(ctx, request)
	defer cancel()
	if err != nil {
		log.DefaultLogger.Errorf("send report error: %v, stack: %s\n\n", err, string(debug.Stack()))
	}
	return response
}
