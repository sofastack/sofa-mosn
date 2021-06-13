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
	"reflect"
	"testing"
)

func Test_AppendRouterName(t *testing.T) {
	type args struct {
		routerName string
	}
	tests := []struct {
		name string
		args args
		want map[string]bool
	}{
		{
			name: "case1",
			args: args{
				routerName: "http.80",
			},
			want: map[string]bool{"http.80": true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AppendRouterName(tt.args.routerName)
			if got := routerNames; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AppendRouterName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GetRouterNames(t *testing.T) {
	routerNames = make(map[string]bool)
	routerNames["http.80"] = true

	tests := []struct {
		name string
		want []string
	}{
		{
			name: "case1",
			want: []string{"http.80"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetRouterNames(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetRouterNames() = %v, want %v", got, tt.want)
			}
		})
	}
}
