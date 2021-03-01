package manage

import (
	"github.com/fasthttp/router"
	"github.com/revolution1/jsonrpc-proxy/oldconfig"
	"github.com/revolution1/jsonrpc-proxy/metrics"
	"github.com/revolution1/jsonrpc-proxy/proxy"
	"github.com/savsgio/gotils/nocopy"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/pprofhandler"
)

type Manage struct {
	nocopy.NoCopy
	config *oldconfig.Config
	Proxy  *proxy.Proxy
}

func NewManage(config *oldconfig.Config, proxy *proxy.Proxy) *Manage {
	return &Manage{config: config, Proxy: proxy}
}

func (m *Manage) RegisterHandler(r *router.Router) {
	r.GET("/debug/pprof/{name:*}", pprofhandler.PprofHandler)
	r.GET(m.config.Manage.MetricsPath, metrics.PrometheusHandler)
	group := r.Group(m.config.Manage.Path)
	group.GET("/", m.Index)
}

func (m *Manage) Index(ctx *fasthttp.RequestCtx) {
	_, _ = ctx.WriteString("JSON-RPC PROXY MANAGE PAGE")
}
