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
	"errors"
	"sync"
	"context"
	"sofastack.io/sofa-mosn/pkg/types"
	mosnctx "sofastack.io/sofa-mosn/pkg/context"
	"strings"
)

var (
	// global scope
	mux              sync.RWMutex
	variables        = make(map[string]Variable, 32) // all built-in variable definitions
	prefixVariables  = make(map[string]Variable, 32) // all prefix getter definitions
	indexedVariables = make([]Variable, 0, 32)       // indexed variables

	// request scope
	indexedValues = make([]IndexedValue, 0, 32) // indexed values, which means its' memory is pre-allocated

	// error message
	errVariableDuplicated   = "duplicate variable register, name: "
	errPrefixDuplicated     = "duplicate prefix variable register, prefix: "
	errUndefinedVariable    = "undefined variable, name: "
	errNoVariablesInContext = "no variables found in context"
	errGetterNotFound       = "getter function undefined, variable name: "
	errSetterNotFound       = "setter function undefined, variable name: "
)

// AddVariable is used to make non-indexed variables into indexed variables. Typical usage is variables used in
// access logs.
func AddVariable(name string) (Variable, error) {
	// find built-in variables
	if variable, ok := variables[name]; ok {
		return variable, nil
	}

	// check prefix variables
	for prefix, variable := range prefixVariables {
		if strings.HasPrefix(name, prefix) {
			// make it into indexed variables
			indexed := NewIndexedVariable(name, name, variable.Getter(), variable.Setter(), variable.Flags())
			// register indexed one
			if err := RegisterVariable(indexed); err != nil {
				return nil, err
			}
			return indexed, nil
		}
	}

	return nil, errors.New(errUndefinedVariable + name)
}

func RegisterVariable(variable Variable) error {
	mux.Lock()
	defer mux.Unlock()

	name := variable.Name()

	// check conflict
	if _, ok := variables[name]; ok {
		return errors.New(errVariableDuplicated + name)
	}

	// register
	variables[name] = variable

	// check index
	if indexer, ok := variable.(Indexer); ok {
		index := len(indexedVariables)
		indexer.SetIndex(uint32(index))

		indexedVariables = append(indexedVariables, variable)
		indexedValues = append(indexedValues, IndexedValue{})
	}
	return nil
}

func RegisterPrefixVariable(prefix string, variable Variable) error {
	mux.Lock()
	defer mux.Unlock()

	// check conflict
	if _, ok := prefixVariables[prefix]; ok {
		return errors.New(errPrefixDuplicated + prefix)
	}

	// register
	prefixVariables[prefix] = variable
	return nil
}

func NewVariableContext(ctx context.Context) context.Context {
	// TODO: sync.Pool reuse
	values := make([]IndexedValue, len(indexedValues)) // TODO: pre-alloc buffer for runtime variable

	return mosnctx.WithValue(ctx, types.ContextKeyVariables, values)
}