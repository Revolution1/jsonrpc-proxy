package main

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/savsgio/gotils"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"go.uber.org/multierr"
	"time"
)

type CacheManager struct {
	cache1m    ProxyCache
	cache1h    ProxyCache
	cacheSolid ProxyCache
}

func NewCacheManager() *CacheManager {
	return &CacheManager{
		cache1m:    NewBigCacheTTL(time.Minute, 30*time.Second),
		cache1h:    NewBigCacheTTL(time.Hour, time.Minute),
		cacheSolid: NewBigCacheTTL(0, 0),
	}
}

func (c *CacheManager) Set(key string, val []byte, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	return c.getCacheForTTL(ttl).Set(key, val, ttl)
}

func (c *CacheManager) getCacheForTTL(ttl time.Duration) ProxyCache {
	switch {
	case ttl < time.Minute:
		return c.cache1m
	case ttl < time.Hour:
		return c.cache1h
	default:
		return c.cacheSolid
	}
}

func (c *CacheManager) Get(key string, suggestTTL time.Duration) []byte {
	if val := c.getCacheForTTL(suggestTTL).Get(key); val != nil {
		return val
	} else if val := c.cacheSolid.Get(key); val != nil {
		return val
	} else if val := c.cache1h.Get(key); val != nil {
		return val
	} else if val := c.cache1m.Get(key); val != nil {
		return val
	}
	return nil
}

func (c *CacheManager) GetItem(key string, suggestTTL time.Duration) *CachedItem {
	val := c.Get(key, suggestTTL)
	if val == nil {
		return nil
	}
	item := AcquireCachedItem()
	err := jsoniter.Unmarshal(val, item)
	if err != nil {
		log.WithError(err).Error("failed to unmarshal cached item")
		ReleaseCachedItem(item)
		return nil
	}
	return item
}

func (c *CacheManager) Clear() error {
	return multierr.Combine(
		c.cache1m.Clear(),
		c.cache1h.Clear(),
		c.cacheSolid.Clear(),
	)
}

type CachedHttpResp struct {
	Code            int    `json:"c,omitempty"`
	ContentEncoding []byte `json:"e,omitempty"`
	ContentType     []byte `json:"t,omitempty"`
	Body            []byte `json:"b,omitempty"`
}

type CachedItem struct {
	RpcError     *RpcError           `json:"e,omitempty"`
	Result       jsoniter.RawMessage `json:"r,omitempty"`
	HttpResponse *CachedHttpResp     `json:"h,omitempty"`
}

func (i *CachedItem) Marshal() []byte {
	d, _ := jsoniter.Marshal(i)
	return d
}

func (i *CachedItem) IsEmpty() bool {
	return i.RpcError == nil && i.HttpResponse == nil && len(i.Result) == 0
}

func (i *CachedItem) Reset() {
	i.RpcError = nil
	i.HttpResponse = nil
	i.Result = nil
}

func (i *CachedItem) IsRpc() bool {
	return i.RpcError != nil || i.Result != nil
}

func (i *CachedItem) IsRpcError() bool {
	return i.RpcError != nil
}

func (i *CachedItem) GetRpcError() *RpcError {
	return i.RpcError
}

func (i *CachedItem) IsHttpResponse() bool {
	return i.HttpResponse != nil
}

func (i *CachedItem) WriteHttpResponse(r *fasthttp.Response) {
	r.Header.SetStatusCode(i.HttpResponse.Code)
	r.Header.SetBytesV(fasthttp.HeaderContentEncoding, i.HttpResponse.ContentEncoding)
	r.Header.SetContentType(gotils.B2S(i.HttpResponse.ContentType))
	r.SetBody(i.HttpResponse.Body)
}

func (i *CachedItem) IsRpcResult() bool {
	return i.Result != nil
}

func (i *CachedItem) GetRpcResponse(id interface{}) *RpcResponse {
	r := &RpcResponse{rpcHeader: rpcHeader{Jsonrpc: JSONRPC2, Id: id}}
	if i.RpcError != nil {
		r.Error = i.RpcError
	} else if i.Result != nil {
		r.Result = i.Result
	}
	return r
}

type CachedItems []CachedItem
