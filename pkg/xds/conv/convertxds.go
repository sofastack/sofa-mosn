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

package conv

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"time"

	udpa_type_v1 "github.com/cncf/udpa/go/udpa/type/v1"
	xdsapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	xdsv2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	xdsauth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	xdscluster "github.com/envoyproxy/go-control-plane/envoy/api/v2/cluster"
	xdscore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	xdsendpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	xdslistener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	xdsroute "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	xdshttpfault "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/fault/v2"
	xdshttpgzip "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/gzip/v2"
	xdstype "github.com/envoyproxy/go-control-plane/envoy/type"
	xdswellknown "github.com/envoyproxy/go-control-plane/pkg/wellknown"
	jsonp "github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/duration"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/valyala/fasthttp"
	"mosn.io/api"
	v2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/configmanager"
	"mosn.io/mosn/pkg/featuregate"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/protocol"
	"mosn.io/mosn/pkg/router"
	"mosn.io/mosn/pkg/xds/v2/rds"
)

// support network filter list
var supportFilter = map[string]bool{
	xdswellknown.HTTPConnectionManager: true,
	xdswellknown.TCPProxy:              true,
	v2.RPC_PROXY:                       true,
	v2.X_PROXY:                         true,
	v2.MIXER:                           true,
}

// todo add support for rpc_proxy
var httpBaseConfig = map[string]bool{
	xdswellknown.HTTPConnectionManager: true,
}

// istio stream filter names, which is quite different from mosn
const (
	IstioFault       = "envoy.fault"
	IstioRouter      = "envoy.router"
	IstioCors        = "envoy.cors"
	MosnPayloadLimit = "mosn.payload_limit"
)

// todo add streamfilters parse
func ConvertListenerConfig(xdsListener *xdsapi.Listener) *v2.Listener {
	// TODO support all filter
	//if !isSupport(xdsListener) {
	//	return nil
	//}

	if xdsListener == nil {
		return nil
	}

	listenerConfig := &v2.Listener{
		ListenerConfig: v2.ListenerConfig{
			Name:           xdsListener.GetName(),
			BindToPort:     convertBindToPort(xdsListener.GetDeprecatedV1()),
			UseOriginalDst: xdsListener.GetUseOriginalDst().GetValue(),
			Inspector:      true,
			AccessLogs:     convertAccessLogs(xdsListener),
		},
		Addr:                    convertAddress(xdsListener.Address),
		PerConnBufferLimitBytes: xdsListener.GetPerConnectionBufferLimitBytes().GetValue(),
	}

	for _, xl := range xdsListener.GetListenerFilters() {
		if xl.Name == xdswellknown.OriginalDestination {
			listenerConfig.UseOriginalDst = true
		}
	}

	//virtual listener need none filters
	//if listenerConfig.Name == "virtual" || listenerConfig.Name == "virtualOutbound" || listenerConfig.Name == "virtualInbound" {
	//	xdsListener.FilterChains = nil
	//	return listenerConfig
	//}

	listenerConfig.ListenerFilters = convertListenerFilters(xdsListener.GetListenerFilters())

	var rawSelectedFilters *xdslistener.Filter
	listenerConfig.FilterChains, rawSelectedFilters = convertFilterChainsAndGetRawFilter(xdsListener)

	if listenerConfig.FilterChains != nil &&
		len(listenerConfig.FilterChains) == 1 &&
		listenerConfig.FilterChains[0].Filters != nil &&
		rawSelectedFilters != nil {
		listenerConfig.StreamFilters = convertStreamFilters(rawSelectedFilters)
	}

	return listenerConfig
}

func convertListenerFilters(listenerFilter []*xdslistener.ListenerFilter) []v2.Filter {
	if listenerFilter == nil {
		return nil
	}

	filters := make([]v2.Filter, 0)
	for _, filter := range listenerFilter {
		listenerfilter := convertListenerFilter(filter.GetName(), filter.GetTypedConfig())
		if listenerfilter.Type != "" {
			log.DefaultLogger.Debugf("add a new listener filter, %v", listenerfilter.Type)
			filters = append(filters, listenerfilter)
		}
	}

	return filters
}

func convertListenerFilter(name string, s *any.Any) v2.Filter {
	filter := v2.Filter{}
	switch name {
	case v2.ORIGINALDST_LISTENER_FILTER:
		// originaldst filter don't need filter.Config
		filter.Type = name

	default:
		log.DefaultLogger.Errorf("not support %s listener filter.", name)
	}

	return filter
}

func ConvertClustersConfig(xdsClusters []*xdsapi.Cluster) []*v2.Cluster {
	if xdsClusters == nil {
		return nil
	}
	clusters := make([]*v2.Cluster, 0, len(xdsClusters))
	for _, xdsCluster := range xdsClusters {
		cluster := &v2.Cluster{
			Name:                 xdsCluster.GetName(),
			ClusterType:          convertClusterType(xdsCluster.GetType()),
			LbType:               convertLbPolicy(xdsCluster.GetLbPolicy()),
			LBSubSetConfig:       convertLbSubSetConfig(xdsCluster.GetLbSubsetConfig()),
			MaxRequestPerConn:    xdsCluster.GetMaxRequestsPerConnection().GetValue(),
			ConnBufferLimitBytes: xdsCluster.GetPerConnectionBufferLimitBytes().GetValue(),
			HealthCheck:          convertHealthChecks(xdsCluster.GetHealthChecks()),
			CirBreThresholds:     convertCircuitBreakers(xdsCluster.GetCircuitBreakers()),
			ConnectTimeout:       &api.DurationConfig{convertTimeDurPoint2TimeDur(xdsCluster.GetConnectTimeout())},
			//OutlierDetection:     convertOutlierDetection(xdsCluster.GetOutlierDetection()),
			Hosts:    convertClusterHosts(xdsCluster.GetHosts()),
			Spec:     convertSpec(xdsCluster),
			TLS:      convertTLS(xdsCluster.GetTlsContext()),
			LbConfig: convertLbConfig(xdsCluster.LbConfig),
		}

		if ass := xdsCluster.GetLoadAssignment(); ass != nil {
			for _, endpoints := range ass.Endpoints {
				hosts := ConvertEndpointsConfig(endpoints)
				cluster.Hosts = append(cluster.Hosts, hosts...)
			}
		}

		clusters = append(clusters, cluster)
	}

	return clusters
}

// TODO support more LB converter
func convertLbConfig(config interface{}) v2.IsCluster_LbConfig {
	switch config.(type) {
	case *xdsv2.Cluster_LeastRequestLbConfig:
		return &v2.LeastRequestLbConfig{ChoiceCount: config.(*xdsv2.Cluster_LeastRequestLbConfig).ChoiceCount.GetValue()}
	default:
		return nil
	}
}

func ConvertEndpointsConfig(xdsEndpoint *xdsendpoint.LocalityLbEndpoints) []v2.Host {
	if xdsEndpoint == nil {
		return nil
	}
	hosts := make([]v2.Host, 0, len(xdsEndpoint.GetLbEndpoints()))
	for _, xdsHost := range xdsEndpoint.GetLbEndpoints() {
		var address string
		if xdsAddress, ok := xdsHost.GetEndpoint().GetAddress().GetAddress().(*xdscore.Address_SocketAddress); ok {
			if xdsPort, ok := xdsAddress.SocketAddress.GetPortSpecifier().(*xdscore.SocketAddress_PortValue); ok {
				address = fmt.Sprintf("%s:%d", xdsAddress.SocketAddress.GetAddress(), xdsPort.PortValue)
			} else if xdsPort, ok := xdsAddress.SocketAddress.GetPortSpecifier().(*xdscore.SocketAddress_NamedPort); ok {
				address = fmt.Sprintf("%s:%s", xdsAddress.SocketAddress.GetAddress(), xdsPort.NamedPort)
			} else {
				log.DefaultLogger.Warnf("unsupported port type")
				continue
			}

		} else if xdsAddress, ok := xdsHost.GetEndpoint().GetAddress().GetAddress().(*xdscore.Address_Pipe); ok {
			address = xdsAddress.Pipe.GetPath()
		} else {
			log.DefaultLogger.Warnf("unsupported address type")
			continue
		}
		host := v2.Host{
			HostConfig: v2.HostConfig{
				Address: address,
			},
			MetaData: convertMeta(xdsHost.Metadata),
		}

		weight := xdsHost.GetLoadBalancingWeight().GetValue()
		if weight < configmanager.MinHostWeight {
			weight = configmanager.MinHostWeight
		} else if weight > configmanager.MaxHostWeight {
			weight = configmanager.MaxHostWeight
		}
		host.Weight = weight

		hosts = append(hosts, host)
	}
	return hosts
}

// todo: more filter type support
func isSupport(xdsListener *xdsapi.Listener) bool {
	if xdsListener == nil {
		return false
	}
	if xdsListener.Name == "virtual" || xdsListener.Name == "virtualOutbound" || xdsListener.Name == "virtualInbound" {
		return true
	}
	for _, filterChain := range xdsListener.GetFilterChains() {
		for _, filter := range filterChain.GetFilters() {
			if value, ok := supportFilter[filter.GetName()]; !ok || !value {
				return false
			}
		}
	}
	return true
}

func convertBindToPort(xdsDeprecatedV1 *xdsapi.Listener_DeprecatedV1) bool {
	if xdsDeprecatedV1 == nil || xdsDeprecatedV1.GetBindToPort() == nil {
		return true
	}
	return xdsDeprecatedV1.GetBindToPort().GetValue()
}

// todo: more filter config support
func convertAccessLogs(xdsListener *xdsapi.Listener) []v2.AccessLog {
	if xdsListener == nil {
		return nil
	}

	accessLogs := make([]v2.AccessLog, 0)
	for _, xdsFilterChain := range xdsListener.GetFilterChains() {
		for _, xdsFilter := range xdsFilterChain.GetFilters() {
			if value, ok := httpBaseConfig[xdsFilter.GetName()]; ok && value {
				filterConfig := GetHTTPConnectionManager(xdsFilter)

				for _, accConfig := range filterConfig.GetAccessLog() {
					if accConfig.Name == xdswellknown.FileAccessLog {
						als, err := GetAccessLog(accConfig)
						if err != nil {
							log.DefaultLogger.Warnf("[convertxds] [accesslog] conversion is fail %s", err)
							continue
						}
						accessLog := v2.AccessLog{
							Path:   als.GetPath(),
							Format: als.GetFormat(),
						}
						accessLogs = append(accessLogs, accessLog)
					}
				}
			} else if xdsFilter.GetName() == xdswellknown.TCPProxy {
				filterConfig := GetTcpProxy(xdsFilter)
				for _, accConfig := range filterConfig.GetAccessLog() {
					if accConfig.Name == xdswellknown.FileAccessLog {
						als, err := GetAccessLog(accConfig)
						if err != nil {
							log.DefaultLogger.Warnf("[convertxds] [accesslog] conversion is fail %s", err)
							continue
						}
						accessLog := v2.AccessLog{
							Path:   als.GetPath(),
							Format: als.GetFormat(),
						}
						accessLogs = append(accessLogs, accessLog)
					}
				}

			} else if xdsFilter.GetName() == v2.X_PROXY {
				log.DefaultLogger.Warnf("[convertxds] [accesslog] xproxy bugaijinlai")
				//filterConfig := &xdsxproxy.XProxy{}
				//xdsutil.StructToMessage(xdsFilter.GetConfig(), filterConfig)
				//for _, accConfig := range filterConfig.GetAccessLog() {
				//	if accConfig.Name == xdsutil.FileAccessLog {
				//		als := &xdsaccesslog.FileAccessLog{}
				//		xdsutil.StructToMessage(accConfig.GetConfig(), als)
				//		accessLog := v2.AccessLog{
				//			Path:   als.GetPath(),
				//			Format: als.GetFormat(),
				//		}
				//		accessLogs = append(accessLogs, accessLog)
				//	}
				//}
			} else if xdsFilter.GetName() == v2.MIXER {
				// support later
				return nil
			} else {
				log.DefaultLogger.Errorf("unsupported filter config type, filter name: %s", xdsFilter.GetName())
			}
		}
	}
	return accessLogs
}

func convertStreamFilters(networkFilter *xdslistener.Filter) []v2.Filter {
	filters := make([]v2.Filter, 0)
	name := networkFilter.GetName()
	if httpBaseConfig[name] {
		filterConfig := GetHTTPConnectionManager(networkFilter)

		for _, filter := range filterConfig.GetHttpFilters() {
			streamFilter := convertStreamFilter(filter.GetName(), filter.GetTypedConfig())
			if streamFilter.Type != "" {
				log.DefaultLogger.Debugf("add a new stream filter, %v", streamFilter.Type)
				filters = append(filters, streamFilter)
			}
		}
	} else if name == v2.X_PROXY {
		log.DefaultLogger.Warnf("[convertxds] [accesslog] xproxy bugaijinlai")
		//filterConfig := &xdsxproxy.XProxy{}
		//xdsutil.StructToMessage(networkFilter.GetConfig(), filterConfig)
		//for _, filter := range filterConfig.GetStreamFilters() {
		//	streamFilter := convertStreamFilter(filter.GetName(), filter.GetConfig())
		//	filters = append(filters, streamFilter)
		//}
	}
	return filters
}

func convertStreamFilter(name string, s *any.Any) v2.Filter {
	var err error
	if strings.HasSuffix(name, v2.GoPluginStreamFilterSuffix) {
		return convertToPluginStreamFilter(name, s)
	}
	filter := v2.Filter{}
	switch name {
	case v2.MIXER:
		filter.Type = name
		filter.Config, err = convertMixerConfig(s)
		if err != nil {
			log.DefaultLogger.Errorf("convertMixerConfig error: %v", err)
		}
	case v2.FaultStream, xdswellknown.Fault:
		filter.Type = v2.FaultStream
		// istio maybe do not contain this config, but have configs in router
		// in this case, we create a fault inject filter that do nothing
		if s == nil {
			streamFault := &v2.StreamFaultInject{}
			filter.Config, err = makeJsonMap(streamFault)
			if err != nil {
				log.DefaultLogger.Errorf("convert fault inject config error: %v", err)
			}
		} else { // common case
			filter.Config, err = convertStreamFaultInjectConfig(s)
			if err != nil {
				log.DefaultLogger.Errorf("convert fault inject config error: %v", err)
			}
		}
	case v2.Gzip, xdswellknown.Gzip:
		filter.Type = v2.Gzip
		// istio maybe do not contain this config, but have configs in router
		// in this case, we create a gzip filter that do nothing
		if s == nil {
			streamGzip := &v2.StreamGzip{}
			filter.Config, err = makeJsonMap(streamGzip)
			if err != nil {
				log.DefaultLogger.Errorf("convert fault inject config error: %v", err)
			}
		} else { // common case
			filter.Config, err = convertStreamGzipConfig(s)
			if err != nil {
				log.DefaultLogger.Errorf("convert gzip config error: %v", err)
			}
		}
	case MosnPayloadLimit:
		if featuregate.Enabled(featuregate.PayLoadLimitEnable) {
			filter.Type = v2.PayloadLimit
			if s == nil {
				payloadLimitInject := &v2.StreamPayloadLimit{}
				filter.Config, err = makeJsonMap(payloadLimitInject)
				if err != nil {
					log.DefaultLogger.Errorf("convert payload limit config error: %v", err)
				}
			} else {
				//filter.Config, err = convertStreamPayloadLimitConfig(s)
				if err != nil {
					log.DefaultLogger.Errorf("convert payload limit config error: %v", err)
				}
			}
		}
	case v2.IstioStats:
		m, err := convertUdpaTypedStructConfig(s)
		if err != nil {
			log.DefaultLogger.Errorf("convert %s config error: %v", name, err)
		}
		filter.Type = name
		filter.Config = m
	default:
	}

	return filter
}

func convertToPluginStreamFilter(name string, config *any.Any) v2.Filter {
	marshal := jsonp.Marshaler{}
	filter := v2.Filter{}
	buf := &bytes.Buffer{}
	err := marshal.Marshal(buf, config)
	if err != nil || buf.Len() == 0 {
		log.DefaultLogger.Errorf("failed to marshal pb.Any to json, err: %v", err)
		return filter
	}

	type PluginStreamFilterConfig struct {
		GoPluginConfig *v2.StreamFilterGoPluginConfig `json:"go_plugin_config"`
		Config         map[string]interface{}         `json:"config"`
	}
	filterConfig := &PluginStreamFilterConfig{}
	_ = json.Unmarshal(buf.Bytes(), &filterConfig)
	filter.GoPluginConfig = filterConfig.GoPluginConfig
	filter.Config = filterConfig.Config
	filter.Type = name
	return filter
}

//func convertStreamPayloadLimitConfig(s *pstruct.Struct) (map[string]interface{}, error) {
//	payloadLimitConfig := &payloadlimit.PayloadLimit{}
//	if err := conversion.StructToMessage(s, payloadLimitConfig); err != nil {
//		return nil, err
//	}
//	payloadLimitStream := &v2.StreamPayloadLimit{
//		MaxEntitySize: payloadLimitConfig.GetMaxEntitySize(),
//		HttpStatus:    payloadLimitConfig.GetHttpStatus(),
//	}
//	return makeJsonMap(payloadLimitStream)
//}

func convertStreamFaultInjectConfig(s *any.Any) (map[string]interface{}, error) {
	faultConfig := &xdshttpfault.HTTPFault{}
	if err := ptypes.UnmarshalAny(s, faultConfig); err != nil {
		return nil, err
	}

	var fixed_delay time.Duration
	if d := faultConfig.Delay.GetFixedDelay(); d != nil {
		fixed_delay = ConvertDuration(d)
	}

	// convert istio percentage to mosn percent
	delayPercent := convertIstioPercentage(faultConfig.Delay.GetPercentage())
	abortPercent := convertIstioPercentage(faultConfig.Abort.GetPercentage())

	streamFault := &v2.StreamFaultInject{
		Delay: &v2.DelayInject{
			DelayInjectConfig: v2.DelayInjectConfig{
				Percent: delayPercent,
				DelayDurationConfig: api.DurationConfig{
					Duration: fixed_delay,
				},
			},
			Delay: fixed_delay,
		},
		Abort: &v2.AbortInject{
			Percent: abortPercent,
			Status:  int(faultConfig.Abort.GetHttpStatus()),
		},
		UpstreamCluster: faultConfig.UpstreamCluster,
		Headers:         convertHeaders(faultConfig.GetHeaders()),
	}
	return makeJsonMap(streamFault)
}

func convertStreamGzipConfig(s *any.Any) (map[string]interface{}, error) {
	gzipConfig := &xdshttpgzip.Gzip{}
	if err := ptypes.UnmarshalAny(s, gzipConfig); err != nil {
		return nil, err
	}

	// convert istio gzip for mosn
	var minContentLength, level uint32
	var contentType []string
	switch gzipConfig.GetCompressionLevel() {
	case xdshttpgzip.Gzip_CompressionLevel_BEST:
		level = fasthttp.CompressBestCompression
	case xdshttpgzip.Gzip_CompressionLevel_SPEED:
		level = fasthttp.CompressBestSpeed
	default:
		level = fasthttp.CompressDefaultCompression
	}

	// TODO upgrade go-control-plane
	// go-control-plane-0.9.5 should use bellow
	//compressor := gzipConfig.GetCompressor()
	//if compressor != nil {
	//	if compressor.GetContentLength() != nil {
	//		minContentLength = compressor.GetContentLength().GetValue()
	//	}
	//	contentType = compressor.GetContentType()
	//}

	if gzipConfig.GetContentLength() != nil {
		minContentLength = gzipConfig.GetContentLength().GetValue()
	}
	contentType = gzipConfig.GetContentType()

	streamGzip := &v2.StreamGzip{
		GzipLevel:     level,
		ContentLength: minContentLength,
		ContentType:   contentType,
	}
	return makeJsonMap(streamGzip)
}

func convertIstioPercentage(percent *xdstype.FractionalPercent) uint32 {
	if percent == nil {
		return 0
	}
	switch percent.Denominator {
	case xdstype.FractionalPercent_MILLION:
		return percent.Numerator / 10000
	case xdstype.FractionalPercent_TEN_THOUSAND:
		return percent.Numerator / 100
	case xdstype.FractionalPercent_HUNDRED:
		return percent.Numerator
	}
	return percent.Numerator
}

func makeJsonMap(v interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil

}

func convertMixerConfig(s *any.Any) (map[string]interface{}, error) {
	mixerConfig := v2.Mixer{}
	err := ptypes.UnmarshalAny(s, &mixerConfig.HttpClientConfig)
	if err != nil {
		return nil, err
	}

	marshaler := jsonp.Marshaler{}
	str, err := marshaler.MarshalToString(&mixerConfig.HttpClientConfig)
	if err != nil {
		return nil, err
	}

	var config map[string]interface{}
	err = json.Unmarshal([]byte(str), &config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func convertUdpaTypedStructConfig(s *any.Any) (map[string]interface{}, error) {
	conf := udpa_type_v1.TypedStruct{}
	err := ptypes.UnmarshalAny(s, &conf)
	if err != nil {
		return nil, err
	}

	config := map[string]interface{}{}
	if conf.Value == nil || conf.Value.Fields == nil || conf.Value.Fields["configuration"] == nil {
		return config, nil
	}
	jsonpbMarshaler := jsonp.Marshaler{}

	buf := bytes.NewBuffer(nil)
	err = jsonpbMarshaler.Marshal(buf, conf.Value.Fields["configuration"])
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(buf.Bytes(), &config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func convertFilterChainsAndGetRawFilter(xdsListener *xdsapi.Listener) ([]v2.FilterChain, *xdslistener.Filter) {
	if xdsListener == nil {
		return nil, nil
	}

	xdsFilterChains := xdsListener.GetFilterChains()

	if xdsFilterChains == nil {
		return nil, nil
	}

	useOriginalDst := false
	for _, xl := range xdsListener.GetListenerFilters() {
		if xl.Name == xdswellknown.OriginalDestination {
			useOriginalDst = true
		}
	}

	var xdsFilters []*xdslistener.Filter
	var chainMatch string
	var tlsConfigs []v2.TLSConfig

	// todo Only one chain is supported now
	// if listener.type == TCP, HTTPS, TLS, Mongo, Redis, MySQL
	//      len(filterChain) = 1        TCP/Redis...
	// else if listener.type == HTTP, GRPC, UnsupportType
	//		len(filterChain) = 2  TCP & connection_manager
	// if listener.ClustnerIP == nil
	//		filterChain.append BlackHoleCluster
	// Give priority to connection_manager, followed by tcp
	for _, xdsFilterChain := range xdsFilterChains {
		xdsFilters = append(xdsFilters, xdsFilterChain.GetFilters()...)

		//todo Distinguish between multiple filterChainMaths
		if xdsFilterChain.GetFilterChainMatch() != nil {
			chainMatch = xdsFilterChain.GetFilterChainMatch().String()
		}

		// collect tls configs
		if (xdsFilterChain.GetTransportSocket() == nil) || (xdsFilterChain.GetTransportSocket().GetTypedConfig() == nil) {
			continue
		}
		downstreamTLSContext := &xdsauth.DownstreamTlsContext{}
		err := ptypes.UnmarshalAny(xdsFilterChain.GetTransportSocket().GetTypedConfig(), downstreamTLSContext)
		if err != nil {
			log.DefaultLogger.Errorf("failed to unmarshal downstream tls context: %v", err)
			continue
		}
		tlsConfigs = append(tlsConfigs, convertTLS(downstreamTLSContext))
	}

	// A port supports only one protocol
	//todo support more Listener & One Listener support more Protocol
	var oneFilter *xdslistener.Filter
	for _, xdsFilter := range xdsFilters {

		if xdsFilter.Name == xdswellknown.TCPProxy {
			oneFilter = xdsFilter
			if useOriginalDst {
				break
			}
		} else if xdsFilter.Name == xdswellknown.HTTPConnectionManager {
			oneFilter = xdsFilter
			break
		}
	}

	return []v2.FilterChain{{
		FilterChainConfig: v2.FilterChainConfig{
			FilterChainMatch: chainMatch,
			Filters:          convertFilters(oneFilter),
		},
		TLSContexts: tlsConfigs,
	},
	}, oneFilter

}

func convertFilters(xdsFilters *xdslistener.Filter) []v2.Filter {
	if xdsFilters == nil {
		return nil
	}

	filters := make([]v2.Filter, 0)

	filterMaps := convertFilterConfig(xdsFilters)
	for typeKey, configValue := range filterMaps {
		filters = append(filters, v2.Filter{
			typeKey,
			nil,
			configValue,
		})
	}

	return filters
}

func toMap(in interface{}) map[string]interface{} {
	var out map[string]interface{}
	data, _ := json.Marshal(in)
	json.Unmarshal(data, &out)
	return out
}

// TODO: more filter config support
func convertFilterConfig(filter *xdslistener.Filter) map[string]map[string]interface{} {
	if filter == nil {
		return nil
	}

	filtersConfigParsed := make(map[string]map[string]interface{})

	var proxyConfig v2.Proxy
	var routerConfig *v2.RouterConfiguration
	var isRds bool
	name := filter.GetName()
	// todo add xprotocol support
	if name == xdswellknown.HTTPConnectionManager {
		filterConfig := GetHTTPConnectionManager(filter)
		routerConfig, isRds = ConvertRouterConf(filterConfig.GetRds().GetRouteConfigName(), filterConfig.GetRouteConfig())

		proxyConfig = v2.Proxy{
			DownstreamProtocol: string(protocol.Auto),
			UpstreamProtocol:   string(protocol.Auto),
		}
	} else if name == v2.X_PROXY {
		//log.DefaultLogger.Errorf("unsupported filter config, filter name: %s", name)
		//filterConfig := &xdsxproxy.XProxy{}
		//xdsutil.StructToMessage(s, filterConfig)
		//routerConfig, isRds = ConvertRouterConf(filterConfig.GetRds().GetRouteConfigName(), filterConfig.GetRouteConfig())
		//
		//proxyConfig = v2.Proxy{
		//	DownstreamProtocol: string(protocol.Xprotocol),
		//	UpstreamProtocol:   string(protocol.Xprotocol),
		//	ExtendConfig:       convertXProxyExtendConfig(filterConfig),
		//}
		return nil
	} else if name == xdswellknown.TCPProxy {
		filterConfig := GetTcpProxy(filter)
		log.DefaultLogger.Tracef("TCPProxy:filter config = %v,v1-config = %v", filterConfig, filterConfig.GetDeprecatedV1())

		d, err := ptypes.Duration(filterConfig.GetIdleTimeout())
		if err != nil {
			log.DefaultLogger.Infof("[xds] [convert] Idletimeout is nil: %s", name)
		}
		tcpProxyConfig := v2.StreamProxy{
			StatPrefix:         filterConfig.GetStatPrefix(),
			Cluster:            filterConfig.GetCluster(),
			IdleTimeout:        &d,
			MaxConnectAttempts: filterConfig.GetMaxConnectAttempts().GetValue(),
		}
		filtersConfigParsed[v2.TCP_PROXY] = toMap(tcpProxyConfig)

		return filtersConfigParsed
	} else if name == v2.MIXER {
		// support later
		return nil
	} else {
		log.DefaultLogger.Errorf("unsupported filter config, filter name: %s", name)
		return nil
	}

	var routerConfigName string

	// get connection manager filter for rds
	if routerConfig != nil {
		routerConfigName = routerConfig.RouterConfigName
		if isRds {
			rds.AppendRouterName(routerConfigName)
		} else {
			if routersMngIns := router.GetRoutersMangerInstance(); routersMngIns == nil {
				log.DefaultLogger.Errorf("xds AddOrUpdateRouters error: router manager in nil")
			} else {
				if err := routersMngIns.AddOrUpdateRouters(routerConfig); err != nil {
					log.DefaultLogger.Errorf("xds AddOrUpdateRouters error: %v", err)
				}
			}
		}
	} else {
		log.DefaultLogger.Errorf("no router config found, filter name: %s", name)
	}

	// get proxy
	proxyConfig.RouterConfigName = routerConfigName
	filtersConfigParsed[v2.DEFAULT_NETWORK_FILTER] = toMap(proxyConfig)
	return filtersConfigParsed
}

func convertCidrRange(cidr []*xdscore.CidrRange) []v2.CidrRange {
	if cidr == nil {
		return nil
	}
	cidrRanges := make([]v2.CidrRange, 0, len(cidr))
	for _, cidrRange := range cidr {
		cidrRanges = append(cidrRanges, v2.CidrRange{
			Address: cidrRange.GetAddressPrefix(),
			Length:  cidrRange.GetPrefixLen().GetValue(),
		})
	}
	return cidrRanges
}

func ConvertRouterConf(routeConfigName string, xdsRouteConfig *xdsapi.RouteConfiguration) (*v2.RouterConfiguration, bool) {
	if routeConfigName != "" {
		return &v2.RouterConfiguration{
			RouterConfigurationConfig: v2.RouterConfigurationConfig{
				RouterConfigName: routeConfigName,
			},
		}, true
	}

	if xdsRouteConfig == nil {
		return nil, false
	}

	virtualHosts := make([]v2.VirtualHost, 0)

	for _, xdsVirtualHost := range xdsRouteConfig.GetVirtualHosts() {
		virtualHost := v2.VirtualHost{
			Name:    xdsVirtualHost.GetName(),
			Domains: xdsVirtualHost.GetDomains(),
			Routers: convertRoutes(xdsVirtualHost.GetRoutes()),
			//RequireTLS:              xdsVirtualHost.GetRequireTls().String(),
			//VirtualClusters:         convertVirtualClusters(xdsVirtualHost.GetVirtualClusters()),
			RequestHeadersToAdd:     convertHeadersToAdd(xdsVirtualHost.GetRequestHeadersToAdd()),
			ResponseHeadersToAdd:    convertHeadersToAdd(xdsVirtualHost.GetResponseHeadersToAdd()),
			ResponseHeadersToRemove: xdsVirtualHost.GetResponseHeadersToRemove(),
			// TODO: rename the method `convertPerRouteConfig` to `convertPerFilterConfig`
			PerFilterConfig: convertPerRouteConfig(xdsVirtualHost.GetTypedPerFilterConfig()),
		}
		virtualHosts = append(virtualHosts, virtualHost)
	}

	return &v2.RouterConfiguration{
		RouterConfigurationConfig: v2.RouterConfigurationConfig{
			RouterConfigName:        xdsRouteConfig.GetName(),
			RequestHeadersToAdd:     convertHeadersToAdd(xdsRouteConfig.GetRequestHeadersToAdd()),
			ResponseHeadersToAdd:    convertHeadersToAdd(xdsRouteConfig.GetResponseHeadersToAdd()),
			ResponseHeadersToRemove: xdsRouteConfig.GetResponseHeadersToRemove(),
		},
		VirtualHosts: virtualHosts,
	}, false
}

func convertRoutes(xdsRoutes []*xdsroute.Route) []v2.Router {
	if xdsRoutes == nil {
		return nil
	}
	routes := make([]v2.Router, 0, len(xdsRoutes))
	for _, xdsRoute := range xdsRoutes {
		if xdsRouteAction := xdsRoute.GetRoute(); xdsRouteAction != nil {
			route := v2.Router{
				RouterConfig: v2.RouterConfig{
					Match: convertRouteMatch(xdsRoute.GetMatch()),
					Route: convertRouteAction(xdsRouteAction),
					//Decorator: v2.Decorator(xdsRoute.GetDecorator().String()),
					RequestMirrorPolicies: convertMirrorPolicy(xdsRouteAction),
				},
				Metadata: convertMeta(xdsRoute.GetMetadata()),
			}
			route.PerFilterConfig = convertPerRouteConfig(xdsRoute.GetTypedPerFilterConfig())
			routes = append(routes, route)
		} else if xdsRouteAction := xdsRoute.GetRedirect(); xdsRouteAction != nil {
			route := v2.Router{
				RouterConfig: v2.RouterConfig{
					Match:    convertRouteMatch(xdsRoute.GetMatch()),
					Redirect: convertRedirectAction(xdsRouteAction),
					//Decorator: v2.Decorator(xdsRoute.GetDecorator().String()),
				},
				Metadata: convertMeta(xdsRoute.GetMetadata()),
			}
			route.PerFilterConfig = convertPerRouteConfig(xdsRoute.GetTypedPerFilterConfig())
			routes = append(routes, route)
		} else if xdsRouteAction := xdsRoute.GetDirectResponse(); xdsRouteAction != nil {
			route := v2.Router{
				RouterConfig: v2.RouterConfig{
					Match:          convertRouteMatch(xdsRoute.GetMatch()),
					DirectResponse: convertDirectResponseAction(xdsRouteAction),
					//Decorator: v2.Decorator(xdsRoute.GetDecorator().String()),
				},
				Metadata: convertMeta(xdsRoute.GetMetadata()),
			}
			route.PerFilterConfig = convertPerRouteConfig(xdsRoute.GetTypedPerFilterConfig())
			routes = append(routes, route)
		} else {
			log.DefaultLogger.Errorf("unsupported route actin, just Route, Redirect and DirectResponse support yet, ignore this route")
			continue
		}
	}
	return routes
}

func convertPerRouteConfig(xdsPerRouteConfig map[string]*any.Any) map[string]interface{} {
	perRouteConfig := make(map[string]interface{}, 0)

	for key, config := range xdsPerRouteConfig {
		switch key {
		case v2.MIXER:
			// TODO: remove mixer
			serviceConfig, err := convertMixerConfig(config)
			if err != nil {
				log.DefaultLogger.Infof("convertPerRouteConfig[%s] error: %v", v2.MIXER, err)
				continue
			}
			perRouteConfig[key] = serviceConfig
		case v2.FaultStream, IstioFault:
			cfg, err := convertStreamFaultInjectConfig(config)
			if err != nil {
				log.DefaultLogger.Infof("convertPerRouteConfig[%s] error: %v", v2.FaultStream, err)
				continue
			}
			log.DefaultLogger.Debugf("add a fault inject stream filter in router")
			perRouteConfig[v2.FaultStream] = cfg
		case v2.PayloadLimit:
			if featuregate.Enabled(featuregate.PayLoadLimitEnable) {
				//cfg, err := convertStreamPayloadLimitConfig(config)
				//if err != nil {
				//	log.DefaultLogger.Infof("convertPerRouteConfig[%s] error: %v", v2.PayloadLimit, err)
				//	continue
				//}
				//log.DefaultLogger.Debugf("add a payload limit stream filter in router")
				//perRouteConfig[v2.PayloadLimit] = cfg
			}
		default:
			log.DefaultLogger.Warnf("unknown per route config: %s", key)
		}
	}

	return perRouteConfig
}

func convertRouteMatch(xdsRouteMatch *xdsroute.RouteMatch) v2.RouterMatch {
	return v2.RouterMatch{
		Prefix: xdsRouteMatch.GetPrefix(),
		Path:   xdsRouteMatch.GetPath(),
		Regex:  xdsRouteMatch.GetRegex(),
		//CaseSensitive: xdsRouteMatch.GetCaseSensitive().GetValue(),
		//Runtime:       convertRuntime(xdsRouteMatch.GetRuntime()),
		Headers: convertHeaders(xdsRouteMatch.GetHeaders()),
	}
}

/*
 func convertRuntime(xdsRuntime *xdscore.RuntimeUInt32) v2.RuntimeUInt32 {
	 if xdsRuntime == nil {
		 return v2.RuntimeUInt32{}
	 }
	 return v2.RuntimeUInt32{
		 DefaultValue: xdsRuntime.GetDefaultValue(),
		 RuntimeKey:   xdsRuntime.GetRuntimeKey(),
	 }
 }
*/

func convertHeaders(xdsHeaders []*xdsroute.HeaderMatcher) []v2.HeaderMatcher {
	if xdsHeaders == nil {
		return nil
	}
	headerMatchers := make([]v2.HeaderMatcher, 0, len(xdsHeaders))
	for _, xdsHeader := range xdsHeaders {
		headerMatcher := v2.HeaderMatcher{}
		if xdsHeader.GetRegexMatch() != "" {
			headerMatcher.Name = xdsHeader.GetName()
			headerMatcher.Value = xdsHeader.GetRegexMatch()
			headerMatcher.Regex = true
		} else {
			headerMatcher.Name = xdsHeader.GetName()
			headerMatcher.Value = xdsHeader.GetExactMatch()
			headerMatcher.Regex = false
		}

		// as pseudo headers not support when Http1.x upgrade to Http2, change pseudo headers to normal headers
		// this would be fix soon
		if strings.HasPrefix(headerMatcher.Name, ":") {
			headerMatcher.Name = headerMatcher.Name[1:]
		}
		headerMatchers = append(headerMatchers, headerMatcher)
	}
	return headerMatchers
}

func convertMeta(xdsMeta *xdscore.Metadata) api.Metadata {
	if xdsMeta == nil {
		return nil
	}
	meta := make(map[string]string, len(xdsMeta.GetFilterMetadata()))
	for key, value := range xdsMeta.GetFilterMetadata() {
		meta[key] = value.String()
	}
	return meta
}

func convertRouteAction(xdsRouteAction *xdsroute.RouteAction) v2.RouteAction {
	if xdsRouteAction == nil {
		return v2.RouteAction{}
	}
	return v2.RouteAction{
		RouterActionConfig: v2.RouterActionConfig{
			ClusterName:      xdsRouteAction.GetCluster(),
			ClusterHeader:    xdsRouteAction.GetClusterHeader(),
			WeightedClusters: convertWeightedClusters(xdsRouteAction.GetWeightedClusters()),
			HashPolicy:       convertHashPolicy(xdsRouteAction.GetHashPolicy()),
			RetryPolicy:      convertRetryPolicy(xdsRouteAction.GetRetryPolicy()),
			PrefixRewrite:    xdsRouteAction.GetPrefixRewrite(),
			HostRewrite:      xdsRouteAction.GetHostRewrite(),
			AutoHostRewrite:  xdsRouteAction.GetAutoHostRewrite().GetValue(),
			//RequestHeadersToAdd:     convertHeadersToAdd(xdsRouteAction.GetRequestHeadersToAdd()),
			//
			//ResponseHeadersToAdd:    convertHeadersToAdd(xdsRouteAction.GetResponseHeadersToAdd()),
			//ResponseHeadersToRemove: xdsRouteAction.GetResponseHeadersToRemove(),
		},
		MetadataMatch: convertMeta(xdsRouteAction.GetMetadataMatch()),
		Timeout:       convertTimeDurPoint2TimeDur(xdsRouteAction.GetTimeout()),
	}
}

func convertHeadersToAdd(headerValueOption []*xdscore.HeaderValueOption) []*v2.HeaderValueOption {
	if len(headerValueOption) < 1 {
		return nil
	}
	valueOptions := make([]*v2.HeaderValueOption, 0, len(headerValueOption))
	for _, opt := range headerValueOption {
		var isAppend *bool
		if opt.Append != nil {
			appendVal := opt.GetAppend().GetValue()
			isAppend = &appendVal
		}
		valueOptions = append(valueOptions, &v2.HeaderValueOption{
			Header: &v2.HeaderValue{
				Key:   opt.GetHeader().GetKey(),
				Value: opt.GetHeader().GetValue(),
			},
			Append: isAppend,
		})
	}
	return valueOptions
}

func convertTimeDurPoint2TimeDur(duration *duration.Duration) time.Duration {
	if duration == nil {
		return 0
	}
	return ConvertDuration(duration)
}

func convertWeightedClusters(xdsWeightedClusters *xdsroute.WeightedCluster) []v2.WeightedCluster {
	if xdsWeightedClusters == nil {
		return nil
	}
	weightedClusters := make([]v2.WeightedCluster, 0, len(xdsWeightedClusters.GetClusters()))
	for _, cluster := range xdsWeightedClusters.GetClusters() {
		weightedCluster := v2.WeightedCluster{
			Cluster: convertWeightedCluster(cluster),
			//RuntimeKeyPrefix: xdsWeightedClusters.GetRuntimeKeyPrefix(),
		}
		weightedClusters = append(weightedClusters, weightedCluster)
	}
	return weightedClusters
}

func convertHashPolicy(hashPolicy []*xdsroute.RouteAction_HashPolicy) []v2.HashPolicy {
	hpReturn := make([]v2.HashPolicy, 0, len(hashPolicy))
	for _, p := range hashPolicy {
		if header := p.GetHeader(); header != nil {
			hpReturn = append(hpReturn, v2.HashPolicy{
				Header: &v2.HeaderHashPolicy{
					Key: header.HeaderName,
				},
			})

			continue
		}

		if cookieConfig := p.GetCookie(); cookieConfig != nil {
			hpReturn = append(hpReturn, v2.HashPolicy{
				Cookie: &v2.CookieHashPolicy{
					Name: cookieConfig.Name,
					Path: cookieConfig.Path,
					TTL: api.DurationConfig{
						convertTimeDurPoint2TimeDur(cookieConfig.Ttl),
					},
				},
			})

			continue
		}

		if ip := p.GetConnectionProperties(); ip != nil {
			hpReturn = append(hpReturn, v2.HashPolicy{
				SourceIP: &v2.SourceIPHashPolicy{},
			})

			continue
		}
	}

	return hpReturn
}

func convertWeightedCluster(xdsWeightedCluster *xdsroute.WeightedCluster_ClusterWeight) v2.ClusterWeight {
	if xdsWeightedCluster == nil {
		return v2.ClusterWeight{}
	}
	return v2.ClusterWeight{
		ClusterWeightConfig: v2.ClusterWeightConfig{
			Name:   xdsWeightedCluster.GetName(),
			Weight: xdsWeightedCluster.GetWeight().GetValue(),
		},
		MetadataMatch: convertMeta(xdsWeightedCluster.GetMetadataMatch()),
	}
}

func convertRetryPolicy(xdsRetryPolicy *xdsroute.RetryPolicy) *v2.RetryPolicy {
	if xdsRetryPolicy == nil {
		return &v2.RetryPolicy{}
	}
	return &v2.RetryPolicy{
		RetryPolicyConfig: v2.RetryPolicyConfig{
			RetryOn:    len(xdsRetryPolicy.GetRetryOn()) > 0,
			NumRetries: xdsRetryPolicy.GetNumRetries().GetValue(),
		},
		RetryTimeout: convertTimeDurPoint2TimeDur(xdsRetryPolicy.GetPerTryTimeout()),
	}
}

func convertRedirectAction(xdsRedirectAction *xdsroute.RedirectAction) *v2.RedirectAction {
	if xdsRedirectAction == nil {
		return nil
	}
	return &v2.RedirectAction{
		SchemeRedirect: xdsRedirectAction.GetSchemeRedirect(),
		HostRedirect:   xdsRedirectAction.GetHostRedirect(),
		PathRedirect:   xdsRedirectAction.GetPathRedirect(),
		ResponseCode:   int(xdsRedirectAction.GetResponseCode()),
	}
}

func convertDirectResponseAction(xdsDirectResponseAction *xdsroute.DirectResponseAction) *v2.DirectResponseAction {
	if xdsDirectResponseAction == nil {
		return nil
	}

	var body string
	if rawData := xdsDirectResponseAction.GetBody(); rawData != nil {
		body = rawData.GetInlineString()
	}

	return &v2.DirectResponseAction{
		StatusCode: int(xdsDirectResponseAction.GetStatus()),
		Body:       body,
	}
}

/*
 func convertVirtualClusters(xdsVirtualClusters []*xdsroute.VirtualCluster) []v2.VirtualCluster {
	 if xdsVirtualClusters == nil {
		 return nil
	 }
	 virtualClusters := make([]v2.VirtualCluster, 0, len(xdsVirtualClusters))
	 for _, xdsVirtualCluster := range xdsVirtualClusters {
		 virtualCluster := v2.VirtualCluster{
			 Pattern: xdsVirtualCluster.GetPattern(),
			 Name:    xdsVirtualCluster.GetName(),
			 Method:  xdsVirtualCluster.GetMethod().String(),
		 }
		 virtualClusters = append(virtualClusters, virtualCluster)
	 }
	 return virtualClusters
 }
*/

func convertAddress(xdsAddress *xdscore.Address) net.Addr {
	if xdsAddress == nil {
		return nil
	}
	var address string
	if addr, ok := xdsAddress.GetAddress().(*xdscore.Address_SocketAddress); ok {
		if xdsPort, ok := addr.SocketAddress.GetPortSpecifier().(*xdscore.SocketAddress_PortValue); ok {
			address = fmt.Sprintf("%s:%d", addr.SocketAddress.GetAddress(), xdsPort.PortValue)
		} else {
			log.DefaultLogger.Warnf("only port value supported")
			return nil
		}
	} else {
		log.DefaultLogger.Errorf("only SocketAddress supported")
		return nil
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		log.DefaultLogger.Errorf("Invalid address: %v", err)
		return nil
	}
	return tcpAddr
}

func convertClusterType(xdsClusterType xdsapi.Cluster_DiscoveryType) v2.ClusterType {
	switch xdsClusterType {
	case xdsapi.Cluster_STATIC:
		return v2.SIMPLE_CLUSTER
	case xdsapi.Cluster_STRICT_DNS:
		return v2.STRICT_DNS_CLUSTER
	case xdsapi.Cluster_LOGICAL_DNS:
	case xdsapi.Cluster_EDS:
		return v2.EDS_CLUSTER
	case xdsapi.Cluster_ORIGINAL_DST:
		return v2.ORIGINALDST_CLUSTER
	}
	return v2.SIMPLE_CLUSTER
}

func convertLbPolicy(xdsLbPolicy xdsapi.Cluster_LbPolicy) v2.LbType {
	switch xdsLbPolicy {
	case xdsapi.Cluster_ROUND_ROBIN:
		return v2.LB_ROUNDROBIN
	case xdsapi.Cluster_LEAST_REQUEST:
		return v2.LB_LEAST_REQUEST
	case xdsapi.Cluster_RANDOM:
		return v2.LB_RANDOM
	case xdsapi.Cluster_ORIGINAL_DST_LB:
		return v2.LB_ORIGINAL_DST
	case xdsapi.Cluster_MAGLEV:
		return v2.LB_MAGLEV
	case xdsapi.Cluster_RING_HASH:
		return v2.LB_MAGLEV
	}
	return v2.LB_RANDOM
}

func convertLbSubSetConfig(xdsLbSubsetConfig *xdsapi.Cluster_LbSubsetConfig) v2.LBSubsetConfig {
	if xdsLbSubsetConfig == nil {
		return v2.LBSubsetConfig{}
	}
	return v2.LBSubsetConfig{
		FallBackPolicy:  uint8(xdsLbSubsetConfig.GetFallbackPolicy()),
		DefaultSubset:   convertTypesStruct(xdsLbSubsetConfig.GetDefaultSubset()),
		SubsetSelectors: convertSubsetSelectors(xdsLbSubsetConfig.GetSubsetSelectors()),
	}
}

func convertTypesStruct(s *structpb.Struct) map[string]string {
	if s == nil {
		return nil
	}
	meta := make(map[string]string, len(s.GetFields()))
	for key, value := range s.GetFields() {
		meta[key] = value.String()
	}
	return meta
}

func convertSubsetSelectors(xdsSubsetSelectors []*xdsapi.Cluster_LbSubsetConfig_LbSubsetSelector) [][]string {
	if xdsSubsetSelectors == nil {
		return nil
	}
	subsetSelectors := make([][]string, 0, len(xdsSubsetSelectors))
	for _, xdsSubsetSelector := range xdsSubsetSelectors {
		subsetSelectors = append(subsetSelectors, xdsSubsetSelector.GetKeys())
	}
	return subsetSelectors
}

func convertHealthChecks(xdsHealthChecks []*xdscore.HealthCheck) v2.HealthCheck {
	if xdsHealthChecks == nil || len(xdsHealthChecks) == 0 || xdsHealthChecks[0] == nil {
		return v2.HealthCheck{}
	}

	return v2.HealthCheck{
		HealthCheckConfig: v2.HealthCheckConfig{
			HealthyThreshold:   xdsHealthChecks[0].GetHealthyThreshold().GetValue(),
			UnhealthyThreshold: xdsHealthChecks[0].GetUnhealthyThreshold().GetValue(),
		},
		Timeout:        ConvertDuration(xdsHealthChecks[0].GetTimeout()),
		Interval:       ConvertDuration(xdsHealthChecks[0].GetInterval()),
		IntervalJitter: ConvertDuration(xdsHealthChecks[0].GetIntervalJitter()),
	}
}

func convertCircuitBreakers(xdsCircuitBreaker *xdscluster.CircuitBreakers) v2.CircuitBreakers {
	if xdsCircuitBreaker == nil || proto.Size(xdsCircuitBreaker) == 0 {
		return v2.CircuitBreakers{}
	}
	thresholds := make([]v2.Thresholds, 0, len(xdsCircuitBreaker.GetThresholds()))
	for _, xdsThreshold := range xdsCircuitBreaker.GetThresholds() {
		threshold := v2.Thresholds{
			MaxConnections:     xdsThreshold.GetMaxConnections().GetValue(),
			MaxPendingRequests: xdsThreshold.GetMaxPendingRequests().GetValue(),
			MaxRequests:        xdsThreshold.GetMaxRequests().GetValue(),
			MaxRetries:         xdsThreshold.GetMaxRetries().GetValue(),
		}
		thresholds = append(thresholds, threshold)
	}
	return v2.CircuitBreakers{
		Thresholds: thresholds,
	}
}

/*
 func convertOutlierDetection(xdsOutlierDetection *xdscluster.OutlierDetection) v2.OutlierDetection {
	 if xdsOutlierDetection == nil || xdsOutlierDetection.Size() == 0 {
		 return v2.OutlierDetection{}
	 }
	 return v2.OutlierDetection{
		 Consecutive5xx:                     xdsOutlierDetection.GetConsecutive_5Xx().GetValue(),
		 Interval:                           convertDuration(xdsOutlierDetection.GetInterval()),
		 BaseEjectionTime:                   convertDuration(xdsOutlierDetection.GetBaseEjectionTime()),
		 MaxEjectionPercent:                 xdsOutlierDetection.GetMaxEjectionPercent().GetValue(),
		 ConsecutiveGatewayFailure:          xdsOutlierDetection.GetEnforcingConsecutive_5Xx().GetValue(),
		 EnforcingConsecutive5xx:            xdsOutlierDetection.GetConsecutive_5Xx().GetValue(),
		 EnforcingConsecutiveGatewayFailure: xdsOutlierDetection.GetEnforcingConsecutiveGatewayFailure().GetValue(),
		 EnforcingSuccessRate:               xdsOutlierDetection.GetEnforcingSuccessRate().GetValue(),
		 SuccessRateMinimumHosts:            xdsOutlierDetection.GetSuccessRateMinimumHosts().GetValue(),
		 SuccessRateRequestVolume:           xdsOutlierDetection.GetSuccessRateRequestVolume().GetValue(),
		 SuccessRateStdevFactor:             xdsOutlierDetection.GetSuccessRateStdevFactor().GetValue(),
	 }
 }
*/

func convertSpec(xdsCluster *xdsapi.Cluster) v2.ClusterSpecInfo {
	if xdsCluster == nil || xdsCluster.GetEdsClusterConfig() == nil {
		return v2.ClusterSpecInfo{}
	}
	specs := make([]v2.SubscribeSpec, 0, 1)
	spec := v2.SubscribeSpec{
		ServiceName: xdsCluster.GetEdsClusterConfig().GetServiceName(),
	}
	specs = append(specs, spec)
	return v2.ClusterSpecInfo{
		Subscribes: specs,
	}
}

func convertClusterHosts(xdsHosts []*xdscore.Address) []v2.Host {
	if xdsHosts == nil {
		return nil
	}
	hostsWithMetaData := make([]v2.Host, 0, len(xdsHosts))
	for _, xdsHost := range xdsHosts {
		hostWithMetaData := v2.Host{
			HostConfig: v2.HostConfig{
				Address: convertAddress(xdsHost).String(),
			},
		}
		hostsWithMetaData = append(hostsWithMetaData, hostWithMetaData)
	}
	return hostsWithMetaData
}

func convertTLS(xdsTLSContext interface{}) v2.TLSConfig {
	var config v2.TLSConfig
	var isDownstream bool
	var isSdsMode bool
	var common *xdsauth.CommonTlsContext

	if xdsTLSContext == nil {
		return config
	}
	if context, ok := xdsTLSContext.(*xdsauth.DownstreamTlsContext); ok {
		if context.GetRequireClientCertificate() != nil {
			config.VerifyClient = context.GetRequireClientCertificate().GetValue()
		}
		common = context.GetCommonTlsContext()
		isDownstream = true
	} else if context, ok := xdsTLSContext.(*xdsauth.UpstreamTlsContext); ok {
		config.ServerName = context.GetSni()
		common = context.GetCommonTlsContext()
		isDownstream = false
	}
	if common == nil {
		return config
	}
	// Currently only a single certificate is supported
	if common.GetTlsCertificates() != nil {
		for _, cert := range common.GetTlsCertificates() {
			if cert.GetCertificateChain() != nil && cert.GetPrivateKey() != nil {
				// use GetFilename to get the cert's path
				config.CertChain = cert.GetCertificateChain().GetFilename()
				config.PrivateKey = cert.GetPrivateKey().GetFilename()
			}
		}
	} else if tlsCertSdsConfig := common.GetTlsCertificateSdsSecretConfigs(); tlsCertSdsConfig != nil && len(tlsCertSdsConfig) > 0 {
		isSdsMode = true
		if validationContext, ok := common.GetValidationContextType().(*xdsauth.CommonTlsContext_CombinedValidationContext); ok {
			config.SdsConfig = &v2.SdsConfig{
				CertificateConfig: &v2.SecretConfigWrapper{ConfigDeprecated: tlsCertSdsConfig[0]},
				ValidationConfig:  &v2.SecretConfigWrapper{ConfigDeprecated: validationContext.CombinedValidationContext.GetValidationContextSdsSecretConfig()},
			}
		}
	}

	if common.GetValidationContext() != nil && common.GetValidationContext().GetTrustedCa() != nil {
		config.CACert = common.GetValidationContext().GetTrustedCa().String()
	}
	if common.GetAlpnProtocols() != nil {
		config.ALPN = strings.Join(common.GetAlpnProtocols(), ",")
	}
	param := common.GetTlsParams()
	if param != nil {
		if param.GetCipherSuites() != nil {
			config.CipherSuites = strings.Join(param.GetCipherSuites(), ":")
		}
		if param.GetEcdhCurves() != nil {
			config.EcdhCurves = strings.Join(param.GetEcdhCurves(), ",")
		}
		config.MinVersion = xdsauth.TlsParameters_TlsProtocol_name[int32(param.GetTlsMinimumProtocolVersion())]
		config.MaxVersion = xdsauth.TlsParameters_TlsProtocol_name[int32(param.GetTlsMaximumProtocolVersion())]
	}

	if !isSdsMode && isDownstream && (config.CertChain == "" || config.PrivateKey == "") {
		log.DefaultLogger.Errorf("tls_certificates are required in downstream tls_context")
		config.Status = false
		return config
	}

	config.Status = true
	return config
}

func convertMirrorPolicy(xdsRouteAction *xdsroute.RouteAction) *v2.RequestMirrorPolicy {
	if len(xdsRouteAction.GetRequestMirrorPolicies()) > 0 {
		return &v2.RequestMirrorPolicy{
			Cluster: xdsRouteAction.GetRequestMirrorPolicies()[0].GetCluster(),
			Percent: convertRuntimePercentage(xdsRouteAction.GetRequestMirrorPolicies()[0].GetRuntimeFraction()),
		}
	}

	return nil
}

func convertRuntimePercentage(percent *xdscore.RuntimeFractionalPercent) uint32 {
	if percent == nil {
		return 0
	}

	v := percent.GetDefaultValue()
	switch v.GetDenominator() {
	case xdstype.FractionalPercent_MILLION:
		return v.Numerator / 10000
	case xdstype.FractionalPercent_TEN_THOUSAND:
		return v.Numerator / 100
	case xdstype.FractionalPercent_HUNDRED:
		return v.Numerator
	}
	return v.Numerator
}
