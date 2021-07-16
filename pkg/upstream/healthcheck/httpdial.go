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

package healthcheck

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/types"
)

const (
	TimeoutCfgKey = "timeout"
	PortCfgKey    = "port"
	PathCfgKey    = "path"
)

const (
	defaultTimeout = uint32(30)
)

func init() {
	httpDialSessionFactory := &HTTPDialSessionFactory{}
	RegisterSessionFactory("http", httpDialSessionFactory)
	RegisterSessionFactory("Http1", httpDialSessionFactory)
}

type HTTPDialSession struct {
	timeout uint32
	*url.URL
}

type HTTPDialSessionFactory struct{}

func parseHostToURL(host types.Host) (*url.URL, error) {
	// first try, something like: http://127.0.0.1:3399/hi
	addressStr := host.AddressString()
	u, err := url.Parse(addressStr)
	if err == nil {
		return u, nil
	}

	// try to parse something like: 127.0.0.1:9900
	var ret = &url.URL{}
	ret.Scheme = "http"
	ret.Host = addressStr

	return ret, nil
}

func (f *HTTPDialSessionFactory) NewSession(cfg map[string]interface{}, host types.Host) types.HealthCheckSession {
	var ret = &HTTPDialSession{}

	u, err := parseHostToURL(host)
	if err != nil {
		log.DefaultLogger.Errorf("[upstream] [health check] [httpdial session] parseHostToURL for host %+v error: %v", host, err)
		return nil
	}

	ret.URL = u

	// re-config port
	if v, ok := cfg[PortCfgKey]; ok {
		if _, ok := v.(int); ok {
			portStr := fmt.Sprintf("%d", v)
			address := strings.Split(u.Host, ":")

			switch len(address) {
			case 1:
				address = append(address, portStr)
			case 2:
				address[1] = portStr
			default:
				log.DefaultLogger.Errorf("[upstream] [health check] [httpdial session] unexcepted address splits: %v", address)
				return nil
			}
			ret.URL.Host = strings.Join(address, ":")
		} else {
			log.DefaultLogger.Errorf("[upstream] [health check] [httpdial session] unexcepted port number type: %+v", reflect.TypeOf(v))
			return nil
		}
	}

	if v, ok := cfg[PathCfgKey]; ok {
		if vv, ok := v.(string); ok {
			ret.URL.Path = vv
		}
	}

	if v, ok := cfg[TimeoutCfgKey]; ok {
		if vv, ok := v.(uint32); ok {
			ret.timeout = vv
		}
	} else {
		ret.timeout = defaultTimeout
	}

	return ret
}

func (s *HTTPDialSession) CheckHealth() bool {
	// default dial timeout, maybe already timeout by checker
	client := http.Client{
		Timeout: time.Second * time.Duration(s.timeout),
	}
	resp, err := client.Get(s.String())
	if err != nil {
		if log.DefaultLogger.GetLogLevel() >= log.INFO {
			log.DefaultLogger.Infof("[upstream] [health check] [httpdial session] http check for host %s error: %v", s.String(), err)
		}
		return false
	}

	return resp.StatusCode == http.StatusOK
}

func (s *HTTPDialSession) OnTimeout() {}
