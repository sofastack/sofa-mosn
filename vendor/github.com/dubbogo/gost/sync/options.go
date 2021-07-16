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

package gxsync

import (
	"fmt"
)

const (
	defaultTaskQNumber = 10
	defaultTaskQLen    = 128
)

/////////////////////////////////////////
// Task Pool Options
/////////////////////////////////////////

// TaskPoolOptions is optional settings for task pool
type TaskPoolOptions struct {
	tQLen      int // task queue length. buffer size per queue
	tQNumber   int // task queue number. number of queue
	tQPoolSize int // task pool size. number of workers
}

func (o *TaskPoolOptions) validate() {
	if o.tQPoolSize < 1 {
		panic(fmt.Sprintf("illegal pool size %d", o.tQPoolSize))
	}

	if o.tQLen < 1 {
		o.tQLen = defaultTaskQLen
	}

	if o.tQNumber < 1 {
		o.tQNumber = defaultTaskQNumber
	}

	if o.tQNumber > o.tQPoolSize {
		o.tQNumber = o.tQPoolSize
	}
}

type TaskPoolOption func(*TaskPoolOptions)

// WithTaskPoolTaskPoolSize set @size of the task queue pool size
func WithTaskPoolTaskPoolSize(size int) TaskPoolOption {
	return func(o *TaskPoolOptions) {
		o.tQPoolSize = size
	}
}

// WithTaskPoolTaskQueueLength set @length of the task queue length
func WithTaskPoolTaskQueueLength(length int) TaskPoolOption {
	return func(o *TaskPoolOptions) {
		o.tQLen = length
	}
}

// WithTaskPoolTaskQueueNumber set @number of the task queue number
func WithTaskPoolTaskQueueNumber(number int) TaskPoolOption {
	return func(o *TaskPoolOptions) {
		o.tQNumber = number
	}
}
