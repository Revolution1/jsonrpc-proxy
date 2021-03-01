package proxyctx

import (
	"github.com/revolution1/jsonrpc-proxy/jsonrpc"
)

const ContextJSONRPC = "jsonrpc"

type JSONRPCContext struct {
	parent Context

	FromIP    string
	FromPath  string
	IsMono    bool
	Requests  []*jsonrpc.RpcRequest
	Responses []*jsonrpc.RpcResponse
}

func NewJSONRPCContext(parent Context, fromIP string, fromPath string, requests []*jsonrpc.RpcRequest) *JSONRPCContext {
	return &JSONRPCContext{parent: parent, FromIP: fromIP, FromPath: fromPath, Requests: requests}
}

func (j *JSONRPCContext) Parent() Context {
	return j.parent
}

func (J JSONRPCContext) Type() string {
	return ContextJSONRPC
}
