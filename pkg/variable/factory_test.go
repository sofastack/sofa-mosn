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

package variable

import (
	"context"
	"testing"

	mbuffer "mosn.io/mosn/pkg/buffer"
	"mosn.io/mosn/pkg/types"
)

func init() {
	RegisterVariable(NewIndexedVariable(types.VarHost, nil, nil, BasicSetter, 0))
}

func Test_VariablePool(t *testing.T) {
	ctx := mbuffer.NewBufferPoolContext(context.Background())

	ctx = NewVariableContext(ctx)
	SetVariableValue(ctx, types.VarHost, "helloworld.default.svc.cluster.local")

	value, err := GetVariableValue(ctx, types.VarHost)
	if value != "helloworld.default.svc.cluster.local" || err != nil {
		t.Error("get variable value failed")
	}
	if ctx := mbuffer.PoolContext(ctx); ctx != nil {
		ctx.Give()
	}
	value, err = GetVariableValue(ctx, types.VarHost)
	if value != "" || err == nil {
		t.Error("get variable value failed")
	}

	newCtx := NewVariableContext(context.Background())
	value, err = GetVariableValue(newCtx, types.VarHost)
	if value != "" || err == nil {
		t.Error("get variable value failed")
	}
}
