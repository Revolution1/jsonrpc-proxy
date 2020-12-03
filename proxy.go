package main

import (
	"bytes"
	"fmt"
	"github.com/fasthttp/router"
	"github.com/savsgio/gotils/nocopy"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/pprofhandler"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type Stats struct {
	CacheHit  uint64
	CacheMiss uint64
}

type Proxy struct {
	nocopy.NoCopy

	config       *Config
	CacheManager *CacheManager
	LBClient     *fasthttp.LBClient

	httpServer *fasthttp.Server
	stats      Stats
	initOnce   sync.Once
}

func NewProxy(config *Config) *Proxy {
	return &Proxy{config: config}
}

func (p *Proxy) init() {
	p.LBClient = &fasthttp.LBClient{Timeout: p.config.Timeout.Duration}
	for _, addr := range p.config.Upstreams {
		c := &fasthttp.HostClient{Addr: ParseUpstream(addr)}
		p.LBClient.Clients = append(p.LBClient.Clients, c)
	}

	p.httpServer = &fasthttp.Server{
		Handler:           fasthttp.CompressHandler(p.requestHandler),
		ErrorHandler:      nil,
		HeaderReceived:    nil,
		ContinueHandler:   nil,
		Name:              "JSON-RPC Proxy Server",
		Concurrency:       0,
		DisableKeepalive:  false,
		ReduceMemoryUsage: false,
		Logger:            log.StandardLogger(),
	}
}

func (p *Proxy) RegisterHandler(r *router.Router) {
	p.initOnce.Do(p.init)
	r.GET("/", func(ctx *fasthttp.RequestCtx) {
		_, _ = fmt.Fprint(ctx, "JSON-RPC Proxy, please request with POST Method")
	})
	r.POST(p.config.Path, p.requestHandler)
}

func (p *Proxy) Serve() error {
	r := router.New()
	if log.GetLevel() == log.DebugLevel {
		r.GET("/debug", pprofhandler.PprofHandler)
	}
	ch := make(chan error)

	go func() {
		defer close(ch)
		if err := fasthttp.ListenAndServe(p.config.Listen, r.Handler); err != nil {
			ch <- err
		}
	}()
	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, os.Interrupt, os.Kill, syscall.SIGTERM)
	select {
	case err := <-ch:
		return err
	case <-sigCh:
		return p.httpServer.Shutdown()
	}
}

func (p *Proxy) requestHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetUserValue("isRpcReq", true)
	body := bytes.TrimSpace(ctx.Request.Body())
	// length of minimum valid request '{"jsonrpc":"2.0","method":"1","id":1}'
	if len(body) < 37 {
		setRpcErr(ctx, ErrRpcParseError)
		return
	}
	reqs, err := ParseRequest(body)
	if err != nil {
		setRpcErr(ctx, err)
		return
	}
	methods := make([]string, len(reqs))
	for i, r := range reqs {
		methods[i] = r.Method
	}
	ctx.SetUserValue("rpcMethods", methods)
}

func (p *Proxy) directPass(ctx *fasthttp.RequestCtx) {

}

func setRpcErr(ctx *fasthttp.RequestCtx, rpcError *RpcError) {
	ctx.ResetBody()
	ctx.SetUserValue("rpcErr", rpcError)
	ctx.SetBodyString(rpcError.JsonError())
	ctx.SetStatusCode(200)
}
