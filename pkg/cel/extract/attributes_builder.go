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

package extract

import (
	"context"
	"encoding/base64"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"mosn.io/api"
	v1 "mosn.io/mosn/pkg/mixer/v1"
	"mosn.io/mosn/pkg/variable"
	"mosn.io/pkg/buffer"

	"mosn.io/mosn/pkg/cel/attribute"
	"mosn.io/mosn/pkg/istio/utils"
	"mosn.io/mosn/pkg/protocol"
	"mosn.io/mosn/pkg/types"
)

func ExtractAttributes(ctx context.Context, reqHeaders api.HeaderMap, respHeaders api.HeaderMap, requestInfo api.RequestInfo, buf buffer.IoBuffer, trailers api.HeaderMap, now time.Time) attribute.Bag {
	return &extractAttributes{
		ctx:         ctx,
		reqHeaders:  reqHeaders,
		respHeaders: respHeaders,
		requestInfo: requestInfo,
		buf:         buf,
		trailers:    trailers,
		now:         now,
		extracted:   map[string]interface{}{},
	}
}

type extractAttributes struct {
	ctx         context.Context
	reqHeaders  api.HeaderMap
	respHeaders api.HeaderMap
	requestInfo api.RequestInfo
	buf         buffer.IoBuffer
	trailers    api.HeaderMap
	now         time.Time
	attributes  map[string]interface{}
	extracted   map[string]interface{}
}

func (e *extractAttributes) Get(name string) (interface{}, bool) {
	v, ok := e.extracted[name]
	if ok {
		if v == nil {
			return nil, false
		}
		return v, ok
	}

	switch name {
	case utils.KOriginIP:
		addr := e.requestInfo.DownstreamLocalAddress()
		if addr != nil {
			ip, _, ret := getIPPort(addr.String())
			if ret {
				v := net.ParseIP(ip)
				e.extracted[utils.KOriginIP] = v
				return v, true
			}
		}
		e.extracted[utils.KOriginIP] = nil
	case utils.KRequestTime:
		return e.requestInfo.StartTime(), true
	case utils.KContextProtocol:
		proto := "http"
		switch t := e.requestInfo.Protocol(); t {
		case protocol.HTTP1:
			proto = "http"
		case protocol.HTTP2:
			proto = "h2"
		default:
			if t != "" {
				proto = string(t)
			}
		}
		e.extracted[utils.KContextProtocol] = proto
		return proto, true
	case utils.KDestinationIP, utils.KDestinationPort:
		hostInfo := e.requestInfo.UpstreamHost()
		if hostInfo != nil {
			address := hostInfo.AddressString()
			ip, port, ret := getIPPort(address)
			if ret {
				e.extracted[utils.KDestinationIP] = net.ParseIP(ip)
				e.extracted[utils.KDestinationPort] = int64(port)
				return e.extracted[name], true
			}
		}
		e.extracted[utils.KDestinationIP] = nil
		e.extracted[utils.KDestinationPort] = nil
	case utils.KRequestHeaders:
		return e.reqHeaders, true
	case utils.KResponseHeaders:
		return e.respHeaders, true
	case utils.KResponseTime:
		return e.now, true
	case utils.KRequestBodySize:
		return int64(e.requestInfo.BytesReceived()), true
	case utils.KResponseBodySize:
		return int64(e.requestInfo.BytesSent()), true
	case utils.KRequestTotalSize:
		var sum int64
		if e.reqHeaders != nil {
			sum += int64(e.reqHeaders.ByteSize())
		}
		if e.buf != nil {
			sum += int64(e.buf.Len())
		}
		if e.trailers != nil {
			sum += int64(e.trailers.ByteSize())
		}
		e.extracted[utils.KRequestTotalSize] = sum
		return sum, true
	case utils.KResponseTotalSize:
		return int64(e.requestInfo.BytesSent() + e.respHeaders.ByteSize()), true
	case utils.KResponseDuration:
		return e.requestInfo.Duration(), true
	case utils.KResponseCode:
		return int64(e.requestInfo.ResponseCode()), true
	case utils.KRequestPath:
		path, err := variable.GetVariableValue(e.ctx, types.VarPath)
		if err != nil || path == "" {
			return nil, false
		}
		return path, true
	case utils.KRequestQueryParms:
		query, err := variable.GetVariableValue(e.ctx, types.VarQueryString)
		if err == nil && query != "" {
			v, err := parseQuery(query)
			if err == nil {
				v := protocol.CommonHeader(v)
				e.extracted[utils.KRequestQueryParms] = v
				return e.extracted[name], true
			}
		}
		e.extracted[utils.KRequestQueryParms] = nil
	case utils.KRequestUrlPath:
		path, err := variable.GetVariableValue(e.ctx, types.VarPath)
		if err == nil && path != "" {
			query, err := variable.GetVariableValue(e.ctx, types.VarQueryString)
			if err == nil && query != "" {
				url := path + "?" + query
				e.extracted[utils.KRequestUrlPath] = url
				return e.extracted[name], true
			}
			e.extracted[utils.KRequestUrlPath] = path
			return e.extracted[name], true
		}
		e.extracted[utils.KRequestUrlPath] = nil
	case utils.KRequestMethod:
		method, err := variable.GetVariableValue(e.ctx, types.VarMethod)
		if err != nil || method == "" {
			return nil, false
		}
		return method, true
	case utils.KRequestHost:
		host, err := variable.GetVariableValue(e.ctx, types.VarHost)
		if err != nil || host == "" {
			return nil, false
		}
		return host, true
	case utils.KDestinationServiceHost, utils.KDestinationServiceName, utils.KDestinationServiceNamespace, utils.KContextReporterKind:
		routeEntry := e.requestInfo.RouteEntry()
		if routeEntry != nil {
			clusterName := routeEntry.ClusterName()
			if clusterName != "" {
				info := paresClusterName(clusterName)
				e.extracted[utils.KDestinationServiceHost] = info.Host
				e.extracted[utils.KDestinationServiceName] = info.Name
				e.extracted[utils.KDestinationServiceNamespace] = info.Namespace
				e.extracted[utils.KContextReporterKind] = info.Kind
				return e.extracted[name], true
			}
		}
		fallthrough
	default:
		if e.attributes == nil {
			e.attributes = map[string]interface{}{}
			val, ret := e.reqHeaders.Get(utils.KIstioAttributeHeader)
			if ret {
				d, err := base64.StdEncoding.DecodeString(val)
				if err == nil {
					var attibutes v1.Attributes
					err = proto.Unmarshal(d, &attibutes)
					if err == nil {
						e.attributes = attributesToStringInterfaceMap(e.attributes, attibutes)
					}
				}
			}
		}
		v, ok = e.attributes[name]
		if ok {
			return v, ok
		}
	}
	return nil, false
}

// getIPPort return ip and port of address
func getIPPort(address string) (ip string, port int32, ret bool) {
	ret = false
	array := strings.Split(address, ":")
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

func attributesToStringInterfaceMap(out map[string]interface{}, attributes v1.Attributes) map[string]interface{} {
	if out == nil {
		out = map[string]interface{}{}
	}
	for key, val := range attributes.Attributes {
		var v interface{}
		switch t := val.Value.(type) {
		case *v1.Attributes_AttributeValue_StringValue:
			v = t.StringValue
		case *v1.Attributes_AttributeValue_Int64Value:
			v = t.Int64Value
		case *v1.Attributes_AttributeValue_DoubleValue:
			v = t.DoubleValue
		case *v1.Attributes_AttributeValue_BoolValue:
			v = t.BoolValue
		case *v1.Attributes_AttributeValue_BytesValue:
			v = t.BytesValue
		case *v1.Attributes_AttributeValue_TimestampValue:
			v = time.Unix(t.TimestampValue.Seconds, int64(t.TimestampValue.Nanos))
		case *v1.Attributes_AttributeValue_DurationValue:
			v = time.Duration(t.DurationValue.Seconds)*time.Second + time.Duration(t.DurationValue.Nanos)*time.Nanosecond
		case *v1.Attributes_AttributeValue_StringMapValue:
			v = protocol.CommonHeader(t.StringMapValue.Entries)
		}
		if v != nil {
			out[key] = v
		}
	}
	return out
}

var outbound = "outbound"

type clusterNameInfo struct {
	Kind      string
	Version   string
	Name      string
	Namespace string
	Host      string
}

func paresClusterName(clusterName string) *clusterNameInfo {
	c := &clusterNameInfo{}
	items := strings.Split(clusterName, "|")
	if len(items) == 4 {
		c.Kind = items[0]
		if outbound == c.Kind {
			c.Version = items[2]
		}
		c.Host = items[3]
		serviceItems := strings.SplitN(c.Host, ".", 3)
		if len(serviceItems) == 3 {
			c.Name = serviceItems[0]
			c.Namespace = serviceItems[1]
		}
	}
	return c
}

func parseQuery(query string) (m map[string]string, err error) {
	m = map[string]string{}
	for query != "" {
		key := query
		if i := strings.IndexAny(key, "&;"); i >= 0 {
			key, query = key[:i], key[i+1:]
		} else {
			query = ""
		}
		if key == "" {
			continue
		}
		value := ""
		if i := strings.Index(key, "="); i >= 0 {
			key, value = key[:i], key[i+1:]
		}
		key, err1 := url.QueryUnescape(key)
		if err1 != nil {
			if err == nil {
				err = err1
			}
			continue
		}
		// Only the first value is taken by default
		if _, ok := m[key]; ok {
			continue
		}
		value, err1 = url.QueryUnescape(value)
		if err1 != nil {
			if err == nil {
				err = err1
			}
			continue
		}

		m[key] = value
	}
	return m, err
}
