package seata

import (
	"context"
	"encoding/json"
	"mosn.io/api"

	v2 "mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/log"
)

func init() {
	// static seata stream filter factory
	api.RegisterStream(v2.SEATA, CreateFilterFactory)
}

type factory struct {
	Conf *v2.Seata
}

func (f *factory) CreateFilterChain(context context.Context, callbacks api.StreamFilterChainFactoryCallbacks) {
	filter, err := NewFilter(f.Conf)
	if err == nil {
		callbacks.AddStreamReceiverFilter(filter, api.BeforeRoute)
		callbacks.AddStreamSenderFilter(filter, api.BeforeSend)
	} else {
		log.DefaultLogger.Fatalf("failed to init seata filter, err: %v", err)
	}
}

func CreateFilterFactory(conf map[string]interface{}) (api.StreamFilterChainFactory, error) {
	config, err := parseConfig(conf)
	if err != nil {
		return nil, err
	}
	return &factory{config }, nil
}

// parseConfig
func parseConfig(cfg map[string]interface{}) (*v2.Seata, error) {
	filterConfig := &v2.Seata{}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, filterConfig); err != nil {
		return nil, err
	}
	return filterConfig, nil
}