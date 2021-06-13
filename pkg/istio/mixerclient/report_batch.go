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
	"sync"
	"time"

	v1 "mosn.io/mosn/pkg/mixer/v1"
	"mosn.io/pkg/utils"
)

const (
	// the size of attribute channel
	attributeChannelSize = 1024
)

// Options controlling report batch.
type reportOptions struct {
	// Maximum number of reports to be batched.
	maxBatchEntries int

	// Maximum time a report item stayed in the buffer for batching.
	maxBatchTime time.Duration
}

type reportBatch struct {
	batchCompressor BatchCompressor
	mixerClient     MixerClient
	options         *reportOptions
	batchSize       int
	attributesChan  chan *v1.Attributes
}

func newReportOptions(maxEntries int, maxBatchTimeMs time.Duration) *reportOptions {
	return &reportOptions{
		maxBatchEntries: maxEntries,
		maxBatchTime:    maxBatchTimeMs,
	}
}

func newReportBatch(compressor *AttributeCompressor, options *reportOptions, client MixerClient) *reportBatch {
	batch := &reportBatch{
		batchCompressor: compressor.CreateBatchCompressor(),
		mixerClient:     client,
		options:         options,
		attributesChan:  make(chan *v1.Attributes, attributeChannelSize),
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	utils.GoWithRecover(func() {
		batch.main(&wg)
	}, nil)
	wg.Wait()

	return batch
}

func (r *reportBatch) main(wg *sync.WaitGroup) {
	timer := time.NewTicker(r.options.maxBatchTime).C

	wg.Done()

	for {
		select {
		case attributes := <-r.attributesChan:
			r.batchCompressor.Add(attributes)
			r.batchSize = r.batchCompressor.Size()
			if r.batchSize >= r.options.maxBatchEntries {
				r.flush()
			}
		case <-timer:
			if r.batchSize > 0 {
				r.flush()
			}
		}
	}
}

func (r *reportBatch) report(attributes *v1.Attributes) {
	r.attributesChan <- attributes
}

func (r *reportBatch) flush() {
	request := r.batchCompressor.Finish()
	r.mixerClient.SendReport(request)
	r.batchCompressor.Clear()
	r.batchSize = 0
}
