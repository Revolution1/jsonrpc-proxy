package main

import (
	"bytes"
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
	return func(h fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			start := time.Now()
			h(ctx)
			duration := time.Since(start)
			status := ctx.Response.StatusCode()
			path := gotils.B2S(ctx.URI().Path())
			ep := gotils.B2S(ctx.Method()) + "_" + path

			var reqSize, resSize int
			if ctx.Request.IsBodyStream() {
				reqSize = len(ctx.Request.Header.RawHeaders()) + ctx.Request.Header.ContentLength() + 4
			} else {
				reqSize = len(ctx.Request.Header.RawHeaders()) + len(ctx.Request.Body()) + 4
			}
			if ctx.Response.IsBodyStream() {
				reqSize = ctx.Response.Header.Len() + ctx.Response.Header.ContentLength() + 4
			} else {
				reqSize = ctx.Response.Header.Len() + len(ctx.Response.Body()) + 4
			}
			isRpcReq, _ := ctx.UserValue("isRpcReq").(bool)
			// TODO: method metrics
			if status != fasthttp.StatusNotFound && path != config.Manage.MetricsPath {
				RecvBytes.Add(float64(reqSize))
				SentBytes.Add(float64(resSize))
				if isRpcReq {
					ReqDur.WithLabelValues(strconv.Itoa(ctx.Response.StatusCode()), ep, "").Observe(float64(duration) / float64(time.Second))
				}
			}
			if isRpcReq {
				errStr := "OK"
				if err, ok := ctx.UserValue("rpcErr").(*RpcError); ok {
					errStr = err.AccessLogError()
					//code = err.Code
				} else if err, ok := ctx.UserValue("rpcErr").(error); ok {
					errStr = err.Error()
				}
				methodsStr := ""
				if methods, ok := ctx.UserValue("rpcMethods").([]string); ok {
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
