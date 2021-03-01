package caching

import (
	"github.com/dgraph-io/ristretto"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/revolution1/jsonrpc-proxy/jsonrpc"
	"github.com/valyala/fasthttp"
	"sync/atomic"
	"time"
)

var (
	ErrCacheDropped             = errors.New("cache setting failed: dropped")
	ErrCacheKeyNotFound         = errors.New("key not found")
	ErrCacheKeySetFail          = errors.New("key set fail")
	ErrCacheValueIsNotBytesType = errors.New("value is not bytes type")
	ErrCacheValueIsNotInt64Type = errors.New("value is not int64 type")
)

type RisCache struct {
	*ristretto.Cache
}

func NewRisCache(maxSizeBytes int64) *RisCache {
	c, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,
		MaxCost:     maxSizeBytes,
		BufferItems: 64,
		Metrics:     false,
		OnEvict:     evictItem,
		KeyToHash:   nil,
		Cost:        nil,
	})
	return &RisCache{c}
}

func (c *RisCache) SetBytes(key string, val []byte, ttl time.Duration) bool {
	cost := int64(len(key) + len(val))
	return c.Cache.SetWithTTL(key, val, cost, ttl)
}

func (c *RisCache) GetBytes(key string) ([]byte, error) {
	v, ok := c.Cache.Get(key)
	if !ok {
		return nil, ErrCacheKeyNotFound
	}
	b, ok := v.([]byte)
	if !ok {
		return nil, ErrCacheValueIsNotBytesType
	}
	return b, nil
}

func (c *RisCache) SetInt64(key string, val int64, ttl time.Duration) bool {
	return c.Cache.SetWithTTL(key, &val, int64(len(key)+8), ttl)
}

func (c *RisCache) GetInt64(key string) (int64, error) {
	v, ok := c.Cache.Get(key)
	if !ok {
		return 0, ErrCacheKeyNotFound
	}
	b, ok := v.(*int64)
	if !ok {
		return 0, ErrCacheValueIsNotBytesType
	}
	return *b, nil
}

func (c *RisCache) IncrInt64By(key string, by int64, ttl time.Duration) (int64, error) {
	v, ok := c.Cache.Get(key)
	if !ok {
		ok = c.SetInt64(key, by, ttl)
		if !ok {
			return 0, ErrCacheKeySetFail
		}
		return by, nil
	}
	b, ok := v.(*int64)
	if !ok {
		return 0, ErrCacheValueIsNotInt64Type
	}
	atomic.AddInt64(b, by)
	return *b, nil
}

func (c *RisCache) SetFloat64(key string, val float64, ttl time.Duration) bool {
	return c.Cache.SetWithTTL(key, val, int64(len(key)+8), ttl)
}

func (c *RisCache) SetRpcCache(req *jsonrpc.RpcRequest, resp *jsonrpc.RpcResponse, ttl time.Duration) error {
	key, err := req.ToCacheKey()
	if err != nil {
		return err
	}
	item := AcquireCachedItem()
	item.RpcError = resp.Error
	result, err := jsoniter.Marshal(resp.Result)
	if err != nil {
		return err
	}
	item.Result = result
	item.HttpResponse = nil
	ok := c.SetWithTTL(key, item, int64(len(key)+len(result)), ttl)
	if !ok {
		return ErrCacheDropped
	}
	return nil
}

func (c *RisCache) SetHttpCache(req *jsonrpc.RpcRequest, resp *fasthttp.Response, ttl time.Duration) error {
	key, err := req.ToCacheKey()
	if err != nil {
		return err
	}
	item := AcquireCachedItem()
	item.Result = nil
	item.RpcError = nil
	item.HttpResponse = &CachedHttpResp{
		Code:            resp.StatusCode(),
		ContentEncoding: resp.Header.Peek(fasthttp.HeaderContentEncoding),
		ContentType:     resp.Header.ContentType(),
		Body:            resp.Body(),
	}
	ok := c.SetWithTTL(key, item, int64(len(key)+item.HttpResponse.Size()), ttl)
	if !ok {
		return ErrCacheDropped
	}
	return nil
}

func (c *RisCache) GetItem(req *jsonrpc.RpcRequest) *CachedItem {
	key, err := req.ToCacheKey()
	if err != nil {
		return nil
	}
	item, ok := c.Get(key)
	if !ok {
		return nil
	}
	i, ok := item.(*CachedItem)
	if !ok {
		return nil
	}
	return i
}

func (c *RisCache) Clear() {
	c.Cache.Clear()
}

func evictItem(key, conflict uint64, value interface{}, cost int64) {
	if i, ok := value.(*CachedItem); ok {
		ReleaseCachedItem(i)
	}
}
