package main

import (
	"bytes"
	"fmt"
	"github.com/fasthttp/router"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/revolution1/jsonrpc-proxy/jsonrpc"
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
	var resps []jsonrpc.RpcResponse
	ctx.SetUserValue("isRpcReq", true)
	reqBody := bytes.TrimSpace(ctx.Request.Body())
	// length of minimum valid request '{"jsonrpc":"2.0","method":"1","id":1}'
	if len(reqBody) < 37 {
		writeRpcErrResp(ctx, jsonrpc.ErrRpcParseError, nil)
		return
	}
	reqs, rpcErr := jsonrpc.ParseRequest(reqBody)
	if rpcErr != nil {
		writeRpcErrResp(ctx, rpcErr, nil)
		return
	}
	isMonoReq := reqBody[0] == '{' // && len(reqs) == 1
	if len(reqs) == 0 {
		writeRpcErrResp(ctx, jsonrpc.ErrRpcInvalidRequest, nil)
		return
	}
	methodNames := make([]string, len(reqs))
	for i, r := range reqs {
		methodNames[i] = r.Method
	}
	setCtxRpcMethods(ctx, methodNames)
	cacheFor := time.Duration(0)
	errFor := p.config.ErrFor.Duration
	allCached := true
	resps = make([]jsonrpc.RpcResponse, len(reqs))
	for idx, req := range reqs {
		if !req.Validate() {
			if isMonoReq {
				writeRpcErrResp(ctx, jsonrpc.ErrRpcInvalidRequest, req.Id)
				return
			}
			// if request is invalid, just set error
			jsonrpc.ErrRpcInvalidRequest.WriteToRpcResponse(&resps[idx], req.Id)
			continue
		}
		// skip cache if is valid req&upResp but no cache config set
		cc := p.config.Search(req.Method)
		if cc == nil {
			allCached = false
			RpcCacheMiss.WithLabelValues(req.Method).Inc()
			break
		}
		// use the minimum non-zero cache duration
		if cacheFor == 0 || cc.For.Duration < cacheFor {
			cacheFor = cc.For.Duration
		}
		if errFor == 0 || cc.For.Duration < errFor {
			errFor = cc.For.Duration
		}
		res := p.GetCachedItem(req, cc)
		if res == nil {
			RpcCacheMiss.WithLabelValues(req.Method).Inc()
			allCached = false
			break
		}
		// found cached
		RpcCacheHit.WithLabelValues(req.Method).Inc()
		if res.IsHttpResponse() && isMonoReq { // cached http error or something
			res.WriteHttpResponse(&ctx.Response)
			return
		} else if res.IsRpc() {
			if isMonoReq {
				resp := res.GetRpcResponse(req.Id)
				writeJsonResp(ctx, resp)
				return
			}
			res.WriteToRpcResponse(&resps[idx], req.Id)
		}
	}
	if allCached {
		writeJsonResps(ctx, resps)
		//for _, r := range resps {
		//	ReleaseRpcResponse(r)
		//}
		resps = nil
		return
	}
	// cache not found
	upResp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(upResp)
	setAcceptEncoding(ctx)
	err := p.um.DoTimeout(&ctx.Request, upResp, p.config.UpstreamRequestTimeout.Duration)
	// network errors
	if err != nil {
		log.WithError(err).WithField("methods", methodNames).Warn("error while requesting from upstream")
		log.WithError(err).Tracef("error while requesting from upstream: \n%s", &ctx.Request)
		e := jsonrpc.ErrWithData(jsonrpc.ErrRpcInternalError, err.Error())
		for idx, req := range reqs {
			p.SetCachedError(req, e, errFor)
			if isMonoReq {
				writeRpcErrResp(ctx, e, req.Id)
				return
			}
			e.WriteToRpcResponse(&resps[idx], req.Id)
		}
		writeJsonResps(ctx, resps)
		ctx.SetStatusCode(jsonrpc.StatusCodeOfRpcError(e))
		return
	}
	upRespBody, err := getResponseBody(upResp)
	// read body or decompression errors
	if upResp.StatusCode() < 200 || upResp.StatusCode() >= 400 || err != nil {
		log.WithError(err).WithField("methods", methodNames).Warn("fail to decode response, simply forward to client")
		log.Debug("decode error: ", err.Error())
		log.Tracef("request:\n%s\n\nresponse:\n%s", &ctx.Request, upResp)
		if isMonoReq {
			p.SetCachedResponse(reqs[0], upResp, errFor)
		}
		//upResp.CopyTo(&ctx.Response)
		p.forwardResponse(ctx, upResp)
		return
	}

	if isMonoReq {
		err = jsoniter.Unmarshal(upRespBody, &resps[0])
	} else {
		//for idx, _ := range reqs {
		//	resps[idx] = AcquireRpcResponse()
		//}
		//resps = []RpcResponse{}
		err = jsoniter.Unmarshal(upRespBody, &resps)
	}

	// json unmarshal errors
	if err != nil {
		log.WithError(err).WithField("methods", methodNames).WithField("res", string(upRespBody)).Warn("fail to decode response to json, simply forward to client")
		log.Debug("unmarshal error: ", err.Error())
		log.Debugf("response: \n%s", upResp)
		if isMonoReq {
			p.SetCachedResponse(reqs[0], upResp, errFor)
		} //upResp.CopyTo(&ctx.Response)
		p.forwardResponse(ctx, upResp)
		return
	}
	fasthttp.ReleaseResponse(upResp)
	for idx, resp := range resps {
		// jsonrpc errors
		if resp.Error != nil {
			if !resp.Error.Is(jsonrpc.ErrRpcInvalidRequest) {
				log.WithField("rpcErr", resp.Error).Tracef("rpc error while requesting from upstream: \n%s\n", reqs[idx])
				p.SetCachedError(reqs[idx], resp.Error, errFor)
			}
			continue
		}
		// no error, cache responses
		p.SetCachedRpcResponse(reqs[idx], &resp, cacheFor)
	}

	if isMonoReq {
		writeJsonResp(ctx, &resps[0])
	} else {
		writeJsonResps(ctx, resps)
	}
}

func (p *Proxy) SetCachedHttpError(req *jsonrpc.RpcRequest, code int, message []byte, errFor time.Duration) {
	key, err := req.ToCacheKey()
	if err != nil {
		return
	}
	err = p.CacheManager.Set(key, (&CachedItem{HttpResponse: &CachedHttpResp{Code: code, Body: message}}).Marshal(), errFor)
	if err != nil {
		log.WithError(err).Error("error while setting cached HTTP error")
	}
}

func (p *Proxy) SetCachedResponse(req *jsonrpc.RpcRequest, resp *fasthttp.Response, errFor time.Duration) {
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

func (p *Proxy) SetCachedError(req *jsonrpc.RpcRequest, e *jsonrpc.RpcError, errFor time.Duration) {
	key, err := req.ToCacheKey()
	if err != nil {
		return
	}
	err = p.CacheManager.Set(key, (&CachedItem{RpcError: e}).Marshal(), errFor)
	if err != nil {
		log.WithError(err).Error("error while setting cached error")
	}
}
func (p *Proxy) SetCachedRpcResponse(req *jsonrpc.RpcRequest, resp *jsonrpc.RpcResponse, cacheFor time.Duration) {
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

func (p *Proxy) GetCachedItem(req *jsonrpc.RpcRequest, cc *CacheConfig) *CachedItem {
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

func writeRpcErrResp(ctx *fasthttp.RequestCtx, rpcError *jsonrpc.RpcError, id interface{}) {
	ctx.ResetBody()
	ctx.SetUserValue("rpcErr", rpcError)
	ctx.SetBodyString(rpcError.JsonError(id))
	ctx.SetStatusCode(jsonrpc.StatusCodeOfRpcError(rpcError))
	ctx.SetContentType("application/json; charset=utf-8")
}

func getCtxRpcErr(ctx *fasthttp.RequestCtx) *jsonrpc.RpcError {
	if e, ok := ctx.UserValue("rpcErr").(*jsonrpc.RpcError); ok {
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

func setCtxRpcMethods(ctx *fasthttp.RequestCtx, methodNames []string) {
	ctx.SetUserValue("rpcMethods", methodNames)
}

func writeJsonResp(ctx *fasthttp.RequestCtx, resp *jsonrpc.RpcResponse) {
	data, err := jsoniter.Marshal(resp)
	if err != nil {
		log.WithError(err).Panic("fail to marshal response from cache")
	}
	writeJsonRespRaw(ctx, data, jsonrpc.StatusCodeOfRpcError(resp.Error))
	if resp.Error != nil {
		ctx.SetUserValue("rpcErr", resp.Error)
	}
}

func writeJsonResps(ctx *fasthttp.RequestCtx, resps []jsonrpc.RpcResponse) {
	data, err := jsoniter.Marshal(resps)
	if err != nil {
		log.WithError(err).Panic("fail to marshal response from cache")
	}
	status := 500
	for _, r := range resps {
		c := jsonrpc.StatusCodeOfRpcError(r.Error)
		if c < status {
			status = c
		}
	}
	writeJsonRespRaw(ctx, data, status)
}

func writeJsonRespRaw(ctx *fasthttp.RequestCtx, body []byte, code int) {
	ctx.Response.SetBody(body)
	if ctx.Request.Header.ConnectionClose() {
		ctx.Response.SetConnectionClose()
	}
	ctx.SetContentType("application/json; charset=utf-8")
	ctx.SetStatusCode(code)
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
