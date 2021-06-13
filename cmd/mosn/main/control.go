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

package main

import (
	_ "net/http/pprof"
	"os"
	"runtime"

	"github.com/urfave/cli"
	"mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/configmanager"
	"mosn.io/mosn/pkg/featuregate"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/metrics"
	"mosn.io/mosn/pkg/mosn"
	"mosn.io/mosn/pkg/types"
)

var (
	flagToMosnLogLevel = map[string]string{
		"trace":    "TRACE",
		"debug":    "DEBUG",
		"info":     "INFO",
		"warning":  "WARN",
		"error":    "ERROR",
		"critical": "FATAL",
		"off":      "OFF",
	}

	cmdStart = cli.Command{
		Name:  "start",
		Usage: "start mosn proxy",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "config, c",
				Usage:  "Load configuration from `FILE`",
				EnvVar: "MOSN_CONFIG",
				Value:  "configs/mosn_config.json",
			}, cli.StringFlag{
				Name:   "service-cluster, s",
				Usage:  "sidecar service cluster",
				EnvVar: "SERVICE_CLUSTER",
			}, cli.StringFlag{
				Name:   "service-node, n",
				Usage:  "sidecar service node",
				EnvVar: "SERVICE_NODE",
			}, cli.StringFlag{
				Name:   "service-type, p",
				Usage:  "sidecar service type",
				EnvVar: "SERVICE_TYPE",
			}, cli.StringSliceFlag{
				Name:   "service-meta, sm",
				Usage:  "sidecar service metadata",
				EnvVar: "SERVICE_META",
			}, cli.StringSliceFlag{
				Name:   "service-lables, sl",
				Usage:  "sidecar service metadata labels",
				EnvVar: "SERVICE_LAB",
			}, cli.StringSliceFlag{
				Name:   "cluster-domain, domain",
				Usage:  "sidecar service metadata labels",
				EnvVar: "CLUSTER_DOMAIN",
			}, cli.StringFlag{
				Name:   "feature-gates, f",
				Usage:  "config feature gates",
				EnvVar: "FEATURE_GATES",
			}, cli.StringFlag{
				Name:   "pod-namespace, pns",
				Usage:  "mosn pod namespaces",
				EnvVar: "POD_NAMESPACE",
			}, cli.StringFlag{
				Name:   "pod-name, pn",
				Usage:  "mosn pod name",
				EnvVar: "POD_NAME",
			}, cli.StringFlag{
				Name:   "pod-ip, pi",
				Usage:  "mosn pod ip",
				EnvVar: "POD_IP",
			}, cli.StringFlag{
				Name:   "log-level, l",
				Usage:  "mosn log level, trace|debug|info|warning|error|critical|off",
				EnvVar: "LOG_LEVEL",
			}, cli.StringFlag{
				Name:  "log-format, lf",
				Usage: "mosn log format, currently useless",
			}, cli.StringSliceFlag{
				Name:  "component-log-level, lc",
				Usage: "mosn component format, currently useless",
			}, cli.StringFlag{
				Name:  "local-address-ip-version",
				Usage: "ip version, v4 or v6, currently useless",
			}, cli.IntFlag{
				Name:  "restart-epoch",
				Usage: "eporch to restart, align to Istio startup params, currently useless",
			}, cli.IntFlag{
				Name:  "drain-time-s",
				Usage: "seconds to drain, align to Istio startup params, currently useless",
			}, cli.StringFlag{
				Name:  "parent-shutdown-time-s",
				Usage: "parent shutdown time seconds, align to Istio startup params, currently useless",
			}, cli.IntFlag{
				Name:  "max-obj-name-len",
				Usage: "object name limit, align to Istio startup params, currently useless",
			}, cli.IntFlag{
				Name:  "concurrency",
				Usage: "concurrency, align to Istio startup params, currently useless",
			}, cli.IntFlag{
				Name:  "log-format-prefix-with-location",
				Usage: "log-format-prefix-with-location, align to Istio startup params, currently useless",
			}, cli.IntFlag{
				Name:  "bootstrap-version", // 目前没用，就是 v3
				Usage: "API version to parse the bootstrap config as (e.g. 3). If unset, all known versions will be attempted",
			}, cli.StringFlag{
				Name:  "drain-strategy", // 目前没用，immediate
				Usage: "immediate",
			}, cli.BoolTFlag{
				Name:  "disable-hot-restart", // 目前没用，
				Usage: "disable-hot-restart",
			},
		},
		Action: func(c *cli.Context) error {
			stm := mosn.NewStageManager(c, c.String("config"))
			// if needs featuregate init in parameter stage or init stage
			// append a new stage and called featuregate.ExecuteInitFunc(keys...)
			// parameter parsed registered
			stm.AppendParamsParsedStage(DefaultParamsParsed)
			// initial registerd
			stm.AppendInitStage(mosn.DefaultInitStage)
			stm.AppendInitStage(func(_ *v2.MOSNConfig) {
				// set version and go version
				metrics.SetVersion(Version)
				metrics.SetGoVersion(runtime.Version())
			})
			// pre-startup
			stm.AppendPreStartStage(mosn.DefaultPreStartStage) // called finally stage by default
			// startup
			stm.AppendStartStage(mosn.DefaultStartStage)
			// execute all runs
			stm.Run()

			// if functions needs to be called after mosn start, add here.

			// wait mosn finished
			stm.WaitFinish()
			return nil

		},
	}

	cmdStop = cli.Command{
		Name:  "stop",
		Usage: "stop mosn proxy",
		Action: func(c *cli.Context) error {
			return nil
		},
	}

	cmdReload = cli.Command{
		Name:  "reload",
		Usage: "reconfiguration",
		Action: func(c *cli.Context) error {
			return nil
		},
	}
)

func DefaultParamsParsed(c *cli.Context) {
	// log level control
	flagLogLevel := c.String("log-level")
	if mosnLogLevel, ok := flagToMosnLogLevel[flagLogLevel]; ok {
		if mosnLogLevel == "OFF" {
			log.GetErrorLoggerManagerInstance().Disable()
		} else {
			log.GetErrorLoggerManagerInstance().SetLogLevelControl(configmanager.ParseLogLevel(mosnLogLevel))
		}
	}
	// set feature gates
	err := featuregate.Set(c.String("feature-gates"))
	if err != nil {
		log.StartLogger.Infof("[mosn] [start] parse feature-gates flag fail : %+v", err)
		os.Exit(1)
	}
	// istio parameters
	serviceCluster := c.String("service-cluster")
	serviceNode := c.String("service-node")
	serviceType := c.String("service-type")
	serviceMeta := c.StringSlice("service-meta")
	metaLabels := c.StringSlice("service-lables")
	clusterDomain := c.String("cluster-domain")
	podName := c.String("pod-name")
	podNamespace := c.String("pod-namespace")
	podIp := c.String("pod-ip")
	if serviceNode != "" {
		types.InitXdsFlags(serviceCluster, serviceNode, serviceMeta, metaLabels)
	} else {
		if types.IsApplicationNodeType(serviceType) {
			sn := podName + "." + podNamespace
			serviceNode := serviceType + "~" + podIp + "~" + sn + "~" + clusterDomain
			types.InitXdsFlags(serviceCluster, serviceNode, serviceMeta, metaLabels)
		} else {
			log.StartLogger.Infof("[mosn] [start] xds service type must be sidecar or router")
		}
	}
}
