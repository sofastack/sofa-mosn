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

package server

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	gometrics "github.com/rcrowley/go-metrics"
	"mosn.io/mosn/pkg/admin/store"
	v2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/configmanager"
	"mosn.io/mosn/pkg/featuregate"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/metrics"
	"mosn.io/mosn/pkg/metrics/sink/console"
	"mosn.io/mosn/pkg/plugin"
	"mosn.io/mosn/pkg/types"
)

var levelMap = map[string]log.Level{
	"FATAL": log.FATAL,
	"ERROR": log.ERROR,
	"WARN":  log.WARN,
	"INFO":  log.INFO,
	"DEBUG": log.DEBUG,
	"TRACE": log.TRACE,
}

const errMsgFmt = `{
	"error": "%s"
}
`

func help(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	buf.WriteString("support apis:\n")
	for key := range apiHandleFuncStore {
		if key != "/" { // do not show this pages
			buf.WriteString(key)
			buf.WriteRune('\n')
		}
	}
	w.Write(buf.Bytes())
}

func configDump(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, error: invalid method: %s", "config dump", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm()
	if len(r.Form) == 0 {
		buf, _ := configmanager.DumpJSON()
		log.DefaultLogger.Infof("[admin api] [config dump] config dump")
		w.WriteHeader(200)
		w.Write(buf)
		return
	}
	if len(r.Form) > 1 {
		w.WriteHeader(400)
		fmt.Fprintf(w, "only support one parameter")
		return
	}
	handle := func(info interface{}) {
		if info == nil {
			log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, parameters:%v", "config dump", r.Form)
			w.WriteHeader(500)
			msg := fmt.Sprintf(errMsgFmt, "internal error")
			fmt.Fprint(w, msg)
		} else {
			buf, _ := json.MarshalIndent(info, "", " ")
			log.DefaultLogger.Infof("[admin api] [config dump] config dump, parameters:%v", r.Form)
			w.WriteHeader(200)
			w.Write(buf)
		}
	}
	for key, param := range r.Form {
		p := param
		switch key {
		case "mosnconfig":
			configmanager.HandleMOSNConfig(configmanager.CfgTypeMOSN, handle)
		case "allrouters":
			configmanager.HandleMOSNConfig(configmanager.CfgTypeRouter, handle)
		case "allclusters":
			configmanager.HandleMOSNConfig(configmanager.CfgTypeCluster, handle)
		case "alllisteners":
			configmanager.HandleMOSNConfig(configmanager.CfgTypeListener, handle)
		case "router":
			configmanager.HandleMOSNConfig(configmanager.CfgTypeRouter, func(v interface{}) {
				routerInfo, ok := v.(map[string]v2.RouterConfiguration)
				if ok && len(p) > 0 {
					handle(routerInfo[p[0]])
				}
			})

		case "cluster":
			configmanager.HandleMOSNConfig(configmanager.CfgTypeCluster, func(v interface{}) {
				clusterInfo, ok := v.(map[string]v2.Cluster)
				if ok && len(p) > 0 {
					handle(clusterInfo[p[0]])
				}
			})

		case "listener":
			configmanager.HandleMOSNConfig(configmanager.CfgTypeListener, func(v interface{}) {
				listenerInfo, ok := v.(map[string]v2.Listener)
				if ok && len(p) > 0 {
					handle(listenerInfo[p[0]])
				}
			})
		default:
			handle(nil)
		}
	}

}

func envoyConfigDump(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, error: invalid method: %s", "config dump envoy", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm()
	if len(r.Form) == 0 {
		buf := configmanager.EnvoyConfigDump()
		log.DefaultLogger.Infof("[admin api] [config dump envoy] config dump envoy")
		w.WriteHeader(200)
		w.Write(buf)
		return
	}
	if len(r.Form) > 1 {
		w.WriteHeader(400)
		fmt.Fprintf(w, "only support one parameter")
		return
	}
	log.DefaultLogger.Warnf("[admin api] [config dump envoy] r.Form %+v", r.Form)
}

func statsDump(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, error: invalid method: %s", "stats dump", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	log.DefaultLogger.Infof("[admin api]  [stats dump] stats dump")
	r.ParseForm()
	if len(r.Form) == 0 {
		allMetrics := metrics.GetAll()
		w.WriteHeader(200)
		sink := console.NewConsoleSink()
		sink.Flush(w, allMetrics)
		return
	}
	if len(r.Form) > 1 {
		w.WriteHeader(400)
		fmt.Fprintf(w, "only support one parameter")
		return
	}
	key := r.FormValue("key")
	m := metrics.GetMetricsFilter(key)
	if m == nil {
		w.WriteHeader(200)
		fmt.Fprintf(w, "no metrics key: %s", key)
		return
	}
	sink := console.NewConsoleSink()
	sink.Flush(w, []types.Metrics{m})
}

func statsDumpProxyTotal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, error: invalid method: %s", "stats dump", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	log.DefaultLogger.Infof("[admin api]  [stats dump] stats dump")
	w.WriteHeader(http.StatusOK)
	m := metrics.GetProxyTotal()
	all := ""
	if m != nil {
		m.Each(func(key string, i interface{}) {
			switch metric := i.(type) {
			case gometrics.Counter:
				all += fmt.Sprintf("%s:%s\n", key, strconv.FormatInt(metric.Count(), 10))
			case gometrics.Gauge:
				all += fmt.Sprintf("%s:%s\n", key, strconv.FormatInt(metric.Value(), 10))
			case gometrics.Histogram:
				h := metric.Snapshot()
				all += fmt.Sprintf("%s:%s\n", key+"_min", strconv.FormatInt(h.Min(), 10))
				all += fmt.Sprintf("%s:%s\n", key+"_max", strconv.FormatInt(h.Max(), 10))
			}
		})
	}
	w.Write([]byte(all))
}

// update log level
type LogLevelData struct {
	LogPath  string `json:"log_path"`
	LogLevel string `json:"log_level"`
}

func getLoggerInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, error: invalid method: %s", "update log level", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	lg := log.GetErrorLoggersInfo()
	data, _ := json.Marshal(lg)
	w.Write(data)
}

func updateLogLevel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, error: invalid method: %s", "update log level", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, error: read body failed, %v", "update log level", err)
		w.WriteHeader(http.StatusBadRequest)
		msg := fmt.Sprintf(errMsgFmt, "read body error")
		fmt.Fprint(w, msg)
		return
	}
	data := &LogLevelData{}
	if err := json.Unmarshal(body, data); err == nil {
		if level, ok := levelMap[data.LogLevel]; ok {
			if log.UpdateErrorLoggerLevel(data.LogPath, level) {
				log.DefaultLogger.Infof("[admin api] [update log level] update log: %s level as %s", data.LogPath, data.LogLevel)
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "update logger success\n")
				return
			}
		}
	}
	log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, update logger level failed with bad request data: %s", "update log level", string(body))
	w.WriteHeader(http.StatusBadRequest) // 400
	msg := fmt.Sprintf(errMsgFmt, "update logger failed")
	fmt.Fprint(w, msg)
}

// post data:
// loggeer path
func enableLogger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, error: invalid method: %s", "enable logger", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	loggerPath, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, error: read body failed, %v", "enable logger", err)
		w.WriteHeader(http.StatusBadRequest)
		msg := fmt.Sprintf(errMsgFmt, "read body error")
		fmt.Fprint(w, msg)
		return
	}
	if !log.ToggleLogger(string(loggerPath), false) {
		log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, error: enbale %s logger failed", "enable logger", string(loggerPath))
		w.WriteHeader(http.StatusBadRequest) // 400
		msg := fmt.Sprintf(errMsgFmt, "enable logger failed")
		fmt.Fprint(w, msg)
		return
	}
	log.DefaultLogger.Infof("[admin api] [enable logger] enable logger %s", string(loggerPath))
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "enable logger success\n")
}

func disableLogger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, error: invalid method: %s", "disable logger", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	loggerPath, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, error: read body failed, %v", "disable logger", err)
		w.WriteHeader(http.StatusBadRequest)
		msg := fmt.Sprintf(errMsgFmt, "read body error")
		fmt.Fprint(w, msg)
		return
	}
	if !log.ToggleLogger(string(loggerPath), true) {
		log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, error: enbale %s logger failed", "disable logger", string(loggerPath))
		w.WriteHeader(http.StatusBadRequest) // 400
		msg := fmt.Sprintf(errMsgFmt, "disable logger failed")
		fmt.Fprint(w, msg)
		return
	}
	log.DefaultLogger.Infof("[admin api] [disable logger] disable logger %s", string(loggerPath))
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "disable logger success\n")
}

// returns data
// pid=xxx&state=xxx
func getState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, error: invalid method: %s", "get state", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	pid := os.Getpid()
	state := store.GetMosnState()
	msg := fmt.Sprintf("pid=%d&state=%d\n", pid, state)
	fmt.Fprint(w, msg)
}

// http://ip:port/plugin?enable=pluginname
// http://ip:port/plugin?disable=pluginname
// http://ip:port/plugin?status=pluginname
// http://ip:port/plugin?status=all
func pluginApi(w http.ResponseWriter, r *http.Request) {
	log.DefaultLogger.Infof("[admin api] [plugin] url %s", r.URL.RequestURI())
	plugin.AdminApi(w, r)
}

func knownFeatures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, error: invalid method: %s", "known features", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm()
	if len(r.Form) == 0 {
		data, _ := json.MarshalIndent(featuregate.KnownFeatures(), "", " ")
		w.Write(data)
		return
	}
	// support only one feature
	value := r.FormValue("key")
	fmt.Fprintf(w, "%t", featuregate.Enabled(featuregate.Feature(value)))
}

type envResults struct {
	Env      map[string]string `json:"env,omityempty"`
	NotFound []string          `json:"not_found,omityempty"`
}

func getEnv(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s, error: invalid method: %s", "get env", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm()
	values, ok := r.Form["key"]
	if !ok {
		log.DefaultLogger.Alertf(types.ErrorKeyAdmin, "api: %s no env key", "get env")
		w.WriteHeader(http.StatusBadRequest)
		msg := fmt.Sprintf(errMsgFmt, "no env key")
		fmt.Fprint(w, msg)
		return
	}
	results := &envResults{
		Env: make(map[string]string),
	}
	for _, key := range values {
		v, exists := os.LookupEnv(key)
		if !exists {
			results.NotFound = append(results.NotFound, key)
		} else {
			results.Env[key] = v
		}
	}
	data, _ := json.MarshalIndent(results, "", " ")
	w.Write(data)
}
