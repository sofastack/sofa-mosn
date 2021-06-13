package conv

import (
	"time"

	envoy_config_accesslog_v3 "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	envoy_config_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_extensions_access_loggers_file_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/file/v3"
	envoy_extensions_filters_network_http_connection_manager_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_extensions_filters_network_tcp_proxy_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	"github.com/envoyproxy/go-control-plane/pkg/conversion"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/duration"
	"mosn.io/mosn/pkg/log"
)

func getFilterConfig(filter *envoy_config_listener_v3.Filter, out proto.Message) error {
	switch c := filter.ConfigType.(type) {
	case *envoy_config_listener_v3.Filter_TypedConfig:
		if err := ptypes.UnmarshalAny(c.TypedConfig, out); err != nil {
			return err
		}
	}
	return nil
}

func GetHTTPConnectionManager(filter *envoy_config_listener_v3.Filter) *envoy_extensions_filters_network_http_connection_manager_v3.HttpConnectionManager {
	cm := &envoy_extensions_filters_network_http_connection_manager_v3.HttpConnectionManager{}
	if err := getFilterConfig(filter, cm); err != nil {
		log.DefaultLogger.Errorf("failed to get HTTP connection manager config: %s", err)
		return nil
	}
	return cm
}

func GetTcpProxy(filter *envoy_config_listener_v3.Filter) *envoy_extensions_filters_network_tcp_proxy_v3.TcpProxy {
	cm := &envoy_extensions_filters_network_tcp_proxy_v3.TcpProxy{}
	if err := getFilterConfig(filter, cm); err != nil {
		log.DefaultLogger.Errorf("failed to get HTTP connection manager config: %s", err)
		return nil
	}
	return cm
}

func GetAccessLog(log *envoy_config_accesslog_v3.AccessLog) (*envoy_extensions_access_loggers_file_v3.FileAccessLog, error) {
	al := &envoy_extensions_access_loggers_file_v3.FileAccessLog{}
	switch log.ConfigType.(type) {
	case *envoy_config_accesslog_v3.AccessLog_TypedConfig:
		if err := ptypes.UnmarshalAny(log.GetTypedConfig(), al); err != nil {
			return nil, err
		}
	case *envoy_config_accesslog_v3.AccessLog_HiddenEnvoyDeprecatedConfig:
		if err := conversion.StructToMessage(log.GetHiddenEnvoyDeprecatedConfig(), al); err != nil {
			return nil, err
		}
	}

	return al, nil

}

func ConvertDuration(p *duration.Duration) time.Duration {
	if p == nil {
		return time.Duration(0)
	}
	d := time.Duration(p.Seconds) * time.Second
	if p.Nanos != 0 {
		dur := d + time.Duration(p.Nanos)
		if (dur < 0) != (p.Nanos < 0) {
			log.DefaultLogger.Warnf("duration: %#v is out of range for time.Duration, ignore nanos", p)
		} else {
			d = dur
		}
	}
	return d
}
