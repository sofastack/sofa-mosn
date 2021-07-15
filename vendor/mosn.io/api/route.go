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

package api

import (
	"context"
	"time"
)

// Route is a route instance
type Route interface {
	// RouteRule returns the route rule
	RouteRule() RouteRule

	// DirectResponseRule returns direct response rule
	DirectResponseRule() DirectResponseRule

	// RedirectRule returns redirect rule
	RedirectRule() RedirectRule
}

// RouteRule defines parameters for a route
type RouteRule interface {
	// VirtualHost returns the virtual host that owns the route
	VirtualHost() VirtualHost

	// ClusterName returns the route's cluster name
	ClusterName(ctx context.Context) string

	// UpstreamProtocol returns the protocol that route's cluster supported
	// If it is configured, the protocol will replace the proxy config's upstream protocol
	UpstreamProtocol() string

	// GlobalTimeout returns the global timeout
	GlobalTimeout() time.Duration

	// Policy returns the route's route policy
	Policy() Policy

	// MetadataMatchCriteria returns the metadata that a subset load balancer should match when selecting an upstream host
	// as we may use weighted cluster's metadata, so need to input cluster's name
	MetadataMatchCriteria(clusterName string) MetadataMatchCriteria

	// PerFilterConfig returns per filter config from xds
	PerFilterConfig() map[string]interface{}

	// FinalizeRequestHeaders do potentially destructive header transforms on request headers prior to forwarding
	FinalizeRequestHeaders(ctx context.Context, headers HeaderMap, requestInfo RequestInfo)

	// FinalizeResponseHeaders do potentially destructive header transforms on response headers prior to forwarding
	FinalizeResponseHeaders(ctx context.Context, headers HeaderMap, requestInfo RequestInfo)

	// PathMatchCriterion returns the route's PathMatchCriterion
	PathMatchCriterion() PathMatchCriterion

	// HeaderMatchCriteria returns the route's HeaderMatchCriteria
	HeaderMatchCriteria() KeyValueMatchCriteria
}

// VirtualHost definition.
type VirtualHost interface {
	Name() string

	// GetRouteFromEntries returns a Route matched the condition
	GetRouteFromEntries(ctx context.Context, headers HeaderMap) Route

	// GetAllRoutesFromEntries returns all Route matched the condition
	GetAllRoutesFromEntries(ctx context.Context, headers HeaderMap) []Route

	// GetRouteFromHeaderKV is used to quickly locate and obtain routes in certain scenarios
	GetRouteFromHeaderKV(key, value string) Route

	// AddRoute adds a new route into virtual host
	AddRoute(route RouteBase) error

	// RemoveAllRoutes clear all the routes in the virtual host
	RemoveAllRoutes()

	// PerFilterConfig returns per filter config from xds
	PerFilterConfig() map[string]interface{}

	// FinalizeRequestHeaders do potentially destructive header transforms on request headers prior to forwarding
	FinalizeRequestHeaders(ctx context.Context, headers HeaderMap, requestInfo RequestInfo)

	// FinalizeResponseHeaders do potentially destructive header transforms on response headers prior to forwarding
	FinalizeResponseHeaders(ctx context.Context, headers HeaderMap, requestInfo RequestInfo)
}

// Policy defines a group of route policy
type Policy interface {
	RetryPolicy() RetryPolicy

	ShadowPolicy() ShadowPolicy

	HashPolicy() HashPolicy

	MirrorPolicy() MirrorPolicy
}

// RetryCheckStatus type
type RetryCheckStatus int

// RetryCheckStatus types
const (
	ShouldRetry   RetryCheckStatus = 0
	NoRetry       RetryCheckStatus = -1
	RetryOverflow RetryCheckStatus = -2
)

// RetryPolicy is a type of Policy
type RetryPolicy interface {
	RetryOn() bool

	TryTimeout() time.Duration

	NumRetries() uint32
}

type DoRetryCallback func()

type RetryState interface {
	Enabled() bool

	ShouldRetry(respHeaders map[string]string, resetReson string, doRetryCb DoRetryCallback) bool
}

// ShadowPolicy is a type of Policy
type ShadowPolicy interface {
	ClusterName() string

	RuntimeKey() string
}

// DirectResponseRule contains direct response info
type DirectResponseRule interface {

	// StatusCode returns the repsonse status code
	StatusCode() int
	// Body returns the response body string
	Body() string
}

// RedirectRule contains redirect info
type RedirectRule interface {
	// RedirectCode returns the redirect repsonse status code
	RedirectCode() int
	// RedirectPath returns the path that will overwrite the current path
	RedirectPath() string
	// RedirectHost returns the host that will overwrite the current host
	RedirectHost() string
	// RedirectScheme returns the scheme that will overwrite the current scheme
	RedirectScheme() string
}

type MetadataMatchCriterion interface {
	// the name of the metadata key
	MetadataKeyName() string

	// the value for the metadata key
	MetadataValue() string
}

type MetadataMatchCriteria interface {
	// @return: a set of MetadataMatchCriterion(metadata) sorted lexically by name
	// to be matched against upstream endpoints when load balancing
	MetadataMatchCriteria() []MetadataMatchCriterion

	MergeMatchCriteria(metadataMatches map[string]string) MetadataMatchCriteria
}

// PathMatchType defines the path match pattern
type PathMatchType uint32

// Path match patterns
const (
	None PathMatchType = iota
	Prefix
	Exact
	Regex
	RPCHeader
	Variable
)

type PathMatchCriterion interface {
	MatchType() PathMatchType
	Matcher() string
}

// KeyValueMatchType defines the header or query param match pattern
type KeyValueMatchType uint32

// Key value match patterns
const (
	ValueExact KeyValueMatchType = iota
	ValueRegex
)

type KeyValueMatchCriterion interface {
	Key() string
	MatchType() KeyValueMatchType
	Matcher() string
}

type KeyValueMatchCriteria interface {
	Get(i int) KeyValueMatchCriterion
	Len() int
	Range(f func(KeyValueMatchCriterion) bool)
}

type HashPolicy interface {
	GenerateHash(context context.Context) uint64
}

type MirrorPolicy interface {
	ClusterName() string
	IsMirror() bool
}

type Matchable interface {
	Match(ctx context.Context, headers HeaderMap) Route
}

type RouteBase interface {
	Route
	Matchable
}
