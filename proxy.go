package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/fasthttp/router"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/savsgio/gotils"
	"github.com/savsgio/gotils/nocopy"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/pprofhandler"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Stats struct {
	CacheHit  uint64
	CacheMiss uint64
}

type Proxy struct {
	nocopy.NoCopy

	config       *Config
	CacheManager *CacheManager
	um           *UpstreamManager

	httpServer *fasthttp.Server
	stats      Stats
	initOnce   sync.Once
}

func NewProxy(config *Config) *Proxy {
	return &Proxy{config: config}
}

func (p *Proxy) init() {
	p.um = NewUpstreamManager(p.config.Upstreams)
	p.CacheManager = NewCacheManager()
	p.httpServer = &fasthttp.Server{
		Name:              "JSON-RPC Proxy Server",
		Handler:           fasthttp.CompressHandler(p.requestHandler),
		ErrorHandler:      nil,
		HeaderReceived:    nil,
		ContinueHandler:   nil,
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
	reqBody := bytes.TrimSpace(ctx.Request.Body())
	// length of minimum valid request '{"jsonrpc":"2.0","method":"1","id":1}'
	if len(reqBody) < 37 {
		setRpcErr(ctx, ErrRpcParseError, nil)
		return
	}
	reqs, err := ParseRequest(reqBody)
	if err != nil {
		setRpcErr(ctx, err, nil)
		return
	}
	setCtxRpcMethods(ctx, reqs)
	//var isMonoReq bool
	switch len(reqs) {
	case 0:
		setRpcErr(ctx, ErrRpcInvalidRequest, nil)
		return
	case 1:
		//isMonoReq = true
		req := reqs[0]
		if !req.Validate() {
			setRpcErr(ctx, ErrRpcInvalidRequest, req.Id)
			return
		}
		// skip cache if is valid req&resp but no cache config set
		cacheFor := time.Duration(0)
		errFor := p.config.ErrFor.Duration
		cc := p.config.Search(req.Method)
		if cc != nil {
			cacheFor = cc.For.Duration
			errFor = cc.For.Duration
		}
		res := p.GetCachedItem(&req, cc)
		// found cached
		if res != nil {
			RpcCacheHit.WithLabelValues(req.Method).Inc()
			switch {
			case res.IsHttpResponse():
				res.WriteHttpResponse(&ctx.Response)
			case res.IsRpc():
				resp := res.GetRpcResponse(req.Id)
				data, err := jsoniter.Marshal(resp)
				if err != nil {
					log.WithError(err).Panic("fail to marshal response from cache")
				}
				if res.IsRpcError() {
					setRpcErr(ctx, resp.Error, req.Id)
				}
				writeJsonResp(ctx, data)
			default:
				log.WithField("res", res).Panic("fail to process item from cache")
			}
			return
		}
		RpcCacheMiss.WithLabelValues(req.Method).Inc()
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)
		setAcceptEncoding(ctx)
		err := p.um.DoTimeout(&ctx.Request, resp, p.config.UpstreamRequestTimeout.Duration)
		if err != nil {
			log.WithError(err).WithField("req", req).Warn("error while requesting from upstream")
			log.WithError(err).Debugf("error while requesting from upstream: %s", &ctx.Request)
			e := ErrWithData(ErrRpcInternalError, err.Error())
			p.SetCachedError(&req, e, errFor)
			setRpcErr(ctx, e, req.Id)
			return
		}
		respBody, err := getResponseBody(resp)
		rpcResp := &RpcResponse{}
		if resp.StatusCode() < 200 || resp.StatusCode() >= 400 || err != nil {
			log.WithError(err).WithField("req", req).Warn("fail to decode response, simply forward to client")
			log.Debug("decode error: ", err.Error())
			log.Debug("request:\n%s\n\nresponse:\n%s", &ctx.Request, resp)
			p.SetCachedResponse(req, resp, errFor)
			//resp.CopyTo(&ctx.Response)
			p.forwardResponse(ctx, resp)
			return
		}
		err = jsoniter.Unmarshal(respBody, rpcResp)
		if err != nil {
			err = json.Unmarshal(respBody, rpcResp)
		}
		if err != nil {
			log.WithError(err).WithField("req", req).WithField("res", string(respBody)).Warn("fail to decode response to json, simply forward to client")
			log.Debug("unmarshal error: ", err.Error())
			log.Debugf("response: \n%s", resp)
			p.SetCachedResponse(req, resp, errFor)
			//resp.CopyTo(&ctx.Response)
			p.forwardResponse(ctx, resp)
			return
		}
		fasthttp.ReleaseResponse(resp)
		if rpcResp.Error != nil {
			log.WithField("rpcErr", rpcResp.Error).Tracef("rpc error while requesting from upstream: \n%s\n", ctx.Request.Body())
			p.SetCachedError(&req, rpcResp.Error, errFor)
			setRpcErr(ctx, rpcResp.Error, req.Id)
			return
		}
		p.SetCachedRpcResponse(&req, rpcResp, cacheFor)
		data, _ := jsoniter.Marshal(rpcResp)
		writeJsonResp(ctx, data)
		return
	default:
		log.Panic("batch request not implemented yet")
	}

	p.simpleForward(ctx)
	log.Debug("end request")
}

func (p *Proxy) SetCachedHttpError(req RpcRequest, code int, message []byte, errFor time.Duration) {
	key, err := req.ToCacheKey()
	if err != nil {
		return
	}
	err = p.CacheManager.Set(key, (&CachedItem{HttpResponse: &CachedHttpResp{Code: code, Body: message}}).Marshal(), errFor)
	if err != nil {
		log.WithError(err).Error("error while setting cached HTTP error")
	}
}

func (p *Proxy) SetCachedResponse(req RpcRequest, resp *fasthttp.Response, errFor time.Duration) {
	key, err := req.ToCacheKey()
	if err != nil {
		return
	}
	err = p.CacheManager.Set(key, (&CachedItem{HttpResponse: &CachedHttpResp{
		Code:            resp.StatusCode(),
		ContentEncoding: resp.Header.Peek(fasthttp.HeaderContentEncoding),
		ContentType:     resp.Header.ContentType(),
		Body:            resp.Body(),
	}}).Marshal(), errFor)
	if err != nil {
		log.WithError(err).Error("error while setting cached HTTP error")
	}
}

func (p *Proxy) SetCachedError(req *RpcRequest, e *RpcError, errFor time.Duration) {
	key, err := req.ToCacheKey()
	if err != nil {
		return
	}
	err = p.CacheManager.Set(key, (&CachedItem{RpcError: e}).Marshal(), errFor)
	if err != nil {
		log.WithError(err).Error("error while setting cached error")
	}
}
func (p *Proxy) SetCachedRpcResponse(req *RpcRequest, resp *RpcResponse, cacheFor time.Duration) {
	key, err := req.ToCacheKey()
	if err != nil {
		return
	}
	data, err := jsoniter.Marshal(resp.Result)
	if err != nil {
		log.WithError(err).Error("error while serializing cached response")
	}
	err = p.CacheManager.Set(key, (&CachedItem{Result: data}).Marshal(), cacheFor)
	if err != nil {
		log.WithError(err).Error("error while setting cached response")
	}
}

func (p *Proxy) GetCachedItem(req *RpcRequest, cc *CacheConfig) *CachedItem {
	dur := time.Duration(0)
	key, err := req.ToCacheKey()
	if err != nil {
		log.WithError(err).WithField("req", req).Error("error while request.ToCacheKey()")
		return nil
	}
	if cc == nil {
		cc = p.config.Search(req.Method)
	}
	if cc == nil {
		log.WithField("method", req.Method).Trace("Cache config not found for method")
	} else {
		dur = cc.For.Duration
	}
	return p.CacheManager.GetItem(key, dur)
}

func (p *Proxy) simpleForward(ctx *fasthttp.RequestCtx) {
	err := p.um.DoTimeout(&ctx.Request, &ctx.Response, p.config.UpstreamRequestTimeout.Duration)
	log.WithError(err).Debug("direct pass")
}

func (p *Proxy) forwardResponse(ctx *fasthttp.RequestCtx, response *fasthttp.Response) {
	r := &ctx.Response
	r.SetStatusCode(response.StatusCode())
	r.Header.SetContentType(gotils.B2S(response.Header.ContentType()))
	r.Header.SetBytesV(fasthttp.HeaderContentEncoding, response.Header.Peek(fasthttp.HeaderContentEncoding))
	_ = response.BodyWriteTo(ctx)
}

func setRpcErr(ctx *fasthttp.RequestCtx, rpcError *RpcError, id interface{}) {
	ctx.ResetBody()
	ctx.SetUserValue("rpcErr", rpcError)
	ctx.SetBodyString(rpcError.JsonError(id))
	ctx.SetStatusCode(200)
	ctx.SetContentType("application/json; charset=utf-8")
}

func getCtxRpcErr(ctx *fasthttp.RequestCtx) *RpcError {
	if e, ok := ctx.UserValue("rpcErr").(*RpcError); ok {
		return e
	}
	return nil
}
func getCtxRpcMethods(ctx *fasthttp.RequestCtx) []string {
	if m, ok := ctx.UserValue("rpcMethods").([]string); ok {
		return m
	}
	return nil
}

func setCtxRpcMethods(ctx *fasthttp.RequestCtx, reqs []RpcRequest) {
	methods := make([]string, len(reqs))
	for i, r := range reqs {
		methods[i] = r.Method
	}
	ctx.SetUserValue("rpcMethods", methods)
}

func writeJsonResp(ctx *fasthttp.RequestCtx, body []byte) {
	ctx.Response.SetBody(body)
	if ctx.Request.Header.ConnectionClose() {
		ctx.Response.SetConnectionClose()
	}
	ctx.SetContentType("application/json; charset=utf-8")
	ctx.SetStatusCode(fasthttp.StatusOK)
}

const (
	strGzip    = "gzip"
	strBr      = "br"
	strDeflate = "deflate"
)

var ErrUnknownContentEncoding = errors.New("Unknown Content Encoding")

func getResponseBody(resp *fasthttp.Response) ([]byte, error) {
	encoding := string(bytes.TrimSpace(resp.Header.Peek(fasthttp.HeaderContentEncoding)))
	switch encoding {
	case strGzip:
		return resp.BodyGunzip()
	case strBr:
		return resp.BodyUnbrotli()
	case strDeflate:
		return resp.BodyInflate()
	default:
		body := resp.Body()
		//if !isASCII(body) {
		//	// give it a try
		//	b, err := resp.BodyGunzip()
		//	if err != nil {
		//		return body, nil
		//	}
		//	return b, nil
		//}
		return body, nil
	}
	//return resp.Body(), errors.Wrapf(ErrUnknownContentEncoding, "encoding: '%s'", encoding)
}

func setAcceptEncoding(ctx *fasthttp.RequestCtx) {
	ctx.Request.Header.Del(fasthttp.HeaderAcceptEncoding)
}
