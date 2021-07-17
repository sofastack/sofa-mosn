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

package mosn

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"time"

	_ "mosn.io/mosn/pkg/admin/debug"
	_ "mosn.io/mosn/pkg/networkextention/discovery/fsdis"
	_ "mosn.io/mosn/pkg/networkextention/l7/stream/filter/echo"
	_ "mosn.io/mosn/pkg/networkextention/l7/stream/filter/metadata"
	_ "mosn.io/mosn/pkg/networkextention/l7/stream/filter/metadata/unit"

	admin "mosn.io/mosn/pkg/admin/server"
	"mosn.io/mosn/pkg/admin/store"
	v2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/configmanager"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/metrics"
	"mosn.io/mosn/pkg/metrics/shm"
	"mosn.io/mosn/pkg/metrics/sink"
	"mosn.io/mosn/pkg/networkextention/l7/stream"
	"mosn.io/mosn/pkg/server"
	"mosn.io/mosn/pkg/xds/conv"
	"mosn.io/pkg/utils"
)

const (
	DefaultMosnConfigPath string = "/home/admin/mosn/conf/mosn.json"
	DefaultOutFile        string = "/home/admin/logs/mosn/out.log"
	QuitMosnAPI           string = "/quitquitquit"
	Version               string = "0.5.0"
)

// create stream filter factories
func Start() {
	// TODO get config path from envoy cfg or environment variable
	initStreamFilterAndMosnService(DefaultMosnConfigPath)
}

func parseMosnConfig(configpath string) *v2.MOSNConfig {
	// parse MOSNConfig
	c := configmanager.Load(configpath)
	return c
}

func hijackStdAndInjectTimestamp(file string) {
	utils.SetHijackStdPipeline(file, true, true)
	injectTimestamp()
}

func initStreamFilterAndMosnService(configPath string) {

	// hijack standard out to a file
	hijackStdAndInjectTimestamp(DefaultOutFile)

	c := parseMosnConfig(configPath)
	// TODO get index filter by port mapping
	streamFilters := c.Servers[0].Listeners[0].StreamFilters

	// init default log
	s := configmanager.ParseServerConfig(&c.Servers[0])
	sc := server.NewConfig(s)
	server.InitDefaultLogger(sc)
	stream.AddOrUpdateFilter(stream.DefaultFilterChainName, streamFilters)

	// add pprof
	initDebug(c)

	// add admin
	initAdmin(c)

	// add metrics
	initMetrics(c)

	store.StartService(nil)
}

func addQuitMosnAdmin() {
	admin.RegisterAdminHandleFunc(QuitMosnAPI, quitquitquit)
}

func quitquitquit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.DefaultLogger.Alertf("[shim init] [quitquitquit]", "api: %s, error: invalid method: %s", "quitquitquit", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// TODO stop other resource
	store.StopService()

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "mosn service quit success\n")
}

func initDebug(config *v2.MOSNConfig) {
	if config == nil {
		log.DefaultLogger.Fatalf("[shim init] [initDebug] the config is null")
		return
	}

	if config.Debug.StartDebug {
		port := 10002 //default use 10002
		if config.Debug.Port != 0 {
			port = config.Debug.Port
		}
		addr := fmt.Sprintf("0.0.0.0:%d", port)
		s := &http.Server{Addr: addr, Handler: nil}
		store.AddService(s, "pprof", nil, nil)
	}
}

func initAdmin(config *v2.MOSNConfig) {
	if config == nil {
		log.DefaultLogger.Fatalf("[shim init] [initAdmin] the config is null")
		return
	}

	// register quit admin api
	addQuitMosnAdmin()

	// init xds admin api
	conv.InitStats()

	s := admin.Server{}
	// only add admin api service to service store
	s.Start(config)
}

func initMetrics(config *v2.MOSNConfig) {
	if config == nil {
		log.DefaultLogger.Fatalf("[shim init] [initMetrics] the config is null")
		return
	}
	// set mosn metrics flush
	metrics.FlushMosnMetrics = true
	// set version and go version
	metrics.SetVersion(Version)
	metrics.SetGoVersion(runtime.Version())
	initializeMetrics(config.Metrics)
}

// TODO support metrics lazyflush
// https://github.com/mosn/mosn/commit/c5d64decbc50a7de447a1a73b2d4c6948964d9d7#diff-9d72186d24c1bf68ba5309c9a835de62e2559f570d6d85d924ceff86671a60ed
func initializeMetrics(config v2.MetricsConfig) {
	// init shm zone
	if config.ShmZone != "" && config.ShmSize > 0 {
		shm.InitDefaultMetricsZone(config.ShmZone, int(config.ShmSize), store.GetMosnState() != store.Active_Reconfiguring)
	}

	// set metrics package
	statsMatcher := config.StatsMatcher
	metrics.SetStatsMatcher(statsMatcher.RejectAll, statsMatcher.ExclusionLabels, statsMatcher.ExclusionKeys)
	//metrics.SetMetricsFeature(config.FlushMosn, config.LazyFlush)
	// create sinks
	for _, cfg := range config.SinkConfigs {
		_, err := sink.CreateMetricsSink(cfg.Type, cfg.Config)
		// abort
		if err != nil {
			log.DefaultLogger.Errorf("[mosn] [init metrics] %s. %v metrics sink is turned off", err, cfg.Type)
			return
		}
		log.DefaultLogger.Infof("[mosn] [init metrics] create metrics sink: %v", cfg.Type)
	}
}

// injection timestamp to stdout
func injectTimestamp() {
	utils.GoWithRecover(func() {
		for {
			fmt.Println(utils.CacheTime())
			time.Sleep(time.Minute)
		}
	}, nil)
}
