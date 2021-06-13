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
	"reflect"
	"regexp"
	"strconv"
	"strings"

	envoy_config_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_type_matcher_v3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
)

// StringMatcher
type StringMatcher interface {
	//	*StringMatcher_Exact (supported)
	//	*StringMatcher_Prefix (supported)
	//	*StringMatcher_Suffix (supported)
	//	*StringMatcher_SafeRegex
	// TODO:
	//	*StringMatcher_HiddenEnvoyDeprecatedRegex
	//	*StringMatcher_Contains
	Equal(string) bool
}

// StringMatcher_Exact
type ExactStringMatcher struct {
	ExactMatch string
}

func (matcher *ExactStringMatcher) Equal(targetValue string) bool {
	return matcher.ExactMatch == targetValue
}

// StringMatcher_Prefix
type PrefixStringMatcher struct {
	PrefixMatch string
}

func (matcher *PrefixStringMatcher) Equal(targetValue string) bool {
	return strings.HasPrefix(targetValue, matcher.PrefixMatch)
}

// StringMatcher_Suffix
type SuffixStringMatcher struct {
	SuffixMatch string
}

func (matcher *SuffixStringMatcher) Equal(targetValue string) bool {
	return strings.HasSuffix(targetValue, matcher.SuffixMatch)
}

// StringMatcher_Regex
type RegexStringMatcher struct {
	RegexMatch *regexp.Regexp
}

func (matcher *RegexStringMatcher) Equal(targetValue string) bool {
	return matcher.RegexMatch.MatchString(targetValue)
}

func NewStringMatcher(match *envoy_type_matcher_v3.StringMatcher) (StringMatcher, error) {
	if match.IgnoreCase {
		// TODO
		return nil, fmt.Errorf("[NewStringMatcher] not support IgnoreCase")
	}
	switch match.MatchPattern.(type) {
	case *envoy_type_matcher_v3.StringMatcher_Exact:
		return &ExactStringMatcher{
			ExactMatch: match.MatchPattern.(*envoy_type_matcher_v3.StringMatcher_Exact).Exact,
		}, nil
	case *envoy_type_matcher_v3.StringMatcher_Prefix:
		return &PrefixStringMatcher{
			PrefixMatch: match.MatchPattern.(*envoy_type_matcher_v3.StringMatcher_Prefix).Prefix,
		}, nil
	case *envoy_type_matcher_v3.StringMatcher_Suffix:
		return &SuffixStringMatcher{
			SuffixMatch: match.MatchPattern.(*envoy_type_matcher_v3.StringMatcher_Suffix).Suffix,
		}, nil
	case *envoy_type_matcher_v3.StringMatcher_SafeRegex:
		re := match.MatchPattern.(*envoy_type_matcher_v3.StringMatcher_SafeRegex).SafeRegex
		if _, ok := re.EngineType.(*envoy_type_matcher_v3.RegexMatcher_GoogleRe2); !ok {
			return nil, fmt.Errorf("[NewStringMatcher] failed to build regex, unsupported engine type: %v", reflect.TypeOf(re.EngineType))
		}
		if rePattern, err := regexp.Compile(re.Regex); err != nil {
			return nil, fmt.Errorf("[NewStringMatcher] failed to build regex, error: %v", err)
		} else {
			return &RegexStringMatcher{
				RegexMatch: rePattern,
			}, nil
		}
	default:
		return nil, fmt.Errorf(
			"[NewStringMatcher] not support StringMatcher type found, detail: %v",
			reflect.TypeOf(match.MatchPattern))
	}
}

// HeaderMatcher
type HeaderMatcher interface {
	//	*HeaderMatcher_ExactMatch (supported)
	//	*HeaderMatcher_RegexMatch (supported)
	//	*HeaderMatcher_RangeMatch (supported)
	//	*HeaderMatcher_PresentMatch (supported)
	//	*HeaderMatcher_PrefixMatch (supported)
	//	*HeaderMatcher_SuffixMatch (supported)
	// TODO:
	//	*HeaderMatcher_SafeRegexMatch
	isHeaderMatcher()
	Equal(string) bool
}

func (matcher *ExactStringMatcher) isHeaderMatcher()        {}
func (matcher *PrefixStringMatcher) isHeaderMatcher()       {}
func (matcher *SuffixStringMatcher) isHeaderMatcher()       {}
func (matcher *RegexStringMatcher) isHeaderMatcher()        {}
func (matcher *HeaderMatcherPresentMatch) isHeaderMatcher() {}
func (matcher *HeaderMatcherRangeMatch) isHeaderMatcher()   {}

// HeaderMatcher_PresentMatch
type HeaderMatcherPresentMatch struct {
	PresentMatch bool
}

func (matcher *HeaderMatcherPresentMatch) Equal(targetValue string) bool {
	return matcher.PresentMatch
}

// HeaderMatcher_RangeMatch
type HeaderMatcherRangeMatch struct {
	Start int64 // inclusive
	End   int64 // exclusive
}

func (matcher *HeaderMatcherRangeMatch) Equal(targetValue string) bool {
	if intValue, err := strconv.ParseInt(targetValue, 10, 64); err != nil {
		// return not match if target value is not a integer
		return false
	} else {
		return intValue >= matcher.Start && intValue < matcher.End
	}
}

func NewHeaderMatcher(header *envoy_config_route_v3.HeaderMatcher) (HeaderMatcher, error) {
	switch header.HeaderMatchSpecifier.(type) {
	case *envoy_config_route_v3.HeaderMatcher_ExactMatch:
		return &ExactStringMatcher{
			ExactMatch: header.HeaderMatchSpecifier.(*envoy_config_route_v3.HeaderMatcher_ExactMatch).ExactMatch,
		}, nil
	case *envoy_config_route_v3.HeaderMatcher_PrefixMatch:
		return &PrefixStringMatcher{
			PrefixMatch: header.HeaderMatchSpecifier.(*envoy_config_route_v3.HeaderMatcher_PrefixMatch).PrefixMatch,
		}, nil
	case *envoy_config_route_v3.HeaderMatcher_SuffixMatch:
		return &SuffixStringMatcher{
			SuffixMatch: header.HeaderMatchSpecifier.(*envoy_config_route_v3.HeaderMatcher_SuffixMatch).SuffixMatch,
		}, nil
	case *envoy_config_route_v3.HeaderMatcher_SafeRegexMatch:
		re := header.HeaderMatchSpecifier.(*envoy_config_route_v3.HeaderMatcher_SafeRegexMatch).SafeRegexMatch
		if _, ok := re.EngineType.(*envoy_type_matcher_v3.RegexMatcher_GoogleRe2); !ok {
			return nil, fmt.Errorf("[NewHeaderMatcher] failed to build regex, unsupported engine type: %v", reflect.TypeOf(re.EngineType))
		}
		if rePattern, err := regexp.Compile(re.Regex); err != nil {
			return nil, fmt.Errorf("[NewHeaderMatcher] failed to build regex, error: %v", err)
		} else {
			return &RegexStringMatcher{
				RegexMatch: rePattern,
			}, nil
		}
	case *envoy_config_route_v3.HeaderMatcher_PresentMatch:
		return &HeaderMatcherPresentMatch{
			PresentMatch: header.HeaderMatchSpecifier.(*envoy_config_route_v3.HeaderMatcher_PresentMatch).PresentMatch,
		}, nil
	case *envoy_config_route_v3.HeaderMatcher_RangeMatch:
		return &HeaderMatcherRangeMatch{
			Start: header.HeaderMatchSpecifier.(*envoy_config_route_v3.HeaderMatcher_RangeMatch).RangeMatch.Start,
			End:   header.HeaderMatchSpecifier.(*envoy_config_route_v3.HeaderMatcher_RangeMatch).RangeMatch.End,
		}, nil
	default:
		return nil, fmt.Errorf(
			"[NewHeaderMatcher] not support HeaderMatchSpecifier type found, detail: %v",
			reflect.TypeOf(header.HeaderMatchSpecifier))
	}
}

// UrlPathMatcher
type UrlPathMatcher interface {
	//	*PathMatcher_Path (supported)
	isUrlPathMatcher()
	Equal(string) bool
}

func (matcher *DefaultUrlPathMatcher) isUrlPathMatcher() {}

type DefaultUrlPathMatcher struct {
	Matcher StringMatcher
}

func (matcher *DefaultUrlPathMatcher) Equal(targetValue string) bool {
	return matcher.Matcher.Equal(targetValue)
}

func NewUrlPathMatcher(urlPath *envoy_type_matcher_v3.PathMatcher) (UrlPathMatcher, error) {
	switch urlPath.Rule.(type) {
	case *envoy_type_matcher_v3.PathMatcher_Path:
		m, err := NewStringMatcher(urlPath.Rule.(*envoy_type_matcher_v3.PathMatcher_Path).Path)
		return &DefaultUrlPathMatcher{
			Matcher: m,
		}, err
	default:
		return nil, fmt.Errorf(
			"[NewUrlPathMatcher] not support PathMatcher type found, detail: %v",
			reflect.TypeOf(urlPath.Rule))
	}
}
