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

package sds

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_extensions_transport_sockets_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	envoy_service_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	envoy_service_secret_v3 "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	ptypes "github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/types"
)

func Test_AddUpdateCallbackV3(t *testing.T) {
	// init prepare
	sdsUdsPath := "/tmp/sds2"
	SubscriberRetryPeriod = 500 * time.Millisecond
	defer func() {
		SubscriberRetryPeriod = 3 * time.Second
	}()
	callback := 0
	SetSdsPostCallback(func() {
		callback = 1
	})
	// mock sds server
	srv := InitMockSdsServerV3(sdsUdsPath, t)
	defer srv.Stop()
	config := InitSdsSecertConfigV3(sdsUdsPath)
	sdsClient := NewSdsClientSingletonV3(config)
	if sdsClient == nil {
		t.Errorf("get sds client fail")
	}

	// Do not call CloseSdsClient(), because the 'SdsClient' is a global single object.
	//defer CloseSdsClient()

	// wait server start and stop makes reconnect
	time.Sleep(time.Second)
	srv.Stop()
	// wait server connection stop
	time.Sleep(time.Second)
	// send request
	updatedChan := make(chan int, 1) // do not block the update channel
	log.DefaultLogger.Infof(" add update callback")
	sdsClient.AddUpdateCallback(config, func(name string, secret *types.SdsSecret) {
		if name != "default" {
			t.Errorf("name should same with config.name")
		}
		log.DefaultLogger.Infof("update callback is called")
		updatedChan <- 1
	})
	// sdsClient.SetSecret(config.Name, &envoy_extensions_transport_sockets_tls_v3.Secret{})
	time.Sleep(time.Second)
	go func() {
		err := srv.Start()
		if !srv.started {
			t.Fatalf("%s start error: %v", sdsUdsPath, err)
		}
	}()
	select {
	case <-updatedChan:
		if callback != 1 {
			t.Fatalf("sds post callback unexpected")
		}
	case <-time.After(time.Second * 10):
		t.Errorf("callback reponse timeout")
	}
}
func Test_DeleteUpdateCallbackV3(t *testing.T) {
	sdsUdsPath := "/tmp/sds3"
	// mock sds server
	srv := InitMockSdsServerV3(sdsUdsPath, t)
	defer srv.Stop()
	config := InitSdsSecertConfigV3(sdsUdsPath)
	config.Name = "delete"
	sdsClient := NewSdsClientSingletonV3(config)
	if sdsClient == nil {
		t.Errorf("get sds client fail")
	}

	// Do not call CloseSdsClient(), because the 'SdsClient' is a global single object.
	//defer CloseSdsClient()

	sdsClient.AddUpdateCallback(config, func(name string, secret *types.SdsSecret) {})
	err := sdsClient.DeleteUpdateCallback(config)
	if err != nil {
		t.Errorf("delete update callback fail")
	}
}

func InitSdsSecertConfigV3(sdsUdsPath string) *envoy_extensions_transport_sockets_tls_v3.SdsSecretConfig {
	gRPCConfig := &envoy_config_core_v3.GrpcService_GoogleGrpc{
		TargetUri:  sdsUdsPath,
		StatPrefix: "sds-prefix",
		ChannelCredentials: &envoy_config_core_v3.GrpcService_GoogleGrpc_ChannelCredentials{
			CredentialSpecifier: &envoy_config_core_v3.GrpcService_GoogleGrpc_ChannelCredentials_LocalCredentials{
				LocalCredentials: &envoy_config_core_v3.GrpcService_GoogleGrpc_GoogleLocalCredentials{},
			},
		},
	}
	config := &envoy_extensions_transport_sockets_tls_v3.SdsSecretConfig{
		Name: "default",
		SdsConfig: &envoy_config_core_v3.ConfigSource{
			ConfigSourceSpecifier: &envoy_config_core_v3.ConfigSource_ApiConfigSource{
				ApiConfigSource: &envoy_config_core_v3.ApiConfigSource{
					ApiType: envoy_config_core_v3.ApiConfigSource_GRPC,
					GrpcServices: []*envoy_config_core_v3.GrpcService{
						{
							TargetSpecifier: &envoy_config_core_v3.GrpcService_GoogleGrpc_{
								GoogleGrpc: gRPCConfig,
							},
						},
					},
				},
			},
		},
	}
	return config
}

func InitMockSdsServerV3(sdsUdsPath string, t *testing.T) *fakeSdsServerV3 {
	s := NewFakeSdsServerV3(sdsUdsPath)
	go func() {
		err := s.Start()
		if !s.started {
			t.Fatalf("server %s failed: %v", sdsUdsPath, err)
		}
	}()
	return s
}

func NewFakeSdsServerV3(sdsUdsPath string) *fakeSdsServerV3 {
	return &fakeSdsServerV3{
		sdsUdsPath: sdsUdsPath,
	}

}

// SecretDiscoveryServiceServer is the server API for SecretDiscoveryService service.
//type SecretDiscoveryServiceServer interface {
//        DeltaSecrets(SecretDiscoveryService_DeltaSecretsServer) error
//        StreamSecrets(SecretDiscoveryService_StreamSecretsServer) error
//        FetchSecrets(context.Context, *v2.DiscoveryRequest) (*v2.DiscoveryResponse, error)
//}
type fakeSdsServerV3 struct {
	envoy_service_secret_v3.SecretDiscoveryServiceServer
	server     *grpc.Server
	sdsUdsPath string
	started    bool
}

func (s *fakeSdsServerV3) Start() error {
	grpcOptions := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(10240),
	}
	grpcWorkloadServer := grpc.NewServer(grpcOptions...)
	s.register(grpcWorkloadServer)
	s.server = grpcWorkloadServer
	ln, err := setUpUdsV3(s.sdsUdsPath)
	if err != nil {
		return err
	}
	s.started = true
	err = s.server.Serve(ln)
	return err
}

func (s *fakeSdsServerV3) Stop() {
	if s.started {
		s.server.Stop()
	}
}

func (s *fakeSdsServerV3) StreamSecrets(stream envoy_service_secret_v3.SecretDiscoveryService_StreamSecretsServer) error {
	log.DefaultLogger.Infof("get stream secrets")
	// wait for request
	// for test just ignore
	_, err := stream.Recv()
	if err != nil {
		log.DefaultLogger.Errorf("streamn receive error: %v", err)
		return err
	}
	resp := &envoy_service_discovery_v3.DiscoveryResponse{
		TypeUrl:     resourcev3.SecretType,
		VersionInfo: "0",
		Nonce:       "0",
	}
	secret := &envoy_extensions_transport_sockets_tls_v3.Secret{
		Name: "default",
	}
	ms, err := ptypes.MarshalAny(secret)
	if err != nil {
		log.DefaultLogger.Errorf("marshal secret error: %v", err)
		return err
	}
	resp.Resources = append(resp.Resources, ms)
	if err := stream.Send(resp); err != nil {
		log.DefaultLogger.Errorf("send response error: %v", err)
		return err
	}
	// keep alive for 3 second for client connection
	time.Sleep(3 * time.Second)
	return nil
}

func (s *fakeSdsServerV3) FetchSecrets(ctx context.Context, discReq *envoy_service_discovery_v3.DiscoveryRequest) (*envoy_service_discovery_v3.DiscoveryResponse, error) {
	// not implement
	return nil, nil
}

func (s *fakeSdsServerV3) register(rpcs *grpc.Server) {
	envoy_service_secret_v3.RegisterSecretDiscoveryServiceServer(rpcs, s)
}

func setUpUdsV3(udsPath string) (net.Listener, error) {
	// Remove unix socket before use.
	if err := os.Remove(udsPath); err != nil && !os.IsNotExist(err) {
		// Anything other than "file not found" is an error.
		return nil, fmt.Errorf("failed to remove unix://%s", udsPath)
	}

	var err error
	udsListener, err := net.Listen("unix", udsPath)
	if err != nil {
		return nil, err
	}

	// Update Sds UDS file permission so that istio-proxy has permission to access it.
	if _, err := os.Stat(udsPath); err != nil {
		return nil, fmt.Errorf("sds uds file %q doesn't exist", udsPath)
	}
	if err := os.Chmod(udsPath, 0666); err != nil {
		return nil, fmt.Errorf("failed to update %q permission", udsPath)
	}

	return udsListener, nil
}
