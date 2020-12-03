package main

import "time"

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

func (c *CacheManager) Clear() error {
	panic("implement me")
}
