package main

import (
	assertion "github.com/stretchr/testify/assert"
	"strconv"
	"testing"
	"time"
)

func TestBigCacheTTL(t *testing.T) {
	assert := assertion.New(t)
	c := NewBigCacheTTL(time.Second, time.Second, 256)
	assert.Nil(c.Get("a"))
	assert.NoError(c.Set("1", []byte("val"), time.Millisecond))
	assert.Equal([]byte("val"), c.Get("1"))
	time.Sleep(2 * time.Millisecond)
	assert.Nil(c.Get("1"))
	assert.NoError(c.Clear())

	c = NewBigCacheTTL(0, 0, 256)
	assert.NoError(c.Set("1", []byte("val"), time.Millisecond))
	assert.Equal([]byte("val"), c.Get("1"))
	time.Sleep(2 * time.Millisecond)
	v, e := c.BigCache.Get("1")
	assert.NoError(e)
	assert.Equal([]byte("val"), v[8:])
	assert.Nil(c.Get("1"))
	t.Log(c.Stats())
}

func BenchmarkBigCacheTTL(b *testing.B) {
	c := NewBigCacheTTL(time.Second, time.Second, 256)
	for i := 0; i < b.N; i++ {
		_ = c.Set(strconv.Itoa(i), []byte("test val"), time.Millisecond)
		_ = c.Get(strconv.Itoa(i - 1))
	}
}
