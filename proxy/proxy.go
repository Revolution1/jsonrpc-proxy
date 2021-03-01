package proxy

import (
	"bytes"
	"fmt"
	"github.com/fasthttp/router"
	jsoniter "github.com/json-iterator/go"
	"github.com/revolution1/jsonrpc-proxy/caching"
	"github.com/revolution1/jsonrpc-proxy/oldconfig"
	"github.com/revolution1/jsonrpc-proxy/jsonrpc"
	"github.com/revolution1/jsonrpc-proxy/metrics"
	"github.com/revolution1/jsonrpc-proxy/upstream"
	"github.com/savsgio/gotils"
	"github.com/savsgio/gotils/nocopy"
	"github.com/sirupsen/logrus"
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

	config       *oldconfig.Config
	CacheManager *caching.BigCacheManager
	um           *upstream.Manager

	httpServer *fasthttp.Server
	stats      Stats
	initOnce   sync.Once
}

func NewProxy(config *oldconfig.Config) *Proxy {
	return &Proxy{config: config}
}

func (p *Proxy) init() {
	p.um = upstream.NewUpstreamManager(p.config.Upstreams)
	p.CacheManager = caching.NewCacheManager()
	p.httpServer = &fasthttp.Server{
		Name:              "JSON-RPC Proxy Server",
		Handler:           fasthttp.CompressHandler(p.requestHandler),
		ErrorHandler:      nil,
		HeaderReceived:    nil,
		ContinueHandler:   nil,
		Concurrency:       0,
		DisableKeepalive:  false,
		ReduceMemoryUsage: false,
		Logger:            log,
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
	if log.Level == logrus.DebugLevel {
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
		WriteRpcErrResp(ctx, jsonrpc.ErrRpcParseError, nil)
		return
	}
	reqs, rpcErr := jsonrpc.ParseRequest(reqBody)
	if rpcErr != nil {
		WriteRpcErrResp(ctx, rpcErr, nil)
		return
	}
	isMonoReq := reqBody[0] == '{' // && len(reqs) == 1
	if len(reqs) == 0 {
		WriteRpcErrResp(ctx, jsonrpc.ErrRpcInvalidRequest, nil)
		return
	}
	methodNames := make([]string, len(reqs))
	for i, r := range reqs {
		methodNames[i] = r.Method
	}
	SetCtxRpcMethods(ctx, methodNames)
	cacheFor := time.Duration(0)
	errFor := p.config.ErrFor.Duration
	allCached := true
	resps = make([]jsonrpc.RpcResponse, len(reqs))
	for idx, req := range reqs {
		if !req.Validate() {
			if isMonoReq {
				WriteRpcErrResp(ctx, jsonrpc.ErrRpcInvalidRequest, req.Id)
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
			metrics.RpcCacheMiss.WithLabelValues(req.Method).Inc()
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
			metrics.RpcCacheMiss.WithLabelValues(req.Method).Inc()
			allCached = false
			break
		}
		// found cached
		metrics.RpcCacheHit.WithLabelValues(req.Method).Inc()
		if res.IsHttpResponse() && isMonoReq { // cached http error or something
			res.WriteHttpResponse(&ctx.Response)
			return
		} else if res.IsRpc() {
			if isMonoReq {
				resp := res.GetRpcResponse(req.Id)
				WriteJsonResp(ctx, resp)
				return
			}
			res.WriteToRpcResponse(&resps[idx], req.Id)
		}
	}
	if allCached {
		WriteJsonResps(ctx, resps)
		//for _, r := range resps {
		//	ReleaseRpcResponse(r)
		//}
		resps = nil
		return
	}
	// cache not found
	upResp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(upResp)
	SetAcceptEncoding(ctx)
	err := p.um.DoTimeout(&ctx.Request, upResp, p.config.UpstreamRequestTimeout.Duration)
	// network errors
	if err != nil {
		log.WithError(err).WithField("methods", methodNames).Warn("error while requesting from upstream")
		log.WithError(err).Tracef("error while requesting from upstream: \n%s", &ctx.Request)
		e := jsonrpc.ErrWithData(jsonrpc.ErrRpcInternalError, err.Error())
		for idx, req := range reqs {
			p.SetCachedError(req, e, errFor)
			if isMonoReq {
				WriteRpcErrResp(ctx, e, req.Id)
				return
			}
			e.WriteToRpcResponse(&resps[idx], req.Id)
		}
		WriteJsonResps(ctx, resps)
		ctx.SetStatusCode(jsonrpc.StatusCodeOfRpcError(e))
		return
	}
	upRespBody, err := GetResponseBody(upResp)
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
		WriteJsonResp(ctx, &resps[0])
	} else {
		WriteJsonResps(ctx, resps)
	}
}

func (p *Proxy) SetCachedHttpError(req *jsonrpc.RpcRequest, code int, message []byte, errFor time.Duration) {
	key, err := req.ToCacheKey()
	if err != nil {
		return
	}
	err = p.CacheManager.Set(key, (&caching.CachedItem{HttpResponse: &caching.CachedHttpResp{Code: code, Body: message}}).Marshal(), errFor)
	if err != nil {
		log.WithError(err).Error("error while setting cached HTTP error")
	}
}

func (p *Proxy) SetCachedResponse(req *jsonrpc.RpcRequest, resp *fasthttp.Response, errFor time.Duration) {
	key, err := req.ToCacheKey()
	if err != nil {
		return
	}
	err = p.CacheManager.Set(key, (&caching.CachedItem{HttpResponse: &caching.CachedHttpResp{
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
	err = p.CacheManager.Set(key, (&caching.CachedItem{RpcError: e}).Marshal(), errFor)
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
	err = p.CacheManager.Set(key, (&caching.CachedItem{Result: data}).Marshal(), cacheFor)
	if err != nil {
		log.WithError(err).Error("error while setting cached response")
	}
}

func (p *Proxy) GetCachedItem(req *jsonrpc.RpcRequest, cc *oldconfig.CacheConfig) *caching.CachedItem {
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
