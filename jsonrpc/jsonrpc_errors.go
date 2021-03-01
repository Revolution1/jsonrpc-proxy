package jsonrpc

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/valyala/fasthttp"
	"strconv"
)

type RpcError struct {
	name    string
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Data    jsoniter.RawMessage `json:"data,omitempty"`
}

func (r *RpcError) Error() string {
	return fmt.Sprintf("RpcError(%d %s)", r.Code, r.Message)
}

func (r *RpcError) JsonError(id interface{}) string {
	s, _ := jsoniter.Marshal(r)
	i, _ := jsoniter.Marshal(id)
	return fmt.Sprintf(`{"id":%s,"jsonrpc":"2.0","error":%s}`, i, s)
}

func (r *RpcError) ErrorResponse(id interface{}) *RpcResponse {
	res := AcquireRpcResponse()
	res.Error = r
	res.Id = id
	return res
}

func (r *RpcError) WriteToRpcResponse(res *RpcResponse, id interface{}) {
	res.Jsonrpc = JSONRPC2
	res.Error = r
	res.Id = id
}

func (r *RpcError) Name() string {
	if r.name != "" {
		return r.name
	}
	return strconv.Itoa(r.Code)
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

func ErrWithData(rpcError *RpcError, data interface{}) *RpcError {
	e := new(RpcError)
	e.name = rpcError.name
	e.Code = rpcError.Code
	e.Message = rpcError.Message
	d, err := jsoniter.Marshal(data)
	if err != nil {
		log.WithError(err).Panic("fail to set rpc error data")
	}
	e.Data = d
	return e
}

// https://www.jsonrpc.org/historical/json-rpc-over-http.html#errors
func StatusCodeOfRpcError(e *RpcError) int {
	if e == nil {
		return 200
	}
	switch {
	case e.Code == ErrRpcParseError.Code:
		return fasthttp.StatusInternalServerError
	case e.Code == ErrRpcInvalidRequest.Code:
		return fasthttp.StatusBadRequest
	case e.Code == ErrRpcMethodNotFound.Code:
		return fasthttp.StatusNotFound
	case e.Code == ErrRpcInvalidParams.Code:
		return fasthttp.StatusInternalServerError
	case e.Code == ErrRpcInternalError.Code:
		return fasthttp.StatusInternalServerError
	case -32099 < e.Code && e.Code < -32000:
		return fasthttp.StatusInternalServerError
	default:
		return 200
	}
}
