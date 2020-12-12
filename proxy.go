package main

import (
	"bytes"
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
	setCtxRpcMethods(ctx, reqs)
	switch len(reqs) {
	case 0:
		setRpcErr(ctx, ErrRpcInvalidRequest)
		return
	case 1:
		req := reqs[0]
		if !req.Validate() {
			setRpcErr(ctx, ErrRpcInvalidRequest)
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
			switch {
			case res.IsHttpResponse():
				res.WriteHttpResponse(&ctx.Response)
			case res.IsRpc():
				resp := res.GetRpcResponse(req.Id)
				data, err := jsoniter.Marshal(resp)
				if err != nil {
					log.WithError(err).Panic("fail to marshal response from cache")
				}
				writeJsonResp(ctx, data)
			default:
				log.WithField("res", res).Panic("fail to process item from cache")
			}
			return
		}
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)
		err := p.um.DoTimeout(&ctx.Request, resp, p.config.UpstreamRequestTimeout.Duration)
		if err != nil {
			log.WithError(err).WithField("req", &ctx.Request).Trace("error while requesting from upstream")
			e := ErrWithData(ErrRpcInternalError, err.Error())
			p.SetCachedError(&req, e, errFor)
			setRpcErr(ctx, e)
			return
		}
		respBody, err := getResponseBody(resp)
		rpcResp := &RpcResponse{}
		if resp.StatusCode() < 200 || resp.StatusCode() >= 400 || err != nil {
			log.WithError(err).Error("fail to decode response, simply forward to client")
			log.Tracef("response: \n%s", resp.String())
			p.SetCachedResponse(req, resp, errFor)
			//resp.CopyTo(&ctx.Response)
			p.forwardResponse(ctx, resp)
			return
		}
		err = jsoniter.Unmarshal(respBody, rpcResp)
		if err != nil {
			log.WithError(err).Error("fail to decode response to json, simply forward to client")
			log.Tracef("response: \n%s", resp.String())
			p.SetCachedResponse(req, resp, errFor)
			//resp.CopyTo(&ctx.Response)
			p.forwardResponse(ctx, resp)
			return
		}
		fasthttp.ReleaseResponse(resp)
		if rpcResp.Error != nil {
			log.WithField("rpcErr", rpcResp.Error).WithField("req", &ctx.Request).Trace("rpc error while requesting from upstream")
			p.SetCachedError(&req, rpcResp.Error, errFor)
			setRpcErr(ctx, rpcResp.Error)
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
		return nil
	}
	return p.CacheManager.GetItem(key, cc.For.Duration)
}

//func (p *Proxy) GetCachedValues(reqs []RpcRequest) []byte {
//	resps := make([]jsoniter.RawMessage, len(reqs))
//	for _, req := range reqs {
//		r := p.GetCachedValue(&req, nil)
//		if r == nil {
//			return nil
//		}
//		resps = append(resps, r)
//	}
//	if len(resps) > 0 {
//		ret, err := jsoniter.Marshal(resps)
//		if err != nil {
//			log.WithError(err).Error("error while marshaling responses from cache")
//		}
//		return ret
//	}
//	return nil
//}

func (p *Proxy) simpleForward(ctx *fasthttp.RequestCtx) {
	err := p.um.DoTimeout(&ctx.Request, &ctx.Response, p.config.UpstreamRequestTimeout.Duration)
	log.WithError(err).Debug("direct pass")
}

func (p *Proxy) forwardResponse(ctx *fasthttp.RequestCtx, response *fasthttp.Response) {
	r := &ctx.Response
	r.SetStatusCode(response.StatusCode())
	r.Header.SetContentType(gotils.B2S(response.Header.ContentType()))
	r.Header.SetBytesV(fasthttp.HeaderContentEncoding, response.Header.Peek(fasthttp.HeaderContentEncoding))
	_, _ = response.WriteTo(ctx)
}

func setRpcErr(ctx *fasthttp.RequestCtx, rpcError *RpcError) {
	ctx.ResetBody()
	ctx.SetUserValue("rpcErr", rpcError)
	ctx.SetBodyString(rpcError.JsonError())
	ctx.SetStatusCode(200)
	ctx.SetContentType("application/json; charset=utf-8")
}

func getCtxRpcErr(ctx *fasthttp.RequestCtx) *RpcError {
	if e, ok := ctx.UserValue("rpcErr").(*RpcError); ok {
		return e
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

var (
	strGzip    = []byte("gzip")
	strBr      = []byte("br")
	strDeflate = []byte("deflate")
)

var ErrUnknownContentEncoding = errors.New("Unknown Content Encoding")

func getResponseBody(resp *fasthttp.Response) ([]byte, error) {
	encoding := resp.Header.Peek(fasthttp.HeaderContentEncoding)
	switch {
	case encoding == nil:
		return resp.Body(), nil
	case bytes.Equal(encoding, strGzip):
		return resp.BodyGunzip()
	case bytes.Equal(encoding, strBr):
		return resp.BodyUnbrotli()
	case bytes.Equal(encoding, strDeflate):
		return resp.BodyInflate()
	}
	return nil, ErrUnknownContentEncoding
}
