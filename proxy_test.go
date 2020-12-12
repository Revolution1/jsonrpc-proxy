package main

import (
	jsoniter "github.com/json-iterator/go"
	"testing"
)

func TestProxy(t *testing.T) {
	t.Log(jsoniter.MarshalToString(&RpcRequest{rpcHeader: rpcHeader{Jsonrpc: JSONRPC2, Id: 1}, Method: "", Params: jsoniter.RawMessage(`{"a":"b"}`)}))
}
