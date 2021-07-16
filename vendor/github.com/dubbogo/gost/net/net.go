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

package gxnet

import (
	"log"
	"net"
	"strconv"
	"strings"
)

import (
	perrors "github.com/pkg/errors"
)

var (
	privateBlocks []*net.IPNet
)

const (
	// Ipv4SplitCharacter use for slipt Ipv4
	Ipv4SplitCharacter = "."
	// Ipv6SplitCharacter use for slipt Ipv6
	Ipv6SplitCharacter = ":"
)

func init() {
	for _, b := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"} {
		if _, block, err := net.ParseCIDR(b); err == nil {
			privateBlocks = append(privateBlocks, block)
		}
	}
}

// GetLocalIP get local ip
func GetLocalIP() (string, error) {
	faces, err := net.Interfaces()
	if err != nil {
		return "", perrors.WithStack(err)
	}

	var addr net.IP
	for _, face := range faces {
		if !isValidNetworkInterface(face) {
			continue
		}

		addrs, err := face.Addrs()
		if err != nil {
			return "", perrors.WithStack(err)
		}

		if ipv4, ok := getValidIPv4(addrs); ok {
			addr = ipv4
			if isPrivateIP(ipv4) {
				return ipv4.String(), nil
			}
		}
	}

	if addr == nil {
		return "", perrors.Errorf("can not get local IP")
	}

	return addr.String(), nil
}

func isPrivateIP(ip net.IP) bool {
	for _, priv := range privateBlocks {
		if priv.Contains(ip) {
			return true
		}
	}
	return false
}

func getValidIPv4(addrs []net.Addr) (net.IP, bool) {
	for _, addr := range addrs {
		var ip net.IP

		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}

		if ip == nil || ip.IsLoopback() {
			continue
		}

		ip = ip.To4()
		if ip == nil {
			// not an valid ipv4 address
			continue
		}

		return ip, true
	}
	return nil, false
}

func isValidNetworkInterface(face net.Interface) bool {
	if face.Flags&net.FlagUp == 0 {
		// interface down
		return false
	}

	if face.Flags&net.FlagLoopback != 0 {
		// loopback interface
		return false
	}

	if strings.Contains(strings.ToLower(face.Name), "docker") {
		return false
	}

	return true
}

// refer from https://github.com/facebookarchive/grace/blob/master/gracenet/net.go#L180
func IsSameAddr(addr1, addr2 net.Addr) bool {
	if addr1.Network() != addr2.Network() {
		return false
	}

	addr1s := addr1.String()
	addr2s := addr2.String()
	if addr1s == addr2s {
		return true
	}

	// This allows for ipv6 vs ipv4 local addresses to compare as equal. This
	// scenario is common when listening on localhost.
	const ipv6prefix = "[::]"
	addr1s = strings.TrimPrefix(addr1s, ipv6prefix)
	addr2s = strings.TrimPrefix(addr2s, ipv6prefix)
	const ipv4prefix = "0.0.0.0"
	addr1s = strings.TrimPrefix(addr1s, ipv4prefix)
	addr2s = strings.TrimPrefix(addr2s, ipv4prefix)
	return addr1s == addr2s
}

// ListenOnTCPRandomPort a tcp server listening on a random port by tcp protocol
func ListenOnTCPRandomPort(ip string) (*net.TCPListener, error) {
	localAddr := net.TCPAddr{
		IP:   net.IPv4zero,
		Port: 0,
	}
	if len(ip) > 0 {
		localAddr.IP = net.ParseIP(ip)
	}

	// on some containers, u can not bind an random port by the following clause.
	// listener, err := net.Listen("tcp", ":0")

	return net.ListenTCP("tcp4", &localAddr)
}

// ListenOnUDPRandomPort a udp endpoint listening on a random port
func ListenOnUDPRandomPort(ip string) (*net.UDPConn, error) {
	localAddr := net.UDPAddr{
		IP:   net.IPv4zero,
		Port: 0,
	}
	if len(ip) > 0 {
		localAddr.IP = net.ParseIP(ip)
	}

	return net.ListenUDP("udp4", &localAddr)
}

// MatchIP is used to determine whether @pattern and @host:@port match, It's supports subnet/range
func MatchIP(pattern, host, port string) bool {
	// if the pattern is subnet format, it will not be allowed to config port param in pattern.
	if strings.Contains(pattern, "/") {
		_, subnet, _ := net.ParseCIDR(pattern)
		return subnet != nil && subnet.Contains(net.ParseIP(host))
	}
	return matchIPRange(pattern, host, port)
}

func matchIPRange(pattern, host, port string) bool {
	if pattern == "" || host == "" {
		log.Print("Illegal Argument pattern or hostName. Pattern:" + pattern + ", Host:" + host)
		return false
	}

	pattern = strings.TrimSpace(pattern)
	if "*.*.*.*" == pattern || "*" == pattern {
		return true
	}

	isIpv4 := true
	ip4 := net.ParseIP(host).To4()

	if ip4 == nil {
		isIpv4 = false
	}

	hostAndPort := getPatternHostAndPort(pattern, isIpv4)
	if hostAndPort[1] != "" && hostAndPort[1] != port {
		return false
	}

	pattern = hostAndPort[0]
	splitCharacter := Ipv4SplitCharacter
	if !isIpv4 {
		splitCharacter = Ipv6SplitCharacter
	}

	mask := strings.Split(pattern, splitCharacter)
	// check format of pattern
	if err := checkHostPattern(pattern, mask, isIpv4); err != nil {
		log.Printf("gost/net check host pattern error: %s", err.Error())
		return false
	}

	if pattern == host {
		return true
	}

	// short name condition
	if !ipPatternContains(pattern) {
		return pattern == host
	}

	ipAddress := strings.Split(host, splitCharacter)
	for i := 0; i < len(mask); i++ {
		if "*" == mask[i] || mask[i] == ipAddress[i] {
			continue
		} else if strings.Contains(mask[i], "-") {
			rangeNumStrs := strings.Split(mask[i], "-")
			if len(rangeNumStrs) != 2 {
				log.Print("There is wrong format of ip Address: " + mask[i])
				return false
			}
			min := getNumOfIPSegment(rangeNumStrs[0], isIpv4)
			max := getNumOfIPSegment(rangeNumStrs[1], isIpv4)
			ip := getNumOfIPSegment(ipAddress[i], isIpv4)
			if ip < min || ip > max {
				return false
			}
		} else if "0" == ipAddress[i] && "0" == mask[i] || "00" == mask[i] || "000" == mask[i] || "0000" == mask[i] {
			continue
		} else if mask[i] != ipAddress[i] {
			return false
		}
	}
	return true
}

func ipPatternContains(pattern string) bool {
	return strings.Contains(pattern, "*") || strings.Contains(pattern, "-")
}

func checkHostPattern(pattern string, mask []string, isIpv4 bool) error {
	if !isIpv4 {
		if len(mask) != 8 && ipPatternContains(pattern) {
			return perrors.New("If you config ip expression that contains '*' or '-', please fill qualified ip pattern like 234e:0:4567:0:0:0:3d:*. ")
		}
		if len(mask) != 8 && !strings.Contains(pattern, "::") {
			return perrors.New("The host is ipv6, but the pattern is not ipv6 pattern : " + pattern)
		}
	} else {
		if len(mask) != 4 {
			return perrors.New("The host is ipv4, but the pattern is not ipv4 pattern : " + pattern)
		}
	}
	return nil
}

func getPatternHostAndPort(pattern string, isIpv4 bool) []string {
	result := make([]string, 2)
	if strings.HasPrefix(pattern, "[") && strings.Contains(pattern, "]:") {
		end := strings.Index(pattern, "]:")
		result[0] = pattern[1:end]
		result[1] = pattern[end+2:]
	} else if strings.HasPrefix(pattern, "[") && strings.HasSuffix(pattern, "]") {
		result[0] = pattern[1 : len(pattern)-1]
		result[1] = ""
	} else if isIpv4 && strings.Contains(pattern, ":") {
		end := strings.Index(pattern, ":")
		result[0] = pattern[:end]
		result[1] = pattern[end+1:]
	} else {
		result[0] = pattern
	}
	return result
}

func getNumOfIPSegment(ipSegment string, isIpv4 bool) int {
	if isIpv4 {
		ipSeg, _ := strconv.Atoi(ipSegment)
		return ipSeg
	}
	ipSeg, _ := strconv.ParseInt(ipSegment, 0, 16)
	return int(ipSeg)
}
