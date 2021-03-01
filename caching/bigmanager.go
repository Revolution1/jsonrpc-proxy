package caching

import (
	jsoniter "github.com/json-iterator/go"
	"time"
)

type BigCacheManager struct {
	cache1m    ProxyCache
	cache1h    ProxyCache
	cacheSolid ProxyCache
}

func NewCacheManager() *BigCacheManager {
	return &BigCacheManager{
		cache1m:    NewBigCacheTTL(time.Minute, 30*time.Second, 64),
		cache1h:    NewBigCacheTTL(time.Hour, time.Minute, 128),
		cacheSolid: NewBigCacheTTL(0, 0, 256),
	}
}

func (c *BigCacheManager) Set(key string, val []byte, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	return c.getCacheForTTL(ttl).Set(key, val, ttl)
}

func (c *BigCacheManager) getCacheForTTL(ttl time.Duration) ProxyCache {
	switch {
	case ttl < time.Minute:
		return c.cache1m
	case ttl < time.Hour:
		return c.cache1h
	default:
		return c.cacheSolid
	}
}

func (c *BigCacheManager) Get(key string, suggestTTL time.Duration) []byte {
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

func (c *BigCacheManager) GetItem(key string, suggestTTL time.Duration) *CachedItem {
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

func (c *BigCacheManager) Clear() {
	c.cache1m.Clear()
	c.cache1h.Clear()
	c.cacheSolid.Clear()
}
