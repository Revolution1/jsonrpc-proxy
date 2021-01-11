// ref: https://www.jsonrpc.org/specification
package main

import (
	"bytes"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"go/types"
	"sync"
)

var (
	rpcReqPool = sync.Pool{}
	rpcResPool = sync.Pool{}
)

//var RawVer = []byte(`"2.0"`)
const JSONRPC2 = "2.0"

var jsonSorted = jsoniter.Config{SortMapKeys: true, EscapeHTML: true}.Froze()

type rpcHeader struct {
	Jsonrpc string      `json:"jsonrpc,intern"`
	Id      interface{} `json:"id,omitempty"`
}

func (h rpcHeader) Validate() bool {
	switch h.Id.(type) {
	case string, float64, types.Nil:
		return h.Jsonrpc == JSONRPC2
	}
	return false
}

type RpcRequest struct {
	rpcHeader
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

func NewRpcRequest(id int, method string, params interface{}) *RpcRequest {
	return &RpcRequest{rpcHeader: rpcHeader{JSONRPC2, id}, Method: method, Params: params}
}

func (r RpcRequest) Validate() bool {
	return r.rpcHeader.Validate() && r.Method != ""
}

func (r RpcRequest) String() string {
	return fmt.Sprintf("%s(%s)", r.Method, r.Params)
}

func (r RpcRequest) ToCacheKey() (string, error) {
	params, err := jsonSorted.MarshalToString(r.Params)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s(%s)", r.Method, params), nil
}

func (r *RpcRequest) Reset() {
	r.rpcHeader.Id = nil
	r.Method = ""
	r.Params = nil
}

type RpcResponse struct {
	rpcHeader
	Error  *RpcError   `json:"error,omitempty"`
	Result interface{} `json:"result,omitempty"`
}

func (r RpcResponse) Success() bool {
	return r.Result != nil && r.Error == nil
}

func (r *RpcResponse) Reset() {
	r.rpcHeader.Id = nil
	r.Error = nil
	r.Result = nil
}

func ParseRequest(data []byte) (RpcRequests, *RpcError) {
	data = bytes.TrimSpace(data)
	if len(data) < 1 {
		return nil, ErrRpcParseError
	}
	if data[0] == '{' { // not batch
		req := AcquireRpcRequest()
		e := jsoniter.Unmarshal(data, &req)
		if e != nil {
			return nil, ErrRpcParseError
		}
		return RpcRequests{req}, nil
	} else {
		var reqs RpcRequests
		e := jsoniter.Unmarshal(data, &reqs)
		if e != nil {
			return nil, ErrRpcParseError
		}
		return reqs, nil
	}
}

type RpcRequests []*RpcRequest

//
//func (rs RpcRequests) Validate() bool {
//	for _,r:=range rs {
//	  return rs.rpcHeader.Validate() && r.Method != ""
//	}
//}
//
//func (rs RpcRequests) String() string {
//	return fmt.Sprintf("%s(%s)", r.Method, r.Params)
//}
//
//func (rs RpcRequests) ToCacheKey() string {
//
//}

func AcquireRpcRequest() *RpcRequest {
	r := rpcReqPool.Get()
	if r == nil {
		return &RpcRequest{rpcHeader: rpcHeader{Jsonrpc: JSONRPC2}}
	}
	return r.(*RpcRequest)
}

func ReleaseRpcRequest(r *RpcRequest) {
	r.Reset()
	rpcReqPool.Put(r)
}

func AcquireRpcResponse() *RpcResponse {
	r := rpcReqPool.Get()
	if r == nil {
		return &RpcResponse{rpcHeader: rpcHeader{Jsonrpc: JSONRPC2}}
	}
	return r.(*RpcResponse)
}

func ReleaseRpcResponse(r *RpcResponse) {
	r.Reset()
	rpcReqPool.Put(r)
}
