package server

import (
	"github.com/fasthttp/router"
	"github.com/pkg/errors"
	"github.com/revolution1/jsonrpc-proxy/listener"
	"github.com/revolution1/jsonrpc-proxy/plugin"
	"github.com/revolution1/jsonrpc-proxy/proxyctx"
	"github.com/revolution1/jsonrpc-proxy/types"
	"github.com/valyala/fasthttp"
	"golang.org/x/sync/errgroup"
	"net"
)

type HTTPConfig struct {
	ID        ID                  `json:"iD"`
	Type      Type                `json:"type"`
	Listeners []types.ListenerID  `json:"listeners"`
	Plugins   []*plugin.Config    `json:"plugins"`
	Routers   []*HTTPRouterConfig `json:"routers"`
}

type HTTPServer struct {
	server      *fasthttp.Server
	config      *HTTPConfig
	listeners   []net.Listener
	plugins     []plugin.Plugin
	router      *router.Router
	httpRouters []*HTTPRouter
}

func NewHTTPServer(config *HTTPConfig) *HTTPServer {
	s := &HTTPServer{config: config}
	s.init()
	return s
}

func (h *HTTPServer) init() {
	// init listeners
	if len(h.config.Listeners) == 0 {
		panic(errors.Errorf("empty listener list of %s", h.config.ID))
	}
	for _, id := range h.config.Listeners {
		l, err := listener.AcquireListener(id)
		if err != nil {
			panic(errors.Wrapf(err, "server %s failed to acquire listener", h.config.ID))
		}
		h.listeners = append(h.listeners, l)
	}
	// init plugins
	for _, pc := range h.config.Plugins {
		info, err := plugin.GetInfoOfID(pc.ID)
		if err != nil {
			panic(err)
		}
		if !info.CanHandle(proxyctx.ContextHTTP) {
			panic(errors.Errorf("plugin %s cannot handle context %s of this server", pc.ID, proxyctx.ContextHTTP))
		}
		plug, err := plugin.NewPlugin(pc)
		if err != nil {
			panic(err)
		}
		h.plugins = append(h.plugins, plug)
	}
	// init routers
	h.router = router.New()
	if len(h.config.Routers) == 0 {
		panic(errors.Errorf("empty router list of %s", h.config.ID))
	}
	for _, rc := range h.config.Routers {
		rt := NewHTTPRouter(rc, h.plugins)
		h.router.Handle(rt.Method(), rt.Path(), rt.Handler)
		h.httpRouters = append(h.httpRouters, rt)
	}
	h.server = &fasthttp.Server{
		Name:    string(h.config.ID),
		Handler: h.Handler,
	}
}

func (h *HTTPServer) Handler(ctx *fasthttp.RequestCtx) {
	//defaultHandler := plugin.DefaultTerminator
	//for _, plug := range h.plugins {
	//	defaultHandler = plug.Handler(defaultHandler)
	//}
	//err := defaultHandler(&proxyctx.HTTPContext{ctx})
	//if err != nil {
	//	if errors.Is(err, proxyctx.Return) {
	//		return
	//	}
	//	if e, ok := err.(*proxyctx.GeneralError); ok {
	//		e.WriteHttpCtx(ctx)
	//	} else {
	//		ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
	//	}
	//	return
	//}
	h.router.Handler(ctx)
}

//func (HTTPServer) fastHttpHandler(h plugin.HandleFunc) fasthttp.RequestHandler {
//	return func(ctx *fasthttp.RequestCtx) {
//		err := h(&proxyctx.HTTPContext{ctx})
//		if err != nil {
//			if errors.Is(err, proxyctx.Return) {
//				return
//			}
//			if e, ok := err.(*proxyctx.GeneralError); ok {
//				e.WriteHttpCtx(ctx)
//			} else {
//				ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
//			}
//		}
//	}
//}

func (h *HTTPServer) Serve() error {
	if len(h.listeners) == 1 {
		return h.server.Serve(h.listeners[0])
	}
	eg := errgroup.Group{}
	for _, l := range h.listeners {
		ln := l
		eg.Go(func() error { return h.server.Serve(ln) })
	}
	return eg.Wait()
}

func (h *HTTPServer) Stop() error {
	for _, r := range h.httpRouters {
		r.Destroy()
	}
	for _, p := range h.plugins {
		p.Destroy()
	}
	return h.server.Shutdown()
}
