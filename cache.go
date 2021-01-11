package main

import (
	"encoding/binary"
	"errors"
	"github.com/allegro/bigcache"
	log "github.com/sirupsen/logrus"
	"time"
)

type ProxyCache interface {
	Set(key string, val []byte, ttl time.Duration) error
	Get(key string) []byte
	Clear() error
}

type BigCacheTTL struct {
	*bigcache.BigCache
}

func NewBigCacheTTL(maxTTL, cleanWindow time.Duration, maxSizeMb int) *BigCacheTTL {
	c, err := bigcache.NewBigCache(bigcache.Config{
		Shards:             1024,
		LifeWindow:         maxTTL,
		CleanWindow:        cleanWindow,
		MaxEntriesInWindow: 1000 * 10 * 60,
		MaxEntrySize:       500,
		Verbose:            true,
		Hasher:             fnv64a{},
		HardMaxCacheSize:   maxSizeMb,
		Logger:             log.StandardLogger(),
	})
	if err != nil {
		panic(err)
	}
	return &BigCacheTTL{c}
}

func (c *BigCacheTTL) Set(key string, val []byte, ttl time.Duration) error {
	v := make([]byte, 8+len(val))
	binary.LittleEndian.PutUint64(v[:], uint64(time.Now().Add(ttl).UnixNano()))
	copy(v[8:], val)
	return c.BigCache.Set(key, v)
}

func (c *BigCacheTTL) Get(key string) []byte {
	val, err := c.BigCache.Get(key)
	if err != nil {
		if !errors.Is(err, bigcache.ErrEntryNotFound) {
			log.WithError(err).WithField("key", key).Debug("error while getting cache")
		}
		return nil
	}
	if len(val) < 8 {
		return nil
	}
	evict := time.Unix(0, int64(binary.LittleEndian.Uint64(val)))
	if !time.Now().Before(evict) {
		err := c.BigCache.Delete(key)
		if err != nil {
			log.WithError(err).WithField("key", key).Debug("delete cache error")
		}
		return nil
	}
	return val[8:]
}

func (c *BigCacheTTL) Clear() error {
	return c.BigCache.Reset()
}

func (c *BigCacheTTL) Iterator() {}
