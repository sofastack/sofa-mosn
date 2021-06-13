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

package http

import (
	"testing"

	v2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/mixer/v1/config/client"
)

func TestHandlerReport(t *testing.T) {
	var config v2.Mixer

	config.HttpClientConfig.Transport = &client.TransportConfig{}
	config.HttpClientConfig.Transport.ReportCluster = "test"

	clientContext := NewClientContext(&config)
	serviiceContext := NewServiceContext(clientContext)

	var serviceConfig client.ServiceConfig
	serviiceContext.SetServiceConfig(&serviceConfig)

	handler := NewRequestHandler(serviiceContext)

	reportData := newMockReportData()
	checkData := newMockCheckData()

	mockReportData, _ := reportData.(*mockReportData)
	mockCheckData, _ := checkData.(*mockCheckData)

	handler.Report(checkData, reportData)

	if mockReportData.getResponseHeaders != 1 ||
		mockReportData.getDestinationIPPort != 1 ||
		mockReportData.getReportInfo != 1 {
		t.Errorf("report error")
	}

	if mockReportData.getReportInfo != 1 ||
		mockCheckData.getSourceIPPort != 1 {
		t.Errorf("check error")
	}
}

func TestHandlerDisableReport(t *testing.T) {
	var config v2.Mixer

	config.HttpClientConfig.Transport = &client.TransportConfig{}
	config.HttpClientConfig.Transport.ReportCluster = "test"

	clientContext := NewClientContext(&config)
	serviiceContext := NewServiceContext(clientContext)

	var serviceConfig client.ServiceConfig
	serviceConfig.DisableReportCalls = true
	serviiceContext.SetServiceConfig(&serviceConfig)

	handler := NewRequestHandler(serviiceContext)

	reportData := newMockReportData()
	checkData := newMockCheckData()

	mockReportData, _ := reportData.(*mockReportData)
	mockCheckData, _ := checkData.(*mockCheckData)

	handler.Report(checkData, reportData)

	if mockReportData.getResponseHeaders != 0 ||
		mockReportData.getDestinationIPPort != 0 ||
		mockReportData.getReportInfo != 0 {
		t.Errorf("report error")
	}

	if mockReportData.getReportInfo != 0 ||
		mockCheckData.getSourceIPPort != 0 {
		t.Errorf("check error")
	}
}
