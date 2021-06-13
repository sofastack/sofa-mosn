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

package http

import (
	"testing"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"mosn.io/mosn/pkg/istio/control"
	v1 "mosn.io/mosn/pkg/mixer/v1"
)

func compareData(pb1 proto.Message, pb2 proto.Message) bool {
	mar := jsonpb.Marshaler{}
	data1, _ := mar.MarshalToString(pb1)
	data2, _ := mar.MarshalToString(pb2)

	return data1 == data2
}

func TestExtractForwardedAttributes(t *testing.T) {
	attributes := v1.Attributes{
		Attributes: make(map[string]*v1.Attributes_AttributeValue, 0),
	}
	attributes.Attributes["test_key"] = &v1.Attributes_AttributeValue{
		Value: &v1.Attributes_AttributeValue_StringValue{
			StringValue: "test_value",
		},
	}

	checkData := newMockCheckData()
	mockCheckData, _ := checkData.(*mockCheckData)
	mockCheckData.istioAtttibutes = &attributes

	requestContext := control.NewRequestContext()
	builder := newAttributesBuilder(requestContext)

	err := builder.ExtractForwardedAttributes(checkData)
	if err != nil {
		t.Errorf("ExtractForwardedAttributes fail: %v", err)
	}

	if compareData(&attributes, &requestContext.Attributes) != true {
		t.Errorf("attributes not equal")
	}
	if mockCheckData.extractIstioAttributes != 1 {
		t.Errorf("callttime not equal")
	}
}
