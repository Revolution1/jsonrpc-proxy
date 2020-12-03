package main

import (
	"github.com/fasthttp/router"
	"github.com/savsgio/gotils/nocopy"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/pprofhandler"
)

type Manage struct {
	nocopy.NoCopy
	config *Config
	Proxy  *Proxy
}

func NewManage(config *Config, proxy *Proxy) *Manage {
	return &Manage{config: config, Proxy: proxy}
}

func (m *Manage) registerHandler(r *router.Router) {
	r.GET("/debug/pprof/{name:*}", pprofhandler.PprofHandler)
	r.GET(m.config.Manage.MetricsPath, PrometheusHandler)
	group := r.Group(m.config.Manage.Path)
	group.GET("/", m.Index)
}

func (m *Manage) Index(ctx *fasthttp.RequestCtx) {
	_, _ = ctx.WriteString("JSON-RPC PROXY MANAGE PAGE")
}
