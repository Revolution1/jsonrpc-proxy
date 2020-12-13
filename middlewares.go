package main

import (
	"bytes"
	"fmt"
	"github.com/AdhityaRamadhanus/fasthttpcors"
	realip "github.com/Ferluci/fast-realip"
	"github.com/savsgio/gotils"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

type MiddleWare func(h fasthttp.RequestHandler) fasthttp.RequestHandler

func useMiddleWares(handler fasthttp.RequestHandler, middleware ...MiddleWare) fasthttp.RequestHandler {
	for _, m := range middleware {
		handler = m(handler)
	}
	return handler
}

func accessLogMetricHandler(prefix string, config *Config) MiddleWare {
	// TODO: from-cache, is-batch
	return func(h fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			start := time.Now()
			h(ctx)
			duration := time.Since(start)
			status := ctx.Response.StatusCode()
			path := gotils.B2S(ctx.URI().Path())

			var reqSize, resSize int
			if ctx.Request.IsBodyStream() {
				reqSize = len(ctx.Request.Header.RawHeaders()) + ctx.Request.Header.ContentLength() + 4
			} else {
				reqSize = len(ctx.Request.Header.RawHeaders()) + len(ctx.Request.Body()) + 4
			}
			if ctx.Response.IsBodyStream() {
				resSize = ctx.Response.Header.Len() + ctx.Response.Header.ContentLength() + 4
			} else {
				resSize = ctx.Response.Header.Len() + len(ctx.Response.Body()) + 4
			}
			isRpcReq, _ := ctx.UserValue("isRpcReq").(bool)
			var methods []string
			if isRpcReq {
				errStr := "OK"
				if status != 200 {
					errStr = fmt.Sprintf("%d(%s)", status, fasthttp.StatusMessage(status))
				}
				if err, ok := ctx.UserValue("rpcErr").(*RpcError); ok {
					errStr = err.AccessLogError()
					//code = err.Code
				} else if err, ok := ctx.UserValue("rpcErr").(error); ok {
					errStr = err.Error()
				}
				methodsStr := ""
				if methods = getCtxRpcMethods(ctx); methods != nil {
					methodsStr = strings.Join(methods, ",")
				}
				if config.AccessLog {
					log.Infof(
						`%sRPC - %s - "%s %s" %s %d %d "%s" %s`+"\n",
						prefix,
						realip.FromRequest(ctx),
						ctx.RequestURI(),
						methodsStr,
						errStr,
						reqSize,
						resSize,
						ctx.UserAgent(),
						duration,
					)
				}
			} else {
				if config.AccessLog {
					log.Infof(
						`%s%s - %s - "%s %s" %d %d %d "%s" %s`+"\n",
						prefix,
						bytes.ToUpper(ctx.URI().Scheme()),
						realip.FromRequest(ctx),
						ctx.Method(),
						ctx.RequestURI(),
						status,
						reqSize,
						resSize,
						ctx.UserAgent(),
						duration,
					)
				}
			}
			// TODO: method metrics
			if status != fasthttp.StatusNotFound && path != config.Manage.MetricsPath {
				RecvBytes.Add(float64(reqSize))
				SentBytes.Add(float64(resSize))
				method := ""
				if len(methods) > 0 {
					method = methods[0]
				}
				ReqDuration.WithLabelValues(strconv.Itoa(ctx.Response.StatusCode()), path, gotils.B2S(ctx.Method()), method).Observe(float64(duration) / float64(time.Second))
			}
		}
	}
}

func panicHandler(h fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				ctx.ResetBody()
				ctx.SetStatusCode(fasthttp.StatusInternalServerError)
				if debugMode {
					_, _ = ctx.Write(debug.Stack())
				}
			}
		}()
		h(ctx)
	}
}

type LeveledLogger struct {
	level log.Level
}

func (l LeveledLogger) Printf(format string, args ...interface{}) {
	log.StandardLogger().Logf(l.level, format, args...)
}

var Cors = fasthttpcors.DefaultHandler().CorsMiddleware
