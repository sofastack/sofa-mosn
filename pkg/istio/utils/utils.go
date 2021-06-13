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
	"net"
	"strconv"
	"strings"

	"github.com/gogo/protobuf/jsonpb"
	"mosn.io/api"
	v1 "mosn.io/mosn/pkg/mixer/v1"
)

const (
//kPerHostMetadataKey = "istio"
)

// GetDestinationUID function
func GetDestinationUID(metadata api.Metadata) (uid string, err error) {
	// TODO
	return "", nil
	/*
		v, exist := metadata[kPerHostMetadataKey]
		if !exist {
			err = fmt.Errorf("cannot find %s metadata", kPerHostMetadataKey)
			return
		}
	*/
}

// GetIPPort return ip and port of address
func GetIPPort(address net.Addr) (ip string, port int32, ret bool) {
	ret = false
	array := strings.Split(address.String(), ":")
	if len(array) != 2 {
		return
	}
	p, err := strconv.Atoi(array[1])
	if err != nil {
		return
	}

	ip = array[0]
	port = int32(p)
	ret = true
	return
}

// FormatAttributesString return attributes json string
func FormatAttributesString(attributes *v1.Attributes) (str string, err error) {
	var mar jsonpb.Marshaler
	str, err = mar.MarshalToString(attributes)

	return
}
