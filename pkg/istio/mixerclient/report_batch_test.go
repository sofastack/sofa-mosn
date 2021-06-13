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
	"testing"
	"time"

	v1 "mosn.io/mosn/pkg/mixer/v1"
)

func TestDisableBatch(t *testing.T) {
	// max_batch_entries = 0 or 1 to disable batch
	client := newMockMixerClient(1, time.Second*10)
	var report v1.Attributes
	client.Report(&report)

	// sleep wait for channel
	time.Sleep(1 * time.Second)

	mockClient, _ := client.(*mockMixerClient)
	// Expect report transport to be called.
	if mockClient.reportSent != 1 {
		t.Fatalf("fatal: report sent not 1: %d", mockClient.reportSent)
	}
}

func TestBatchReport(t *testing.T) {
	// a long time to disable timeout flush
	client := newMockMixerClient(100, time.Second*100)
	var report v1.Attributes

	for i := 0; i < 10; i++ {
		client.Report(&report)
	}

	// sleep wait for channel
	time.Sleep(1 * time.Second)

	mockClient, _ := client.(*mockMixerClient)
	if mockClient.reportSent != 0 {
		t.Fatalf("fatal: report sent not 0: %d", mockClient.reportSent)
	}
	mockClient.reportBatch.flush()
	if mockClient.reportSent != 1 {
		t.Fatalf("fatal: report sent not 1: %d", mockClient.reportSent)
	}
}

func TestBatchReportWithTimeout(t *testing.T) {
	client := newMockMixerClient(100, time.Second*4)
	var report v1.Attributes

	client.Report(&report)

	// sleep until timeout
	time.Sleep(5 * time.Second)

	mockClient, _ := client.(*mockMixerClient)
	if mockClient.reportSent != 1 {
		t.Fatalf("fatal: report sent not 0: %d", mockClient.reportSent)
	}
}
