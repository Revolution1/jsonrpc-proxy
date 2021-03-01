package server

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/revolution1/jsonrpc-proxy/plugin"
	"github.com/revolution1/jsonrpc-proxy/proxyctx"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type HTTPRouterConfig struct {
	Path    string           `json:"path"`
	Method  string           `json:"method"`
	Plugins []*plugin.Config `json:"plugins"`
}

type HTTPRouter struct {
	config         *HTTPRouterConfig
	defaultPlugins []plugin.Plugin
	plugins        []plugin.Plugin

	logger *logrus.Entry
}

func NewHTTPRouter(config *HTTPRouterConfig, defaultPlugins []plugin.Plugin) *HTTPRouter {
	r := &HTTPRouter{config: config, defaultPlugins: defaultPlugins}
	r.init()
	return r
}

func (h *HTTPRouter) Name() string {
	return fmt.Sprintf("%s(%s", h.Method(), h.Path())
}

func (h *HTTPRouter) Method() string {
	return h.config.Method
}

func (h *HTTPRouter) Path() string {
	if h.config.Path == "" {
		return fasthttp.MethodGet
	}
	return h.config.Path
}

func (h *HTTPRouter) init() {
	//init plugins
	h.logger = logrus.WithField("router", h.Name())
	for _, pc := range h.config.Plugins {
		info, err := plugin.GetInfoOfID(pc.ID)
		if err != nil {
			panic(err)
		}
		if !info.CanHandle(proxyctx.ContextHTTP) {
			panic(errors.Errorf("plugin %s cannot handle context %s of this router", pc.ID, proxyctx.ContextHTTP))
		}
		plug, err := plugin.NewPlugin(pc)
		if err != nil {
			panic(err)
		}
		h.plugins = append(h.plugins, plug)
	}
}

func (h *HTTPRouter) Handler(ctx *fasthttp.RequestCtx) {
	handler := h.handler(plugin.DefaultTerminator)
	err := handler(proxyctx.NewHTTPContext(nil, ctx))
	if err != nil {
		if errors.Is(err, proxyctx.Return) {
			return
		}
		if e, ok := err.(*proxyctx.GeneralError); ok {
			e.WriteHttpCtx(ctx)
		} else {
			ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
		}
		return
	}
}

func (h *HTTPRouter) handler(next plugin.HandleFunc) plugin.HandleFunc {
	f := next
	for i := len(h.plugins) - 1; i >= 0; i-- {
		f = h.plugins[i].Handler(f)
	}
	for i := len(h.defaultPlugins) - 1; i >= 0; i-- {
		f = h.defaultPlugins[i].Handler(f)
	}
	return func(ctx proxyctx.Context) (err error) {
		h.logger.Tracef("start handling in router %s", h.Name())
		err = f(ctx)
		h.logger.Tracef("done handling in router %s", h.Name())
		return
	}
}

func (h *HTTPRouter) Destroy() {
	for _, p := range h.plugins {
		p.Destroy()
	}
}
