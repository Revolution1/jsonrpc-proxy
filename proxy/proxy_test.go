package proxy

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/revolution1/jsonrpc-proxy/jsonrpc"
	"testing"
)

func TestProxy(t *testing.T) {
	t.Log(jsoniter.MarshalToString(&jsonrpc.RpcRequest{RpcHeader: jsonrpc.RpcHeader{Jsonrpc: jsonrpc.JSONRPC2, Id: 1}, Method: "", Params: jsoniter.RawMessage(`{"a":"b"}`)}))
}
