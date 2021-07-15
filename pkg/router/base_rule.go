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

package router

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"mosn.io/api"
	v2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/types"
	"mosn.io/mosn/pkg/upstream/cluster"
	"mosn.io/mosn/pkg/variable"
)

var (
	// https://tools.ietf.org/html/rfc3986#section-3.1
	schemeValidator = regexp.MustCompile("^[a-z][a-z0-9.+-]*$")
)

type RouteRuleImplBase struct {
	// match
	vHost       api.VirtualHost
	routerMatch v2.RouterMatch
	// rewrite
	prefixRewrite         string
	regexRewrite          v2.RegexRewrite
	regexPattern          *regexp.Regexp
	hostRewrite           string
	autoHostRewrite       bool
	autoHostRewriteHeader string
	requestHeadersParser  *headerParser
	responseHeadersParser *headerParser
	// information
	upstreamProtocol string
	perFilterConfig  map[string]interface{}
	// policy
	policy *policy
	// direct response
	directResponseRule *directResponseImpl
	// redirect
	redirectRule *redirectImpl
	// action
	routerAction       v2.RouteAction
	defaultCluster     *weightedClusterEntry // cluster name and metadata
	weightedClusters   map[string]weightedClusterEntry
	totalClusterWeight uint32
	lock               sync.Mutex
	randInstance       *rand.Rand
}

func NewRouteRuleImplBase(vHost api.VirtualHost, route *v2.Router) (*RouteRuleImplBase, error) {
	base := &RouteRuleImplBase{
		vHost:                 vHost,
		routerMatch:           route.Match,
		prefixRewrite:         route.Route.PrefixRewrite,
		hostRewrite:           route.Route.HostRewrite,
		autoHostRewrite:       route.Route.AutoHostRewrite,
		autoHostRewriteHeader: route.Route.AutoHostRewriteHeader,
		requestHeadersParser:  getHeaderParser(route.Route.RequestHeadersToAdd, nil),
		responseHeadersParser: getHeaderParser(route.Route.ResponseHeadersToAdd, route.Route.ResponseHeadersToRemove),
		upstreamProtocol:      route.Route.UpstreamProtocol,
		perFilterConfig:       route.PerFilterConfig,
		policy:                &policy{},
		routerAction:          route.Route,
		defaultCluster: &weightedClusterEntry{
			clusterName: route.Route.ClusterName,
		},
		lock: sync.Mutex{},
	}
	//check and store regrex rewrite pattern
	if route.Route.RegexRewrite != nil && len(route.Route.RegexRewrite.Pattern.Regex) > 1 && len(route.Route.PrefixRewrite) == 0 {
		base.regexRewrite = *route.Route.RegexRewrite
		regexPattern, err := regexp.Compile(route.Route.RegexRewrite.Pattern.Regex)
		if err != nil {
			log.DefaultLogger.Errorf(RouterLogFormat, "routerule", "check regrex pattern failed.", "invalid regex:"+err.Error())
			return nil, err
		}
		base.regexPattern = regexPattern
	}

	// add clusters
	base.weightedClusters, base.totalClusterWeight = getWeightedClusterEntry(route.Route.WeightedClusters)
	if len(route.Route.MetadataMatch) > 0 {
		base.defaultCluster.clusterMetadataMatchCriteria = NewMetadataMatchCriteriaImpl(route.Route.MetadataMatch)
	}
	// add policy
	if route.Route.RetryPolicy != nil {
		base.policy.retryPolicy = &retryPolicyImpl{
			retryOn:      route.Route.RetryPolicy.RetryOn,
			retryTimeout: route.Route.RetryPolicy.RetryTimeout,
			numRetries:   route.Route.RetryPolicy.NumRetries,
		}
	}
	// add hash policy
	if route.Route.HashPolicy != nil && len(route.Route.HashPolicy) >= 1 {
		hp := route.Route.HashPolicy[0]
		if hp.Header != nil {
			base.policy.hashPolicy = &headerHashPolicyImpl{
				key: hp.Header.Key,
			}
		}
		if hp.Cookie != nil {
			base.policy.hashPolicy = &cookieHashPolicyImpl{
				name: hp.Cookie.Name,
				path: hp.Cookie.Path,
				ttl:  hp.Cookie.TTL,
			}
		}
	}
	// use source ip hash policy as default hash policy
	if base.policy.hashPolicy == nil {
		base.policy.hashPolicy = &sourceIPHashPolicyImpl{}
	}
	// add direct repsonse rule
	if route.DirectResponse != nil {
		base.directResponseRule = &directResponseImpl{
			status: route.DirectResponse.StatusCode,
			body:   route.DirectResponse.Body,
		}
	}
	// add redirect rule
	if route.Redirect != nil {
		r := route.Redirect

		scheme := r.SchemeRedirect
		if len(scheme) > 0 {
			scheme = strings.ToLower(scheme)
			if !schemeValidator.MatchString(scheme) {
				return nil, fmt.Errorf("invalid scheme: %s", scheme)
			}
		}

		rule := &redirectImpl{
			path:   r.PathRedirect,
			host:   r.HostRedirect,
			scheme: scheme,
		}

		switch r.ResponseCode {
		case 0:
			// default to 301
			rule.code = http.StatusMovedPermanently
		case http.StatusMovedPermanently, http.StatusFound,
			http.StatusSeeOther, http.StatusTemporaryRedirect,
			http.StatusPermanentRedirect:
			rule.code = r.ResponseCode
		default:
			return nil, fmt.Errorf("redirect code not supported yet: %d", r.ResponseCode)
		}
		base.redirectRule = rule
	}

	// add mirror policies
	if route.RequestMirrorPolicies != nil {
		base.policy.mirrorPolicy = &mirrorImpl{
			cluster: route.RequestMirrorPolicies.Cluster,
			percent: int(route.RequestMirrorPolicies.Percent),
			rand:    rand.New(rand.NewSource(time.Now().UnixNano())),
		}
	}
	if base.policy.mirrorPolicy == nil {
		base.policy.mirrorPolicy = &mirrorImpl{}
	}
	return base, nil
}

func (rri *RouteRuleImplBase) VirtualHost() api.VirtualHost {
	return rri.vHost
}

func (rri *RouteRuleImplBase) DirectResponseRule() api.DirectResponseRule {
	return rri.directResponseRule
}

func (rri *RouteRuleImplBase) RedirectRule() api.RedirectRule {
	if rri.redirectRule == nil {
		return nil
	}
	return rri.redirectRule
}

// types.RouteRule
// Select Cluster for Routing
// if weighted cluster is nil, return clusterName directly, else
// select cluster from weighted-clusters
func (rri *RouteRuleImplBase) ClusterName(ctx context.Context) string {
	if len(rri.weightedClusters) == 0 {
		// If both 'cluster_name' and 'cluster_variable' are configured, 'cluster_name' is preferred.
		if rri.defaultCluster.clusterName != "" {
			return rri.defaultCluster.clusterName
		}

		if clusterName, err := variable.GetString(ctx, rri.routerAction.ClusterVariable); err == nil {
			return clusterName
		}

		return rri.defaultCluster.clusterName
	}

	rri.lock.Lock()
	if rri.randInstance == nil {
		rri.randInstance = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	selectedValue := rri.randInstance.Intn(int(rri.totalClusterWeight))
	rri.lock.Unlock()

	for _, weightCluster := range rri.weightedClusters {
		selectedValue = selectedValue - int(weightCluster.clusterWeight)
		if selectedValue <= 0 {
			return weightCluster.clusterName
		}
	}

	return rri.defaultCluster.clusterName
}

func (rri *RouteRuleImplBase) UpstreamProtocol() string {
	return rri.upstreamProtocol
}

func (rri *RouteRuleImplBase) GlobalTimeout() time.Duration {
	return rri.routerAction.Timeout
}

func (rri *RouteRuleImplBase) Policy() api.Policy {
	return rri.policy
}

func (rri *RouteRuleImplBase) MetadataMatchCriteria(clusterName string) api.MetadataMatchCriteria {
	criteria := rri.defaultCluster.clusterMetadataMatchCriteria
	if len(rri.weightedClusters) != 0 {
		if cluster, ok := rri.weightedClusters[clusterName]; ok {
			criteria = cluster.clusterMetadataMatchCriteria
		}
	}
	if criteria == nil {
		return nil
	}
	return criteria

}

func (rri *RouteRuleImplBase) PerFilterConfig() map[string]interface{} {
	return rri.perFilterConfig
}

func (rri *RouteRuleImplBase) FinalizePathHeader(ctx context.Context, headers api.HeaderMap, matchedPath string) {
	rri.finalizePathHeader(ctx, headers, matchedPath)
}

func (rri *RouteRuleImplBase) finalizePathHeader(ctx context.Context, headers api.HeaderMap, matchedPath string) {

	if len(rri.prefixRewrite) < 1 && len(rri.regexRewrite.Pattern.Regex) < 1 {
		return
	}

	path, err := variable.GetString(ctx, types.VarPath)
	if err == nil && path != "" {

		//If both prefix_rewrite and regex_rewrite are configured
		//prefix rewrite by default
		if len(rri.prefixRewrite) > 1 {
			if strings.HasPrefix(path, matchedPath) {
				// origin path need to save in the header
				headers.Set(types.HeaderOriginalPath, path)
				variable.SetString(ctx, types.VarPath, rri.prefixRewrite+path[len(matchedPath):])
				if log.DefaultLogger.GetLogLevel() >= log.INFO {
					log.DefaultLogger.Infof(RouterLogFormat, "routerule", "finalizePathHeader", "add prefix to path, prefix is "+rri.prefixRewrite)
				}
			}
			return
		}

		// regex rewrite path if configured
		if len(rri.regexRewrite.Pattern.Regex) > 1 && rri.regexPattern != nil {
			rewritedPath := rri.regexPattern.ReplaceAllString(path, rri.regexRewrite.Substitution)
			if rewritedPath != path {
				headers.Set(types.HeaderOriginalPath, path)
				variable.SetString(ctx, types.VarPath, rewritedPath)
				if log.DefaultLogger.GetLogLevel() >= log.INFO {
					log.DefaultLogger.Infof(RouterLogFormat, "routerule", "finalizePathHeader", "regex rewrite path, rewrited path is "+rewritedPath)
				}
			}
		}

	}
}

func (rri *RouteRuleImplBase) FinalizeRequestHeaders(ctx context.Context, headers api.HeaderMap, requestInfo api.RequestInfo) {
	rri.finalizeRequestHeaders(ctx, headers, requestInfo)
}

func (rri *RouteRuleImplBase) finalizeRequestHeaders(ctx context.Context, headers api.HeaderMap, requestInfo api.RequestInfo) {
	rri.requestHeadersParser.evaluateHeaders(headers, requestInfo)
	rri.vHost.FinalizeRequestHeaders(ctx, headers, requestInfo)
	if len(rri.hostRewrite) > 0 {
		variable.SetString(ctx, types.VarIstioHeaderHost, rri.hostRewrite)
	} else if len(rri.autoHostRewriteHeader) > 0 {
		if headerValue, ok := headers.Get(rri.autoHostRewriteHeader); ok {
			variable.SetString(ctx, types.VarIstioHeaderHost, headerValue)
		}
	} else if rri.autoHostRewrite {
		clusterSnapshot := cluster.GetClusterMngAdapterInstance().GetClusterSnapshot(context.TODO(), rri.routerAction.ClusterName)
		if clusterSnapshot != nil && (clusterSnapshot.ClusterInfo().ClusterType() == v2.STRICT_DNS_CLUSTER) {
			variable.SetString(ctx, types.VarIstioHeaderHost, requestInfo.UpstreamHost().Hostname())
		}
	}
}

func (rri *RouteRuleImplBase) FinalizeResponseHeaders(ctx context.Context, headers api.HeaderMap, requestInfo api.RequestInfo) {
	rri.responseHeadersParser.evaluateHeaders(headers, requestInfo)
	rri.vHost.FinalizeResponseHeaders(ctx, headers, requestInfo)
}
