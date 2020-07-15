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

package network

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"mosn.io/api"
	"mosn.io/mosn/pkg/log"
	"mosn.io/pkg/buffer"
)

const UdpPacketMaxSize = 64 * 1024

var (
	bufPool  =  buffer.IoBufferPool{}
	ProxyMap = sync.Map{}
)

func GetProxyMapKey(raddr, laddr string) string {
	return fmt.Sprintf("%s:%s", raddr, laddr)
}

func SetUdpProxyMap(key string, conn api.Connection) {
	ProxyMap.Store(key, conn)
}

func DelUdpProxyMap(key string) {
	ProxyMap.Delete(key)
}

func ReadMsgLoop(lctx context.Context, l *listener) {
	conn := l.packetConn.(*net.UDPConn)
	buf := buffer.GetIoBuffer(UdpPacketMaxSize)
	defer buffer.PutIoBuffer(buf)
	for {
		buf.Reset()
		n, rAddr, err := conn.ReadFromUDP(buf.Bytes()[:buf.Cap()])
		buf.Grow(n)

		if err != nil {
			if nerr, ok := err.(net.Error); ok && (nerr.Timeout() || nerr.Timeout()) {
				log.DefaultLogger.Errorf("[network] [udp] recv from udp error: %v", err)
				break
			}
			if strings.Contains(err.Error(), "use of closed network connection") {
				log.DefaultLogger.Errorf("[network] [udp] recv from udp error: %v", err)
				break
			}
			log.DefaultLogger.Errorf("[network] [udp] recv from udp error: %v", err)
			continue
		}

		proxyKey := GetProxyMapKey(conn.LocalAddr().String(), rAddr.String())
		if dc, ok := ProxyMap.Load(proxyKey); !ok {
			fd, _ := conn.File()
			clientConn, _ := net.FilePacketConn(fd)

			l.cb.OnAccept(clientConn.(*net.UDPConn), l.useOriginalDst, rAddr, nil, buf.Bytes()[0:n])
		} else {
			c := dc.(api.Connection)
			c.OnRead(buf)
		}
	}
}
