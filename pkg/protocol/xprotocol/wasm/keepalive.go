package wasm

import (
	"context"

	"mosn.io/api"
	mosnctx "mosn.io/mosn/pkg/context"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/types"
)

func (proto *wasmProtocol) keepaliveRequest(context context.Context, requestId uint64) api.XFrame {
	ctx := mosnctx.Get(context, types.ContextKeyWasmContext)
	if ctx == nil {
		log.DefaultLogger.Errorf("[protocol] wasm %s keepalive request failed, wasm context not found.", proto.name)
		return nil
	}

	wasmCtx := ctx.(*Context)
	wasmCtx.instance.Lock(wasmCtx.abi)
	wasmCtx.abi.SetABIImports(wasmCtx)

	// invoke plugin keepalive impl
	err := wasmCtx.exports.ProxyKeepAliveBufferBytes(wasmCtx.contextId, requestId)
	if err != nil {
		wasmCtx.instance.Unlock()
		log.DefaultLogger.Errorf("[protocol] wasm %s keepalive request failed, err %v.", proto.name, err)

		// keepalive not supported.
		proto.finishWasmContext(context)
		return nil
	}

	wasmCtx.instance.Unlock()

	// keepalive detect
	if requestId == 0 {
		proto.finishWasmContext(context)
	}

	// When encode is called, the proxy gets the correct buffer
	wasmCtx.keepaliveReq = NewWasmRequestWithId(uint32(requestId), nil, nil)
	wasmCtx.keepaliveReq.Flag = HeartBeatFlag
	wasmCtx.keepaliveReq.ctx = wasmCtx

	return wasmCtx.keepaliveReq
}

func (proto *wasmProtocol) keepaliveResponse(context context.Context, request api.XFrame) api.XRespFrame {
	ctx := mosnctx.Get(context, types.ContextKeyWasmContext)
	if ctx == nil {
		log.DefaultLogger.Errorf("[protocol] wasm %s keepalive response failed, wasm context not found.", proto.name)
		return nil
	}

	wasmCtx := ctx.(*Context)
	wasmCtx.instance.Lock(wasmCtx.abi)
	wasmCtx.abi.SetABIImports(wasmCtx)

	// invoke plugin keepalive impl
	err := wasmCtx.exports.ProxyReplyKeepAliveBufferBytes(wasmCtx.contextId, request.(*Request))
	if err != nil {
		wasmCtx.instance.Unlock()
		log.DefaultLogger.Errorf("[protocol] wasm %s keepalive response failed, err %v.", proto.name, err)
		return nil
	}

	wasmCtx.instance.Unlock()

	// When encode is called, the proxy gets the correct buffer
	resp := NewWasmResponseWithId(uint32(request.GetRequestId()), nil, nil)
	resp.Flag |= HeartBeatFlag
	wasmCtx.keepaliveResp = resp
	wasmCtx.keepaliveResp.ctx = wasmCtx

	if !resp.IsReplacedId {
		resp.RpcId = resp.GetId()
	}

	return wasmCtx.keepaliveResp
}
