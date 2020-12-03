package main

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
)

type RpcError struct {
	name    string              `json:"_"`
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Data    jsoniter.RawMessage `json:"data,omitempty"`
}

func (r *RpcError) Error() string {
	return fmt.Sprintf("RpcError(%d %s)", r.Code, r.Message)
}

func (r *RpcError) JsonError() string {
	return fmt.Sprintf(`{"error":{"code":%d,"message":"%s"},"id":null,"jsonrpc":"2.0"}`, r.Code, r.Message)
}

func (r *RpcError) AccessLogError() string {
	return fmt.Sprintf("%s(%d)", r.name, r.Code)
}

func (r *RpcError) Is(err error) bool {
	e, ok := err.(*RpcError)
	if !ok {
		return false
	}
	return r.Code == e.Code
}

var (
	ErrRpcParseError     = &RpcError{name: "ParseError", Code: -32700, Message: "Parse error"}
	ErrRpcInvalidRequest = &RpcError{name: "InvalidRequest", Code: -32600, Message: "Invalid Request"}
	ErrRpcMethodNotFound = &RpcError{name: "MethodNotFound", Code: -32601, Message: "Method not found"}
	ErrRpcInvalidParams  = &RpcError{name: "InvalidParams", Code: -32602, Message: "Invalid params"}
	ErrRpcInternalError  = &RpcError{name: "InternalError", Code: -32603, Message: "Internal error"}
	ErrProcedureIsMethod = &RpcError{name: "ProcedureIsMethod", Code: -32604, Message: "Procedure is method"}
)
