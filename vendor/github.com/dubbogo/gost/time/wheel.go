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

// Package gxtime encapsulates some golang.time functions
// ref: https://github.com/AlexStocks/go-practice/blob/master/time/siddontang_time_wheel.go
package gxtime

import (
	"sync"
	"time"
)

type Wheel struct {
	sync.RWMutex
	span   time.Duration
	period time.Duration
	ticker *time.Ticker
	index  int
	ring   []chan struct{}
	once   sync.Once
	now    time.Time
}

func NewWheel(span time.Duration, buckets int) *Wheel {
	var (
		w *Wheel
	)

	if span == 0 {
		panic("@span == 0")
	}
	if buckets == 0 {
		panic("@bucket == 0")
	}

	w = &Wheel{
		span:   span,
		period: span * (time.Duration(buckets)),
		ticker: time.NewTicker(span),
		index:  0,
		ring:   make([]chan struct{}, buckets),
		now:    time.Now(),
	}

	go func() {
		var notify chan struct{}
		for t := range w.ticker.C {
			w.Lock()
			w.now = t

			notify = w.ring[w.index]
			w.ring[w.index] = nil
			w.index = (w.index + 1) % len(w.ring)

			w.Unlock()

			if notify != nil {
				close(notify)
			}
		}
	}()

	return w
}

func (w *Wheel) Stop() {
	w.once.Do(func() { w.ticker.Stop() })
}

func (w *Wheel) After(timeout time.Duration) <-chan struct{} {
	if timeout >= w.period {
		panic("@timeout over ring's life period")
	}

	var pos = int(timeout / w.span)
	if 0 < pos {
		pos--
	}

	w.Lock()
	pos = (w.index + pos) % len(w.ring)
	if w.ring[pos] == nil {
		w.ring[pos] = make(chan struct{})
	}
	c := w.ring[pos]
	w.Unlock()

	return c
}

func (w *Wheel) Now() time.Time {
	w.RLock()
	now := w.now
	w.RUnlock()

	return now
}
