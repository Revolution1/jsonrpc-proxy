package main

import "sync"

//import "sync"
//
//var rpcReqPool sync.Pool
//
//
//
//func AcquireRpcReq() *RpcError {
//
//}

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
