package main

import (
	jsoniter "github.com/json-iterator/go"
	assertion "github.com/stretchr/testify/assert"
	"testing"
)

func TestParseRequest(t *testing.T) {
	assert := assertion.New(t)
	assert.True(RpcResponse{Error: nil, Result: 1}.Success())
	assert.False(RpcResponse{Error: &ErrRpcParseError, Result: 1}.Success())
	data := []byte(`[{"jsonrpc": "2.0", "method": "z", "id": 1},{}]`)
	reqs, err := ParseRequest(data)
	assert.NoError(err)
	assert.Equal(float64(1), reqs[0].Id)
	assert.Equal(nil, reqs[1].Id)
	assert.True(reqs[0].Validate())
	assert.False(reqs[1].Validate())
	m, err := jsoniter.MarshalToString(reqs)
	assert.NoError(err)
	t.Log(m)
	t.Log(reqs[0].ToCacheKey())
}
