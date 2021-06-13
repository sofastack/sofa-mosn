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
	"encoding/json"
	"io/ioutil"
	"testing"
)

func TestParseRbacFilterConfig(t *testing.T) {
	// config
	conf, err := ioutil.ReadFile("./test_conf/principal-or.json")
	if err != nil {
		t.Error("TestParseRbacFilterConfig failed")
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(conf, &cfg); err != nil {
		t.Error("TestParseRbacFilterConfig failed")
	}

	rbacConfig, err := ParseRbacFilterConfig(cfg)
	if err != nil {
		t.Error("TestParseRbacFilterConfig failed")
	}

	if rbacConfig.Version != "app1_version_1" {
		t.Error("TestParseRbacFilterConfig failed")
	}

	if len(rbacConfig.Rules.Policies) != 1 {
		t.Error("TestParseRbacFilterConfig failed")
	}

	if len(rbacConfig.ShadowRules.Policies) != 0 {
		t.Error("TestParseRbacFilterConfig failed")
	}
}
