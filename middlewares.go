package main

import (
	"bytes"
	"fmt"
	"github.com/AdhityaRamadhanus/fasthttpcors"
	realip "github.com/Ferluci/fast-realip"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/savsgio/gotils"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

var WellKnownHealthCheckerUserAgentPrefixes = []string{
	"ELB-HealthChecker",
	"kube-probe",
	"Prometheus",
}

func guessIsHealthChecker(ua string) bool {
	for _, p := range WellKnownHealthCheckerUserAgentPrefixes {
		if strings.HasPrefix(ua, p) {
			return true
		}
	}
	return false
}

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
			method := string(ctx.Method())
			rpcErrCode := 0
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
			var rpcMethods []string
			if isRpcReq {
				errStr := "OK"
				if status != 200 {
					errStr = fmt.Sprintf("%d(%s)", status, fasthttp.StatusMessage(status))
				}
				if err, ok := ctx.UserValue("rpcErr").(*RpcError); ok {
					errStr = err.AccessLogError()
					rpcErrCode = err.Code
				} else if err, ok := ctx.UserValue("rpcErr").(error); ok {
					errStr = err.Error()
				}
				methodsStr := ""
				if rpcMethods = getCtxRpcMethods(ctx); rpcMethods != nil {
					methodsStr = strings.Join(rpcMethods, ",")
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
					fun := log.Infof
					if guessIsHealthChecker(string(ctx.UserAgent())) {
						fun = log.Tracef
					}
					fun(
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
			if status != fasthttp.StatusNotFound &&
				path != config.Manage.MetricsPath &&
				!bytes.Equal(ctx.Method(), []byte(fasthttp.MethodOptions)) {
				RecvBytes.Add(float64(reqSize))
				SentBytes.Add(float64(resSize))
				if isRpcReq {
					for _, m := range rpcMethods {
						ReqDuration.With(prometheus.Labels{
							"code":       strconv.Itoa(rpcErrCode),
							"path":       path,
							"method":     method,
							"rpc_method": m,
						}).Observe(float64(duration) / float64(time.Second))
					}
				} else {
					ReqDuration.With(prometheus.Labels{
						"code":       strconv.Itoa(status),
						"path":       path,
						"method":     method,
						"rpc_method": "",
					}).Observe(float64(duration) / float64(time.Second))
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

var Cors MiddleWare

func init() {
	corsHandler := fasthttpcors.NewCorsHandler(fasthttpcors.Options{
		AllowedOrigins: []string{"*"},
		AllowedHeaders: []string{"*"},
		AllowedMethods: []string{"GET", "POST"},
	})
	Cors = corsHandler.CorsMiddleware
}
