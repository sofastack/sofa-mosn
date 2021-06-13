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

package v3

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	envoy_config_bootstrap_v3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	envoy_config_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	httpconnectionmanagerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_extensions_transport_sockets_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	envoy_service_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"mosn.io/mosn/pkg/featuregate"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/xds/v3/conv"
)

//  Init parsed ds and clusters config for xds
func (c *XDSConfig) Init(dynamicResources *envoy_config_bootstrap_v3.Bootstrap_DynamicResources, staticResources *envoy_config_bootstrap_v3.Bootstrap_StaticResources) error {
	err := c.loadClusters(staticResources)
	if err != nil {
		return err
	}
	if err = loadStaticResources(staticResources); err != nil {
		return err
	}
	err = c.loadADSConfig(dynamicResources)
	if err != nil {
		return err
	}
	return nil
}

const (
	connectionManager = "envoy.filters.network.http_connection_manager"
)

var (
	typeFactoryMapping = map[string]func() proto.Message{
		connectionManager: func() proto.Message { return new(httpconnectionmanagerv3.HttpConnectionManager) },
	}
)

func loadStaticResources(staticResources *envoy_config_bootstrap_v3.Bootstrap_StaticResources) (err error) {
	var clusters []*envoy_config_cluster_v3.Cluster
	if cs := staticResources.Clusters; cs != nil && len(cs) > 0 {
		clusters = make([]*envoy_config_cluster_v3.Cluster, 0, len(cs))
		for _, c := range cs {
			if name := c.Name; name == "zipkin" {
				continue
			}
			clusters = append(clusters, c)
		}
	}
	if clusters != nil && len(clusters) > 0 {
		conv.ConvertUpdateClusters(clusters)
	}
	var listeners []*listenerv3.Listener
	if listeners = staticResources.Listeners; listeners == nil || len(listeners) <= 0 {
		return
	}
	var routes []*routev3.RouteConfiguration
	if listeners, routes, err = adaptStaticListenersToDynamic(listeners); err != nil {
		return
	}
	conv.ConvertAddOrUpdateListeners(listeners)
	conv.ConvertAddOrUpdateRouters(routes)
	return
}

func adaptStaticListenersToDynamic(listeners []*listenerv3.Listener) (
	[]*listenerv3.Listener, []*routev3.RouteConfiguration, error) {

	if listeners == nil || len(listeners) <= 0 {
		return nil, nil, nil
	}

	collector := &routerCollector{routes: make([]*routev3.RouteConfiguration, 0, len(listeners))}
	for _, listener := range listeners {
		port := adaptListenerName(listener)
		if err := collector.collectRoute(listener, port); err != nil {
			return nil, nil, err
		}
	}

	return listeners, collector.routes, nil
}

func adaptListenerName(listener *listenerv3.Listener) (port uint32) {
	if name := listener.Name; len(name) > 0 {
		return
	}
	if address := listener.Address.GetSocketAddress(); address == nil {
		return
	} else if port = address.GetPortValue(); port == 0 {
		return
	}
	listener.Name = fmt.Sprintf("127.0.0.1_%d", port)
	return
}

type routerCollector struct {
	routes []*routev3.RouteConfiguration
}

func (rc *routerCollector) collectRoute(listener *listenerv3.Listener, port uint32) (err error) {
	var filterChains []*listenerv3.FilterChain
	if filterChains = listener.FilterChains; filterChains == nil || len(filterChains) <= 0 {
		return nil
	}
	for _, filterChain := range filterChains {
		var filters []*listenerv3.Filter
		if filters = filterChain.Filters; filters == nil || len(filters) <= 0 {
			continue
		}
		for _, filter := range filters {
			if factory, exist := typeFactoryMapping[filter.Name]; exist {
				typedConfig := factory()
				if err = ptypes.UnmarshalAny(filter.GetTypedConfig(), typedConfig); err != nil {
					return
				}
				switch typedConfig.(type) {
				case *httpconnectionmanagerv3.HttpConnectionManager:
					manager := typedConfig.(*httpconnectionmanagerv3.HttpConnectionManager)
					if routerConfig := manager.GetRouteConfig(); routerConfig != nil {
						if name := routerConfig.Name; len(name) <= 0 && port > 0 {
							routerConfig.Name = fmt.Sprintf("inbound|%d||", port)
							if a, e := ptypes.MarshalAny(manager); e != nil {
								log.DefaultLogger.Errorf("marshal connection manager back to any failed, %s", e)
							} else {
								filter.ConfigType = &listenerv3.Filter_TypedConfig{TypedConfig: a}
							}
						}
						rc.routes = append(rc.routes, routerConfig)
					}
				default:
					log.DefaultLogger.Warnf("cannot handle route config type, listener: %s, name: %s",
						listener.Name, filter.Name)
				}
			} else {
				log.DefaultLogger.Warnf("cannot handle route type, listener: %s, filter: %s",
					listener.Name, filter.Name)
			}
		}
	}
	return nil
}

func (c *XDSConfig) loadADSConfig(dynamicResources *envoy_config_bootstrap_v3.Bootstrap_DynamicResources) error {
	if dynamicResources == nil || dynamicResources.AdsConfig == nil {
		log.DefaultLogger.Errorf("DynamicResources is null")
		err := errors.New("null point exception")
		return err
	}
	err := dynamicResources.AdsConfig.Validate()
	if err != nil {
		log.DefaultLogger.Errorf("Invalid DynamicResources")
		return err
	}
	config, err := c.getAPISourceEndpoint(dynamicResources.AdsConfig)
	if err != nil {
		log.DefaultLogger.Errorf("fail to get api source endpoint")
		return err
	}
	c.ADSConfig = config
	return nil
}

func (c *XDSConfig) getAPISourceEndpoint(source *envoy_config_core_v3.ApiConfigSource) (*ADSConfig, error) {
	config := &ADSConfig{}
	if source.ApiType != envoy_config_core_v3.ApiConfigSource_GRPC {
		log.DefaultLogger.Errorf("unsupported api type: %v", source.ApiType)
		err := errors.New("only support GRPC api type yet")
		return nil, err
	}
	config.APIType = source.ApiType
	if source.RefreshDelay == nil || source.RefreshDelay.GetSeconds() <= 0 {
		duration := time.Duration(time.Second * 10) // default refresh delay
		config.RefreshDelay = &duration
	} else {
		duration := conv.ConvertDuration(source.RefreshDelay)
		config.RefreshDelay = &duration
	}

	config.Services = make([]*ServiceConfig, 0, len(source.GrpcServices))
	for _, service := range source.GrpcServices {
		t := service.TargetSpecifier
		if target, ok := t.(*envoy_config_core_v3.GrpcService_EnvoyGrpc_); ok {
			serviceConfig := ServiceConfig{}
			if service.Timeout == nil || (service.Timeout.GetSeconds() <= 0 && service.Timeout.GetNanos() <= 0) {
				duration := time.Duration(time.Second) // default connection timeout
				serviceConfig.Timeout = &duration
			} else {
				var nanos = service.Timeout.Seconds*int64(time.Second) + int64(service.Timeout.Nanos)
				duration := time.Duration(nanos)
				serviceConfig.Timeout = &duration
			}
			clusterName := target.EnvoyGrpc.ClusterName
			serviceConfig.ClusterConfig = c.Clusters[clusterName]
			if serviceConfig.ClusterConfig == nil {
				log.DefaultLogger.Errorf("cluster not found: %s", clusterName)
				return nil, fmt.Errorf("cluster not found: %s", clusterName)
			}
			config.Services = append(config.Services, &serviceConfig)
		} else if _, ok := t.(*envoy_config_core_v3.GrpcService_GoogleGrpc_); ok {
			log.DefaultLogger.Warnf("GrpcService_GoogleGrpc_ not support yet")
			continue
		}
	}
	return config, nil
}

func (c *XDSConfig) loadClusters(staticResources *envoy_config_bootstrap_v3.Bootstrap_StaticResources) error {
	if staticResources == nil {
		log.DefaultLogger.Errorf("StaticResources is null")
		err := errors.New("null point exception")
		return err
	}
	err := staticResources.Validate()
	if err != nil {
		log.DefaultLogger.Errorf("Invalid StaticResources")
		return err
	}
	c.Clusters = make(map[string]*ClusterConfig)
	for _, cluster := range staticResources.Clusters {
		name := cluster.Name
		config := ClusterConfig{}

		if cluster.TransportSocket != nil && cluster.TransportSocket.Name == wellknown.TransportSocketTls {
			config.TlsContext = cluster.TransportSocket
		}

		if cluster.LbPolicy != envoy_config_cluster_v3.Cluster_RANDOM {
			log.DefaultLogger.Warnf("only random lbPoliy supported, convert to random")
		}

		config.LbPolicy = envoy_config_cluster_v3.Cluster_RANDOM
		if cluster.ConnectTimeout.GetSeconds() <= 0 {
			duration := time.Second * 10
			config.ConnectTimeout = &duration // default connect timeout
		} else {
			duration := conv.ConvertDuration(cluster.ConnectTimeout)
			config.ConnectTimeout = &duration
		}

		if len(cluster.LoadAssignment.Endpoints) == 0 {
			log.DefaultLogger.Fatalf("xds v3 cluster.loadassignment is empty")
		}

		config.Address = make([]string, 0, len(cluster.LoadAssignment.GetEndpoints()[0].LbEndpoints))
		for _, host := range cluster.LoadAssignment.GetEndpoints()[0].LbEndpoints {
			endpoint := host.GetEndpoint()

			// Istio 1.8+ use istio-agent proxy request Istiod
			if pipe := endpoint.Address.GetPipe(); pipe != nil {
				newAddress := fmt.Sprintf("unix://%s", pipe.Path)
				config.Address = append(config.Address, newAddress)
				break
			}

			if endpoint.Address.GetSocketAddress() == nil {
				log.DefaultLogger.Fatalf("xds v3 cluster.loadassignment pipe and socket both empty")
			}
			if port, ok := endpoint.Address.GetSocketAddress().PortSpecifier.(*envoy_config_core_v3.SocketAddress_PortValue); ok {
				newAddress := fmt.Sprintf("%s:%d", endpoint.Address.GetSocketAddress().Address, port.PortValue)
				config.Address = append(config.Address, newAddress)
			} else {
				log.DefaultLogger.Warnf("only PortValue supported")
				continue
			}
		}
		c.Clusters[name] = &config
	}
	return nil
}

// GetEndpoint return an endpoint address by random
func (c *ClusterConfig) GetEndpoint() (string, *time.Duration) {
	if c.LbPolicy != envoy_config_cluster_v3.Cluster_RANDOM || len(c.Address) < 1 {
		// never happen
		return "", nil
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	idx := r.Intn(len(c.Address))

	return c.Address[idx], c.ConnectTimeout
}

// GetStreamClient return a grpc stream client that connected to ads
func (c *ADSConfig) GetStreamClient() envoy_service_discovery_v3.AggregatedDiscoveryService_StreamAggregatedResourcesClient {
	if c.StreamClient != nil && c.StreamClient.Client != nil {
		return c.StreamClient.Client
	}

	sc := &StreamClient{
		Conn: c.buildClient(),
	}

	if sc.Conn == nil {
		return nil
	}
	client := envoy_service_discovery_v3.NewAggregatedDiscoveryServiceClient(sc.Conn)

	ctx, cancel := context.WithCancel(context.Background())
	sc.Cancel = cancel
	streamClient, err := client.StreamAggregatedResources(ctx)
	if err != nil {
		log.DefaultLogger.Infof("fail to create stream client: %v", err)
		if sc.Conn != nil {
			sc.Conn.Close()
		}
		return nil
	}
	sc.Client = streamClient
	c.StreamClient = sc
	return streamClient
}

const (
	unixSocksPrefix = "unix://"
	unixSocksPrefixLength = len(unixSocksPrefix)
)

func normalizeUnixSocksPath(maybeUnixSocks string) (normalized string) {
	if !strings.HasPrefix(maybeUnixSocks, unixSocksPrefix) {
		normalized = maybeUnixSocks
		return
	}
	absolutePath, _ := filepath.Abs(maybeUnixSocks[unixSocksPrefixLength:])
	normalized = unixSocksPrefix + absolutePath
	return
}

func (c *ADSConfig) buildClient() *grpc.ClientConn {
	if c.Services == nil {
		log.DefaultLogger.Errorf("no available ads service")
		return nil
	}
	var endpoint string
	var tlsContext *envoy_config_core_v3.TransportSocket
	for _, service := range c.Services {
		if service.ClusterConfig == nil {
			continue
		}
		endpoint, _ = service.ClusterConfig.GetEndpoint()
		if len(endpoint) > 0 {
			tlsContext = service.ClusterConfig.TlsContext
			break
		}
	}
	if len(endpoint) == 0 {
		log.DefaultLogger.Errorf("no available ads endpoint")
		return nil
	}
	endpoint = normalizeUnixSocksPath(endpoint)

	if tlsContext == nil || !featuregate.Enabled(featuregate.XdsMtlsEnable) {
		conn, err := grpc.Dial(endpoint, grpc.WithInsecure(), generateDialOption())
		if err != nil {
			log.DefaultLogger.Errorf("did not connect: %v", err)
			return nil
		}
		log.DefaultLogger.Infof("mosn estab grpc connection to pilot with address at %v", endpoint)
		return conn
	}

	// Grpc with mTls support
	creds, err := c.getTLSCreds(tlsContext)
	if err != nil {
		log.DefaultLogger.Errorf("xds-grpc get tls creds fail: err= %v", err)
		return nil
	}
	conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(creds), generateDialOption())
	if err != nil {
		log.DefaultLogger.Errorf("did not connect: %v", err)
		return nil
	}
	log.DefaultLogger.Infof("mosn estab grpc connection to pilot with address and mTls at %v", endpoint)
	return conn
}

func (c *ADSConfig) getTLSCreds(tlsContextConfig *envoy_config_core_v3.TransportSocket) (credentials.TransportCredentials, error) {

	tlsContext := &envoy_extensions_transport_sockets_tls_v3.UpstreamTlsContext{}
	if err := ptypes.UnmarshalAny(tlsContextConfig.GetTypedConfig(), tlsContext); err != nil {
		return nil, err
	}

	if tlsContext.CommonTlsContext.GetValidationContext() == nil ||
		tlsContext.CommonTlsContext.GetValidationContext().GetTrustedCa() == nil {
		return nil, errors.New("can't find trusted ca ")
	}
	rootCAPath := tlsContext.CommonTlsContext.GetValidationContext().GetTrustedCa().GetFilename()
	if len(tlsContext.CommonTlsContext.GetTlsCertificates()) <= 0 {
		return nil, errors.New("can't find client certificates")
	}
	if tlsContext.CommonTlsContext.GetTlsCertificates()[0].GetCertificateChain() == nil ||
		tlsContext.CommonTlsContext.GetTlsCertificates()[0].GetPrivateKey() == nil {
		return nil, errors.New("can't read client certificates fail")
	}
	certChainPath := tlsContext.CommonTlsContext.GetTlsCertificates()[0].GetCertificateChain().GetFilename()
	privateKeyPath := tlsContext.CommonTlsContext.GetTlsCertificates()[0].GetPrivateKey().GetFilename()
	log.DefaultLogger.Infof("mosn start with tls context,root ca certificate path = %v\n cert chain path = %v\n private key path = %v\n",
		rootCAPath, certChainPath, privateKeyPath)
	certPool := x509.NewCertPool()
	bs, err := ioutil.ReadFile(rootCAPath)
	if err != nil {
		return nil, err
	}
	ok := certPool.AppendCertsFromPEM(bs)
	if !ok {
		return nil, errors.New("failed to append certs")
	}
	certificate, err := tls.LoadX509KeyPair(
		certChainPath,
		privateKeyPath,
	)
	creds := credentials.NewTLS(&tls.Config{
		ServerName:   "",
		Certificates: []tls.Certificate{certificate},
		RootCAs:      certPool,
	})
	return creds, nil
}

func (c *ADSConfig) getADSRefreshDelay() *time.Duration {
	return c.RefreshDelay
}

func (c *ADSConfig) closeADSStreamClient() {
	if c.StreamClient == nil {
		return
	}
	c.StreamClient.Cancel()
	if c.StreamClient.Conn != nil {
		c.StreamClient.Conn.Close()
		c.StreamClient.Conn = nil
	}
	c.StreamClient.Client = nil
	c.StreamClient = nil
}

// [xds] [ads client] get resp timeout: rpc error: code = ResourceExhausted desc = grpc: received message larger than max (5193322 vs. 4194304), retry after 1s
// https://github.com/istio/istio/blob/9686754643d0939c1f4dd0ee20443c51183f3589/pilot/pkg/bootstrap/server.go#L662
// Istio xDS DiscoveryServer not set grpc MaxSendMsgSize. If this is not set, gRPC uses the default `math.MaxInt32`.
func generateDialOption() grpc.DialOption {
	return grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(math.MaxInt32),
	)
}
