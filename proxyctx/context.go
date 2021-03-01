package proxyctx

import (
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
)

type Context interface {
	Type() string
	Parent() Context
}

var (
	//Return returning this error will skip following plugins
	Return = errors.New("return to upper level context")
)

type GeneralError struct {
	Name    string
	Code    int
	Message string
	Body    []byte
	Headers map[string]string
}

func (g GeneralError) Error() string {
	return g.Name + ": " + g.Message
}

func (g *GeneralError) WriteHttpCtx(ctx *fasthttp.RequestCtx) {
	ctx.Error(g.Error(), g.Code)
	if g.Headers != nil {
		for k, v := range g.Headers {
			ctx.Response.Header.Set(k, v)
		}
	}
	if g.Body != nil {
		ctx.SetBody(g.Body)
	}
}
