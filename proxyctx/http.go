package proxyctx

import "github.com/valyala/fasthttp"

const ContextHTTP = "http"

type HTTPContext struct {
	parent Context
	Ctx    *fasthttp.RequestCtx
}

func NewHTTPContext(parent Context, ctx *fasthttp.RequestCtx) *HTTPContext {
	return &HTTPContext{Ctx: ctx, parent: parent}
}

func (h *HTTPContext) Parent() Context {
	return h.parent
}

func (h *HTTPContext) Type() string {
	return ContextHTTP
}
