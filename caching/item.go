package caching

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/revolution1/jsonrpc-proxy/jsonrpc"
	"github.com/savsgio/gotils"
	"github.com/valyala/fasthttp"
	"sync"
)

func (c *CachedHttpResp) Size() int {
	if c == nil {
		return 0
	}
	return 8 + len(c.ContentType) + len(c.ContentEncoding) + len(c.Body)
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

func (i *CachedItem) GetRpcError() *jsonrpc.RpcError {
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

func (i *CachedItem) GetRpcResponse(id interface{}) *jsonrpc.RpcResponse {
	r := jsonrpc.AcquireRpcResponse()
	r.Id = id
	if i.RpcError != nil {
		r.Error = i.RpcError
	} else if i.Result != nil {
		r.Result = i.Result
	}
	return r
}

func (i *CachedItem) WriteToRpcResponse(r *jsonrpc.RpcResponse, id interface{}) {
	r.Jsonrpc = jsonrpc.JSONRPC2
	r.Id = id
	if i.RpcError != nil {
		r.Error = i.RpcError
	} else if i.Result != nil {
		r.Result = i.Result
	}
}

type CachedItems []CachedItem

var cachedItemPool = sync.Pool{New: func() interface{} { return &CachedItem{} }}

func AcquireCachedItem() *CachedItem {
	v := cachedItemPool.Get()
	if v == nil {
		return &CachedItem{}
	}
	return v.(*CachedItem)
}

func ReleaseCachedItem(item *CachedItem) {
	item.Reset()
	cachedItemPool.Put(item)
}
