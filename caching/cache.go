package caching

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/revolution1/jsonrpc-proxy/jsonrpc"
	"github.com/valyala/fasthttp"
	"time"
)

type ProxyCache interface {
	Set(key string, val []byte, ttl time.Duration) error
	Get(key string) []byte
	Clear()
}

type CacheManager interface {
	SetRpcCache(req *jsonrpc.RpcRequest, resp *jsonrpc.RpcResponse, ttl time.Duration) error
	SetHttpCache(req *jsonrpc.RpcRequest, resp *fasthttp.Response, ttl time.Duration) error
	GetItem(req *jsonrpc.RpcRequest) *CachedItem
	Clear()
}

type CachedHttpResp struct {
	Code            int    `json:"c,omitempty"`
	ContentEncoding []byte `json:"e,omitempty"`
	ContentType     []byte `json:"t,omitempty"`
	Body            []byte `json:"b,omitempty"`
}

type CachedItem struct {
	RpcError     *jsonrpc.RpcError   `json:"e,omitempty"`
	Result       jsoniter.RawMessage `json:"r,omitempty"`
	HttpResponse *CachedHttpResp     `json:"h,omitempty"`
}
