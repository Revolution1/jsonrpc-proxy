package middleware

import (
	"bytes"
	"fmt"
	realip "github.com/Ferluci/fast-realip"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/revolution1/jsonrpc-proxy/oldconfig"
	"github.com/revolution1/jsonrpc-proxy/jsonrpc"
	"github.com/revolution1/jsonrpc-proxy/metrics"
	"github.com/revolution1/jsonrpc-proxy/proxy"
	"github.com/savsgio/gotils"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"strconv"
	"strings"
	"time"
)

func AccessLogMetricHandler(prefix string, config *oldconfig.Config) MiddleWare {
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
				if err, ok := ctx.UserValue("rpcErr").(*jsonrpc.RpcError); ok {
					errStr = err.AccessLogError()
					rpcErrCode = err.Code
				} else if err, ok := ctx.UserValue("rpcErr").(error); ok {
					errStr = err.Error()
				}
				methodsStr := ""
				if rpcMethods = proxy.GetCtxRpcMethods(ctx); rpcMethods != nil {
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
				metrics.RecvBytes.Add(float64(reqSize))
				metrics.SentBytes.Add(float64(resSize))
				if isRpcReq {
					for _, m := range rpcMethods {
						metrics.ReqDuration.With(prometheus.Labels{
							"code":       strconv.Itoa(rpcErrCode),
							"path":       path,
							"method":     method,
							"rpc_method": m,
						}).Observe(float64(duration) / float64(time.Second))
					}
				} else {
					metrics.ReqDuration.With(prometheus.Labels{
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
