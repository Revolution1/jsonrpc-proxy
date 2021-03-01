package caching

import (
	"github.com/dustin/go-humanize"
	jsoniter "github.com/json-iterator/go"
	"github.com/revolution1/jsonrpc-proxy/jsonrpc"
	"github.com/revolution1/jsonrpc-proxy/utils"
	assertion "github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"testing"
	"time"
)

var testCacheSize int64 = 1 * humanize.MiByte

func TestRisCache(t *testing.T) {
	assert := assertion.New(t)
	c := NewRisCache(testCacheSize)

	i, e := c.IncrInt64By("foo", 5, 0)
	assert.NoError(e)
	assert.Equal(int64(5), i)
	time.Sleep(10 * time.Millisecond)
	i, e = c.GetInt64("foo")
	assert.NoError(e)
	assert.Equal(int64(5), i)

	i, e = c.IncrInt64By("foo", 1, 0)
	assert.NoError(e)
	assert.Equal(int64(6), i)
	time.Sleep(10 * time.Millisecond)
	i, e = c.GetInt64("foo")
	assert.NoError(e)
	assert.Equal(int64(6), i)

	i, e = c.IncrInt64By("foo", -2, 0)
	assert.NoError(e)
	assert.Equal(int64(4), i)
	time.Sleep(10 * time.Millisecond)
	i, e = c.GetInt64("foo")
	assert.NoError(e)
	assert.Equal(int64(4), i)
}

func TestRisCapacity(t *testing.T) {
	assert := assertion.New(t)
	const key = "key"
	c := NewRisCache(testCacheSize)
	ok := c.SetBytes(key, utils.Blob('x', 1024*1024), 0)
	assert.True(ok)
	time.Sleep(10 * time.Millisecond)
	v, err := c.GetBytes(key)
	assert.Error(err)

	c = NewRisCache(2 * testCacheSize)
	ok = c.SetBytes(key, utils.Blob('x', 1024*1024), 0)
	assert.True(ok)
	time.Sleep(10 * time.Millisecond)
	v, err = c.GetBytes(key)
	assert.NoError(err)
	assert.Equal(1024*1024, len(v))
}

func TestRisCachedItem(t *testing.T) {
	assert := assertion.New(t)
	c := NewRisCache(testCacheSize)
	req := &jsonrpc.RpcRequest{
		Method: "blah",
		Params: []string{"a", "b"},
	}

	resp := &jsonrpc.RpcResponse{
		Result: "abc",
	}
	assert.NoError(c.SetRpcCache(req, resp, 0))
	time.Sleep(10 * time.Millisecond)
	item := c.GetItem(req)
	assert.NotNil(item)
	if item != nil {
		assert.True(item.IsRpc())
		assert.Equal(jsoniter.RawMessage("\"abc\""), item.Result)
	}

	errResp := &jsonrpc.RpcResponse{
		Error: jsonrpc.ErrRpcInvalidRequest,
	}
	assert.NoError(c.SetRpcCache(req, errResp, 0))
	time.Sleep(10 * time.Millisecond)
	item = c.GetItem(req)
	assert.NotNil(item)
	if item != nil {
		assert.True(item.IsRpcError())
		assert.EqualError(item.RpcError, jsonrpc.ErrRpcInvalidRequest.Error())
	}

	httpResp := fasthttp.AcquireResponse()
	httpResp.SetStatusCode(200)
	httpResp.SetBodyRaw([]byte("body content"))
	httpResp.Header.SetContentType("content-type")
	httpResp.Header.Set(fasthttp.HeaderContentEncoding, "utf8")
	assert.NoError(c.SetHttpCache(req, httpResp, 0))
	time.Sleep(10 * time.Millisecond)
	item = c.GetItem(req)
	assert.NotNil(item)
	if item != nil {
		assert.True(item.IsHttpResponse())
		assert.Equal([]byte("utf8"), item.HttpResponse.ContentEncoding)
	}
}
