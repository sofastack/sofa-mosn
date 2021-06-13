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
	"regexp"
	"testing"
)

func TestStringMatcher(t *testing.T) {
	var matcher StringMatcher
	targetString := "test_string_matcher"

	//	StringMatcherExact
	matcher = &ExactStringMatcher{ExactMatch: "test_string_matcher"}
	if !matcher.Equal(targetString) {
		t.Error("ExactStringMatcher failed")
		return
	}
	matcher = &ExactStringMatcher{ExactMatch: "TEST_string_matcher"}
	if matcher.Equal(targetString) {
		t.Error("ExactStringMatcher failed")
		return
	}

	//	StringMatcherPrefix
	matcher = &PrefixStringMatcher{PrefixMatch: "test_"}
	if !matcher.Equal(targetString) {
		t.Error("PrefixStringMatcher failed")
		return
	}
	matcher = &PrefixStringMatcher{PrefixMatch: "TEST_"}
	if matcher.Equal(targetString) {
		t.Error("PrefixStringMatcher failed")
		return
	}

	//	StringMatcherSuffix
	matcher = &SuffixStringMatcher{SuffixMatch: "_matcher"}
	if !matcher.Equal(targetString) {
		t.Error("SuffixStringMatcher failed")
		return
	}
	matcher = &SuffixStringMatcher{SuffixMatch: "_MATCHER"}
	if matcher.Equal(targetString) {
		t.Error("SuffixStringMatcher failed")
		return
	}

	//	StringMatcherRegex
	matcher = &RegexStringMatcher{RegexMatch: regexp.MustCompile("test_")}
	if !matcher.Equal(targetString) {
		t.Error("RegexStringMatcher failed")
		return
	}
	// test case-insensitive
	matcher = &RegexStringMatcher{RegexMatch: regexp.MustCompile("(?i)TEST_")}
	if !matcher.Equal(targetString) {
		t.Error("RegexStringMatcher failed")
		return
	}
}

func TestHeaderMatcher(t *testing.T) {
	var matcher HeaderMatcher

	// HeaderMatcherPresentMatch
	matcher = &HeaderMatcherPresentMatch{PresentMatch: true}
	if !matcher.Equal("") {
		t.Error("HeaderMatcherPresentMatch failed")
		return
	}

	// HeaderMatcherRangeMatch
	matcher = &HeaderMatcherRangeMatch{
		Start: 0,
		End:   100,
	}
	if !matcher.Equal("0") {
		t.Error("HeaderMatcherRangeMatch failed")
		return
	}
	if !matcher.Equal("99") {
		t.Error("HeaderMatcherRangeMatch failed")
		return
	}
	if matcher.Equal("100") {
		t.Error("HeaderMatcherRangeMatch failed")
		return
	}
	if matcher.Equal("aaa") {
		t.Error("HeaderMatcherRangeMatch failed")
		return
	}
}
