// ref: https://www.jsonrpc.org/specification
package main

import (
	"bytes"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"go/types"
)

//var RawVer = []byte(`"2.0"`)
const JSONRPC2 = "2.0"

var jsonSorted = jsoniter.Config{SortMapKeys: true, EscapeHTML: true}.Froze()

//type JsonRpcVersion string
//
//func (j JsonRpcVersion) MarshalJSON() ([]byte, error) {
//	if j != "2.0" {
//		return nil, ErrRpcInvalidRequest
//	}
//	return RawVer, nil
//}

type rpcHeader struct {
	Jsonrpc string      `json:"jsonrpc"`
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

func (r RpcRequest) ToCacheKey() (string, error) {
	params, err := jsonSorted.MarshalToString(r.Params)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s(%s)", r.Method, params), nil
}

type RpcResponse struct {
	rpcHeader
	Error  *RpcError   `json:"error,omitempty"`
	Result interface{} `json:"result,omitempty"`
}

func (r RpcResponse) Success() bool {
	return r.Result != nil && r.Error == nil
}

func ParseRequest(data []byte) ([]RpcRequest, *RpcError) {
	data = bytes.TrimSpace(data)
	if len(data) < 1 {
		return nil, ErrRpcParseError
	}
	if data[0] == '{' { // not batch
		req := RpcRequest{}
		e := jsoniter.Unmarshal(data, &req)
		if e != nil {
			return nil, ErrRpcParseError
		}
		return []RpcRequest{req}, nil
	} else {
		var reqs []RpcRequest
		e := jsoniter.Unmarshal(data, &reqs)
		if e != nil {
			return nil, ErrRpcParseError
		}
		return reqs, nil
	}
}
