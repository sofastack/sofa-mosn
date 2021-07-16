package jwtauthn

import (
	"context"

	"mosn.io/api"
	"mosn.io/mosn/pkg/variable"
	"mosn.io/pkg/buffer"

	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/types"
)

type filter struct {
	config  FilterConfig
	handler api.StreamReceiverFilterHandler
}

func newJwtAuthnFilter(config FilterConfig) *filter {
	return &filter{
		config: config,
	}
}

func (f *filter) OnReceive(ctx context.Context, headers api.HeaderMap, buf buffer.IoBuffer, trailers api.HeaderMap) api.StreamFilterStatus {
	if log.DefaultLogger.GetLogLevel() >= log.DEBUG {
		log.DefaultLogger.Debugf("call jwt_authn filter")
	}

	// TODO(huangrh): bypass

	requestArg, err := variable.GetString(ctx, types.VarHttpRequestArg)
	if err != nil {
		log.DefaultLogger.Errorf("[jwt_authn filter] get query parameter: %v", err)
		return api.StreamFilterContinue
	}

	requestPath, err := variable.GetString(ctx, types.VarHttpRequestPath)
	if err != nil {
		log.DefaultLogger.Errorf("[jwt_authn filter] get path: %v", err)
		return api.StreamFilterStop
	}

	// Verify the JWT token
	verifier := f.config.FindVerifier(headers, requestArg, requestPath)
	if verifier == nil {
		return api.StreamFilterContinue
	}
	if err := verifier.Verify(headers, requestPath); err != nil {
		f.handler.SendHijackReply(401, headers)
		return api.StreamFilterStop
	}

	return api.StreamFilterContinue
}

func (f *filter) SetReceiveFilterHandler(handler api.StreamReceiverFilterHandler) {
	f.handler = handler
}

func (f *filter) OnDestroy() {}
