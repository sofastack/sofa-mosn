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

package gxcontext

import (
	"context"
)

var (
	defaultCtxKey = 1
)

type Values struct {
	m map[interface{}]interface{}
}

func (v Values) Get(key interface{}) (interface{}, bool) {
	i, b := v.m[key]
	return i, b
}

func (c Values) Set(key interface{}, value interface{}) {
	c.m[key] = value
}

func (c Values) Delete(key interface{}) {
	delete(c.m, key)
}

type ValuesContext struct {
	context.Context
}

func NewValuesContext(ctx context.Context) *ValuesContext {
	if ctx == nil {
		ctx = context.Background()
	}

	return &ValuesContext{
		Context: context.WithValue(
			ctx,
			defaultCtxKey,
			Values{m: make(map[interface{}]interface{}, 4)},
		),
	}
}

func (c *ValuesContext) Get(key interface{}) (interface{}, bool) {
	return c.Context.Value(defaultCtxKey).(Values).Get(key)
}

func (c *ValuesContext) Delete(key interface{}) {
	c.Context.Value(defaultCtxKey).(Values).Delete(key)
}

func (c *ValuesContext) Set(key interface{}, value interface{}) {
	c.Context.Value(defaultCtxKey).(Values).Set(key, value)
}
