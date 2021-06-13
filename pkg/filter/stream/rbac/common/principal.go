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

package common

import (
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"

	envoy_config_rabc_v3 "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v3"
	envoy_type_matcher_v3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"mosn.io/api"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/mtls"
)

type InheritPrincipal interface {
	isInheritPrincipal()
	// A policy matches if and only if at least one of InheritPermission.Match return true
	// AND at least one of InheritPrincipal.Match return true
	Match(cb api.StreamReceiverFilterHandler, headers api.HeaderMap) bool
}

func (*PrincipalAny) isInheritPrincipal()            {}
func (*PrincipalDirectRemoteIp) isInheritPrincipal() {}
func (*PrincipalSourceIp) isInheritPrincipal()       {}
func (*PrincipalRemoteIp) isInheritPrincipal()       {}
func (*PrincipalHeader) isInheritPrincipal()         {}
func (*PrincipalAndIds) isInheritPrincipal()         {}
func (*PrincipalOrIds) isInheritPrincipal()          {}
func (*PrincipalNotId) isInheritPrincipal()          {}
func (*PrincipalMetadata) isInheritPrincipal()       {}

// Principal_Any
type PrincipalAny struct {
	Any bool
}

func NewPrincipalAny(principal *envoy_config_rabc_v3.Principal_Any) (*PrincipalAny, error) {
	return &PrincipalAny{
		Any: principal.Any,
	}, nil
}

func (principal *PrincipalAny) Match(cb api.StreamReceiverFilterHandler, headers api.HeaderMap) bool {
	return principal.Any
}

// Principal_DirectRemoteIp
type PrincipalDirectRemoteIp struct {
	CidrRange *net.IPNet
}

func NewPrincipalDirectRemoteIp(principal *envoy_config_rabc_v3.Principal_DirectRemoteIp) (*PrincipalDirectRemoteIp, error) {
	addressPrefix := principal.DirectRemoteIp.AddressPrefix
	prefixLen := principal.DirectRemoteIp.PrefixLen.GetValue()
	if _, ipNet, err := net.ParseCIDR(addressPrefix + "/" + strconv.Itoa(int(prefixLen))); err != nil {
		return nil, err
	} else {
		inheritPrincipal := &PrincipalDirectRemoteIp{
			CidrRange: ipNet,
		}
		return inheritPrincipal, nil
	}
}

func (principal *PrincipalDirectRemoteIp) Match(cb api.StreamReceiverFilterHandler, headers api.HeaderMap) bool {
	if cb.Connection().RawConn() == nil {
		return false
	}
	directRemoteAddr := cb.Connection().RawConn().RemoteAddr()
	addr, err := net.ResolveTCPAddr(directRemoteAddr.Network(), directRemoteAddr.String())
	if err != nil {
		panic(fmt.Errorf("[PrincipalSourceIp.Match] failed to parse remote address in rbac filter, err: %v", err))
	}
	if principal.CidrRange.Contains(addr.IP) {
		return true
	} else {
		return false
	}
}

// Principal_SourceIp
type PrincipalSourceIp struct {
	CidrRange *net.IPNet
}

func NewPrincipalSourceIp(principal *envoy_config_rabc_v3.Principal_SourceIp) (*PrincipalSourceIp, error) {
	addressPrefix := principal.SourceIp.AddressPrefix
	prefixLen := principal.SourceIp.PrefixLen.GetValue()
	if _, ipNet, err := net.ParseCIDR(addressPrefix + "/" + strconv.Itoa(int(prefixLen))); err != nil {
		return nil, err
	} else {
		inheritPrincipal := &PrincipalSourceIp{
			CidrRange: ipNet,
		}
		return inheritPrincipal, nil
	}
}

func (principal *PrincipalSourceIp) Match(cb api.StreamReceiverFilterHandler, headers api.HeaderMap) bool {
	remoteAddr := cb.Connection().RemoteAddr()
	addr, err := net.ResolveTCPAddr(remoteAddr.Network(), remoteAddr.String())
	if err != nil {
		panic(fmt.Errorf("[PrincipalSourceIp.Match] failed to parse remote address in rbac filter, err: %v", err))
	}
	if principal.CidrRange.Contains(addr.IP) {
		return true
	} else {
		return false
	}
}

// Principal_DirectRemoteIp
type PrincipalRemoteIp struct {
	CidrRange *net.IPNet
}

func NewPrincipalRemoteIp(principal *envoy_config_rabc_v3.Principal_RemoteIp) (*PrincipalRemoteIp, error) {
	addressPrefix := principal.RemoteIp.AddressPrefix
	prefixLen := principal.RemoteIp.PrefixLen.GetValue()
	if _, ipNet, err := net.ParseCIDR(addressPrefix + "/" + strconv.Itoa(int(prefixLen))); err != nil {
		return nil, err
	} else {
		inheritPrincipal := &PrincipalRemoteIp{
			CidrRange: ipNet,
		}
		return inheritPrincipal, nil
	}
}

func (principal *PrincipalRemoteIp) Match(cb api.StreamReceiverFilterHandler, headers api.HeaderMap) bool {

	remoteAddr := cb.Connection().RemoteAddr()
	addr, err := net.ResolveTCPAddr(remoteAddr.Network(), remoteAddr.String())
	if err != nil {
		panic(fmt.Errorf("[PrincipalSourceIp.Match] failed to parse remote address in rbac filter, err: %v", err))
	}
	if principal.CidrRange.Contains(addr.IP) {
		return true
	}
	xffIp, found := principal.getRemoteIpFromXFF(headers)
	if found && principal.CidrRange.Contains(xffIp) {
		return true
	}
	return false
}

func (principal *PrincipalRemoteIp) getRemoteIpFromXFF(headers api.HeaderMap) (net.IP, bool) {
	xff, found := headers.Get("X-Forwarded-For")
	if !found {
		return nil, false
	}
	for _, ip := range strings.Split(xff, ",") {
		ip = strings.TrimSpace(ip)
		if result := net.ParseIP(ip); result != nil {
			return result, true
		}
	}
	return nil, false
}

// Principal_Header
type PrincipalHeader struct {
	Target      string
	Matcher     HeaderMatcher
	InvertMatch bool
}

func NewPrincipalHeader(principal *envoy_config_rabc_v3.Principal_Header) (*PrincipalHeader, error) {
	inheritPrincipal := &PrincipalHeader{}
	inheritPrincipal.Target = principal.Header.Name
	inheritPrincipal.InvertMatch = principal.Header.InvertMatch
	if headerMatcher, err := NewHeaderMatcher(principal.Header); err != nil {
		return nil, err
	} else {
		inheritPrincipal.Matcher = headerMatcher
		return inheritPrincipal, nil
	}
}

func (principal *PrincipalHeader) Match(cb api.StreamReceiverFilterHandler, headers api.HeaderMap) bool {
	targetValue, found := headerMapper(principal.Target, headers)

	// HeaderMatcherPresentMatch is a little special
	if matcher, ok := principal.Matcher.(*HeaderMatcherPresentMatch); ok {
		// HeaderMatcherPresentMatch matches if and only if header found and PresentMatch is true
		isMatch := found && matcher.PresentMatch
		return principal.InvertMatch != isMatch
	}

	// return false when targetValue is not found, except matcher is `HeaderMatcherPresentMatch`
	if !found {
		return false
	} else {
		isMatch := principal.Matcher.Equal(targetValue)
		// principal.InvertMatch xor isMatch
		return principal.InvertMatch != isMatch
	}
}

// Principal_AndIds
type PrincipalAndIds struct {
	AndIds []InheritPrincipal
}

func NewPrincipalAndIds(principal *envoy_config_rabc_v3.Principal_AndIds) (*PrincipalAndIds, error) {
	inheritPrincipal := &PrincipalAndIds{}
	inheritPrincipal.AndIds = make([]InheritPrincipal, len(principal.AndIds.Ids))
	for idx, subPrincipal := range principal.AndIds.Ids {
		if subInheritPrincipal, err := NewInheritPrincipal(subPrincipal); err != nil {
			return nil, err
		} else {
			inheritPrincipal.AndIds[idx] = subInheritPrincipal
		}
	}
	return inheritPrincipal, nil
}

func (principal *PrincipalAndIds) Match(cb api.StreamReceiverFilterHandler, headers api.HeaderMap) bool {
	for _, ids := range principal.AndIds {
		if isMatch := ids.Match(cb, headers); isMatch {
			continue
		} else {
			return false
		}
	}
	return true
}

// Principal_OrIds
type PrincipalOrIds struct {
	OrIds []InheritPrincipal
}

func NewPrincipalOrIds(principal *envoy_config_rabc_v3.Principal_OrIds) (*PrincipalOrIds, error) {
	inheritPrincipal := &PrincipalOrIds{}
	inheritPrincipal.OrIds = make([]InheritPrincipal, len(principal.OrIds.Ids))
	for idx, subPrincipal := range principal.OrIds.Ids {
		if subInheritPrincipal, err := NewInheritPrincipal(subPrincipal); err != nil {
			return nil, err
		} else {
			inheritPrincipal.OrIds[idx] = subInheritPrincipal
		}
	}
	return inheritPrincipal, nil
}

func (principal *PrincipalOrIds) Match(cb api.StreamReceiverFilterHandler, headers api.HeaderMap) bool {
	for _, ids := range principal.OrIds {
		if isMatch := ids.Match(cb, headers); isMatch {
			return true
		} else {
			continue
		}
	}
	return false
}

// Principal_NotId
type PrincipalNotId struct {
	NotId InheritPrincipal
}

func NewPrincipalNotId(principal *envoy_config_rabc_v3.Principal_NotId) (*PrincipalNotId, error) {
	inheritPrincipal := &PrincipalNotId{}
	subPermission := principal.NotId
	if subInheritPermission, err := NewInheritPrincipal(subPermission); err != nil {
		return nil, err
	} else {
		inheritPrincipal.NotId = subInheritPermission
	}
	return inheritPrincipal, nil
}

func (principal *PrincipalNotId) Match(cb api.StreamReceiverFilterHandler, headers api.HeaderMap) bool {
	rule := principal.NotId
	return !rule.Match(cb, headers)
}

// Principal_Metadata
type PrincipalMetadata struct {
	Filter  string
	Path    string
	Matcher StringMatcher
}

func NewPrincipalMetadata(principal *envoy_config_rabc_v3.Principal_Metadata) (*PrincipalMetadata, error) {
	if principal.Metadata == nil || principal.Metadata.Value == nil {
		return nil, fmt.Errorf("unsupported Principal_Metadata Metadata nil: %v", principal)
	}
	// TODO
	if principal.Metadata.Filter != "istio_authn" {
		return nil, fmt.Errorf("unsupported Principal_Metadata filter: %s", principal.Metadata.Filter)
	}
	// TODO:
	path := principal.Metadata.Path
	if len(path) == 0 || path[0].GetKey() != "source.principal" {
		return nil, fmt.Errorf("unsupported Principal_Metadata path: %v", path)
	}
	// TODO
	matcher, ok := principal.Metadata.Value.MatchPattern.(*envoy_type_matcher_v3.ValueMatcher_StringMatch)
	if !ok {
		return nil, fmt.Errorf("unsupported Principal_Metadata matcher: %s", reflect.TypeOf(principal.Metadata.Value))
	}
	stringMatcher, err := NewStringMatcher(matcher.StringMatch)
	if err != nil {
		return nil, err
	}

	return &PrincipalMetadata{
		Filter:  principal.Metadata.Filter,
		Path:    path[0].GetKey(),
		Matcher: stringMatcher,
	}, nil
}

func (principal *PrincipalMetadata) Match(cb api.StreamReceiverFilterHandler, headers api.HeaderMap) bool {
	// TODO
	if principal.Filter != "istio_authn" {
		return false
	}
	// TODO:
	if principal.Path != "source.principal" {
		return false
	}
	// check principal
	conn := cb.Connection().RawConn()
	if conn == nil {
		return false
	}
	if tlsConn, ok := conn.(*mtls.TLSConn); ok {
		cert := tlsConn.ConnectionState().PeerCertificates[0]
		for _, uri := range cert.URIs {
			if uri.Scheme == "spiffe" &&
				principal.Matcher.Equal(strings.TrimPrefix(uri.String(), "spiffe://")) {
				return true
			}
		}
		// Subject Common Name check
		if principal.Matcher.Equal(cert.Subject.CommonName) {
			return true
		}
		if log.Proxy.GetLogLevel() >= log.DEBUG {
			log.DefaultLogger.Debugf("[PrincipalMetadata.Match] cert not match: %+v, %+v", cert.URIs, cert.Subject)
		}
		return false
	} else {
		if log.Proxy.GetLogLevel() >= log.DEBUG {
			log.DefaultLogger.Debugf("[PrincipalMetadata.Match] not tlsConn: %+v", conn)
		}
		return false
	}
}

// Receive the v2alpha.Principal input and convert it to mosn rbac principal
func NewInheritPrincipal(principal *envoy_config_rabc_v3.Principal) (InheritPrincipal, error) {
	// Types that are valid to be assigned to Identifier:
	//	*Principal_AndIds (supported)
	//	*Principal_OrIds (supported)
	//	*Principal_Any (supported)
	//	*Principal_NotId (supported)
	//	*Principal_SourceIp (supported)
	//	*Principal_DirectRemoteIp (supported)
	//	*Principal_RemoteIp (supported)
	//	*Principal_Header (supported)
	//	*Principal_Metadata (supported)
	// TODO:
	//	*Principal_UrlPath
	//	*Principal_Authenticated_
	switch principal.Identifier.(type) {
	case *envoy_config_rabc_v3.Principal_Any:
		return NewPrincipalAny(principal.Identifier.(*envoy_config_rabc_v3.Principal_Any))
	// Difference of DirectRemoteIp, SourceIp, RemoteIp:
	// see https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/rbac/v3/rbac.proto#envoy-v3-api-msg-config-rbac-v3-principal
	case *envoy_config_rabc_v3.Principal_DirectRemoteIp:
		return NewPrincipalDirectRemoteIp(principal.Identifier.(*envoy_config_rabc_v3.Principal_DirectRemoteIp))
	case *envoy_config_rabc_v3.Principal_SourceIp:
		return NewPrincipalSourceIp(principal.Identifier.(*envoy_config_rabc_v3.Principal_SourceIp))
	case *envoy_config_rabc_v3.Principal_RemoteIp:
		return NewPrincipalRemoteIp(principal.Identifier.(*envoy_config_rabc_v3.Principal_RemoteIp))
	case *envoy_config_rabc_v3.Principal_Header:
		return NewPrincipalHeader(principal.Identifier.(*envoy_config_rabc_v3.Principal_Header))
	case *envoy_config_rabc_v3.Principal_AndIds:
		return NewPrincipalAndIds(principal.Identifier.(*envoy_config_rabc_v3.Principal_AndIds))
	case *envoy_config_rabc_v3.Principal_OrIds:
		return NewPrincipalOrIds(principal.Identifier.(*envoy_config_rabc_v3.Principal_OrIds))
	case *envoy_config_rabc_v3.Principal_NotId:
		return NewPrincipalNotId(principal.Identifier.(*envoy_config_rabc_v3.Principal_NotId))
	case *envoy_config_rabc_v3.Principal_Metadata:
		return NewPrincipalMetadata(principal.Identifier.(*envoy_config_rabc_v3.Principal_Metadata))
	default:
		return nil, fmt.Errorf("[NewInheritPrincipal] not supported Principal.Identifier type found, detail: %v",
			reflect.TypeOf(principal.Identifier))
	}
}
