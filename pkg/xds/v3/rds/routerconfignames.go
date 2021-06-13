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

package rds

import (
	"sync"
)

/*
rds store the router config names which need to fetch virtualhosts configuration from RDS
*/

var (
	mu          sync.Mutex
	routerNames map[string]bool
)

// AppendRouterName use to append rds router configname to subscript
func AppendRouterName(name string) {
	mu.Lock()
	defer mu.Unlock()
	if routerNames == nil {
		routerNames = make(map[string]bool)
	}
	routerNames[name] = true
}

// GetRouterNames return disctict router config names
func GetRouterNames() []string {
	mu.Lock()
	defer mu.Unlock()
	names := make([]string, len(routerNames))
	i := 0
	for name, _ := range routerNames {
		names[i] = name
		i++
	}
	return names
}
