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

package utils

import (
	"fmt"
	"testing"

	v1 "mosn.io/mosn/pkg/mixer/v1"
)

func TestFormatAttributesString(t *testing.T) {
	attributes := v1.Attributes{
		Attributes: map[string]*v1.Attributes_AttributeValue{
			"ip": &v1.Attributes_AttributeValue{
				Value: &v1.Attributes_AttributeValue_StringValue{
					StringValue: "127.0.0.1",
				},
			},
		},
	}

	str, err := FormatAttributesString(&attributes)
	if err != nil {
		t.Errorf("format error: %v", err)
	}
	fmt.Printf("str: %s", str)
}
