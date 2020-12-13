package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

const MetricsNs = "jsonrpc_proxy"

// since prometheus/client_golang use net/http we need this net/http adapter for fasthttp
var PrometheusHandler = fasthttpadaptor.NewFastHTTPHandler(promhttp.Handler())

// TODO: ref https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/metrics.go
var (
	ReqDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: MetricsNs,
			Name:      "request_duration_seconds",
			Help:      "request latencies of success requests",
			Buckets:   []float64{.005, .01, .02, .04, .06, .1, .2, .4, .6, 1, 2, 4},
		},
		[]string{"code", "path", "method", "rpc_method"},
	)
	ReqCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: MetricsNs,
			Name:      "requests_total",
			Help:      "Total number of rpc requests.",
		},
		[]string{"code", "path", "method", "rpc_method"},
	)
	HttpReqCnt = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: MetricsNs,
			Name:      "http_requests_total",
			Help:      "Total number of rpc requests by HTTP status code.",
		},
		[]string{"code", "path", "method"},
	)
	SentBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: MetricsNs,
		Name:      "server_bytes_sent",
		Help:      "total bytes sent by server",
	})
	RecvBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: MetricsNs,
		Name:      "server_bytes_recv",
		Help:      "total bytes received by server",
	})

	RpcCacheHit = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: MetricsNs,
			Name:      "rpc_cache_hit",
			Help:      "Total number of rpc requests cache hit.",
		},
		[]string{"method"},
	)
	RpcCacheMiss = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: MetricsNs,
			Name:      "rpc_cache_miss",
			Help:      "Total number of rpc requests cache miss.",
		},
		[]string{"method"},
	)
)

//func PromFastHttpMiddleware(metricsPath string) MiddleWare {
//	return func(h fasthttp.RequestHandler) fasthttp.RequestHandler {
//		return func(ctx *fasthttp.RequestCtx) {
//			uri := string(ctx.Request.URI().Path())
//			if uri == metricsPath {
//				// next
//				h(ctx)
//				return
//			}
//			start := time.Now()
//			// next
//			h(ctx)
//			status := strconv.Itoa(ctx.Response.StatusCode())
//			elapsed := float64(time.Since(start)) / float64(time.Second)
//			ep := string(ctx.Method()) + "_" + uri
//			ReqDuration.WithLabelValues(status, ep).Observe(elapsed)
//		}
//	}
//}

func init() {
	prometheus.MustRegister(
		ReqDuration, ReqCount, HttpReqCnt, SentBytes, RecvBytes,
		RpcCacheHit, RpcCacheMiss,
	)
}
