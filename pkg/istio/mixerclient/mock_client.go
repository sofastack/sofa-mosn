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
	"time"

	v1 "mosn.io/mosn/pkg/mixer/v1"
)

type mockMixerClient struct {
	reportSent          int
	reportBatch         *reportBatch
	attributeCompressor *AttributeCompressor
}

func newMockMixerClient(maxEntries int, maxBatchTimeMs time.Duration) MixerClient {
	attributeCompressor := NewAttributeCompressor()
	c := &mockMixerClient{
		attributeCompressor: attributeCompressor,
	}
	c.reportBatch = newReportBatch(c.attributeCompressor, newReportOptions(maxEntries, maxBatchTimeMs), c)
	return c
}

// Report RPC call
func (c *mockMixerClient) Report(attributes *v1.Attributes) {
	c.reportBatch.report(attributes)
}

// SendReport send report request
func (c *mockMixerClient) SendReport(request *v1.ReportRequest) *v1.ReportResponse {
	c.reportSent++
	return &v1.ReportResponse{}
}
