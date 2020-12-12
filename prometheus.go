package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// since prometheus/client_golang use net/http we need this net/http adapter for fasthttp
var PrometheusHandler = fasthttpadaptor.NewFastHTTPHandler(promhttp.Handler())

// TODO: ref https://github.com/caddyserver/caddy/blob/master/modules/caddyhttp/metrics.go
var (
	ReqDur = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			//Subsystem: subsystem,
			Name:    "request_duration_seconds",
			Help:    "request latencies of success requests",
			Buckets: []float64{.005, .01, .02, 0.04, .06, 0.08, .1, 0.15, .25, 0.4, .6, .8, 1, 1.5, 2, 3, 5},
		},
		[]string{"code", "path", "method"},
	)
	ReqCnt = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_requests_total",
			Help: "Total number of rpc requests by HTTP status code.",
		},
		[]string{"code", "path", "method"},
	)
	HttpReqCnt = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of rpc requests by HTTP status code.",
		},
		[]string{"code", "path", "method"},
	)
	SentBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "server_bytes_sent",
		Help: "total bytes sent by server",
	})
	RecvBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "server_bytes_recv",
		Help: "total bytes received by server",
	})
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
//			ReqDur.WithLabelValues(status, ep).Observe(elapsed)
//		}
//	}
//}

func init() {
	prometheus.MustRegister(ReqDur, ReqCnt, HttpReqCnt, SentBytes, RecvBytes)
}
