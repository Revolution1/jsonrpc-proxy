package proxy

import (
	"bytes"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/revolution1/jsonrpc-proxy/jsonrpc"
	"github.com/valyala/fasthttp"
)

func WriteRpcErrResp(ctx *fasthttp.RequestCtx, rpcError *jsonrpc.RpcError, id interface{}) {
	ctx.ResetBody()
	ctx.SetUserValue("rpcErr", rpcError)
	ctx.SetBodyString(rpcError.JsonError(id))
	ctx.SetStatusCode(jsonrpc.StatusCodeOfRpcError(rpcError))
	ctx.SetContentType("application/json; charset=utf-8")
}

func GetCtxRpcErr(ctx *fasthttp.RequestCtx) *jsonrpc.RpcError {
	if e, ok := ctx.UserValue("rpcErr").(*jsonrpc.RpcError); ok {
		return e
	}
	return nil
}
func GetCtxRpcMethods(ctx *fasthttp.RequestCtx) []string {
	if m, ok := ctx.UserValue("rpcMethods").([]string); ok {
		return m
	}
	return nil
}

func SetCtxRpcMethods(ctx *fasthttp.RequestCtx, methodNames []string) {
	ctx.SetUserValue("rpcMethods", methodNames)
}

func WriteJsonResp(ctx *fasthttp.RequestCtx, resp *jsonrpc.RpcResponse) {
	data, err := jsoniter.Marshal(resp)
	if err != nil {
		log.WithError(err).Panic("fail to marshal response from cache")
	}
	WriteJsonRespRaw(ctx, data, jsonrpc.StatusCodeOfRpcError(resp.Error))
	if resp.Error != nil {
		ctx.SetUserValue("rpcErr", resp.Error)
	}
}

func WriteJsonResps(ctx *fasthttp.RequestCtx, resps []jsonrpc.RpcResponse) {
	data, err := jsoniter.Marshal(resps)
	if err != nil {
		log.WithError(err).Panic("fail to marshal response from cache")
	}
	status := 500
	for _, r := range resps {
		c := jsonrpc.StatusCodeOfRpcError(r.Error)
		if c < status {
			status = c
		}
	}
	WriteJsonRespRaw(ctx, data, status)
}

func WriteJsonRespRaw(ctx *fasthttp.RequestCtx, body []byte, code int) {
	ctx.Response.SetBody(body)
	if ctx.Request.Header.ConnectionClose() {
		ctx.Response.SetConnectionClose()
	}
	ctx.SetContentType("application/json; charset=utf-8")
	ctx.SetStatusCode(code)
}

const (
	strGzip    = "gzip"
	strBr      = "br"
	strDeflate = "deflate"
)

var ErrUnknownContentEncoding = errors.New("Unknown Content Encoding")

func GetResponseBody(resp *fasthttp.Response) ([]byte, error) {
	encoding := string(bytes.TrimSpace(resp.Header.Peek(fasthttp.HeaderContentEncoding)))
	switch encoding {
	case strGzip:
		return resp.BodyGunzip()
	case strBr:
		return resp.BodyUnbrotli()
	case strDeflate:
		return resp.BodyInflate()
	default:
		body := resp.Body()
		//if !isASCII(body) {
		//	// give it a try
		//	b, err := resp.BodyGunzip()
		//	if err != nil {
		//		return body, nil
		//	}
		//	return b, nil
		//}
		return body, nil
	}
	//return resp.Body(), errors.Wrapf(ErrUnknownContentEncoding, "encoding: '%s'", encoding)
}

func SetAcceptEncoding(ctx *fasthttp.RequestCtx) {
	ctx.Request.Header.Del(fasthttp.HeaderAcceptEncoding)
}
