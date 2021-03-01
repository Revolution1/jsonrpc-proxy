package middleware

import (
	"github.com/revolution1/jsonrpc-proxy/oldconfig"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"runtime/debug"
)

func PanicHandler(h fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				log.Errorf("panic %v", r)
				ctx.ResetBody()
				ctx.SetStatusCode(fasthttp.StatusInternalServerError)
				if oldconfig.DebugMode {
					_, _ = ctx.Write(debug.Stack())
				}
			}
		}()
		h(ctx)
	}
}
