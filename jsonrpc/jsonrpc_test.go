package jsonrpc

import (
	jsoniter "github.com/json-iterator/go"
	assertion "github.com/stretchr/testify/assert"
	"testing"
)

func TestParseRequest(t *testing.T) {
	assert := assertion.New(t)
	assert.True(RpcResponse{Error: nil, Result: 1}.Success())
	assert.False(RpcResponse{Error: ErrRpcParseError, Result: 1}.Success())
	data := []byte(`[{"jsonrpc": "2.0", "method": "z", "id": 1},{}]`)
	reqs, err := ParseRequest(data)
	assert.Nil(err)
	assert.Equal(float64(1), reqs[0].Id)
	assert.Equal(nil, reqs[1].Id)
	assert.True(reqs[0].Validate())
	assert.False(reqs[1].Validate())
	m, e := jsoniter.MarshalToString(reqs)
	assert.NoError(e)
	t.Log("MarshalToString:", m)
	k, e := reqs[0].ToCacheKey()
	assert.NoError(e)
	t.Log("cache key:", k)
	empty := []byte(`[{}]`)
	reqs, err = ParseRequest(empty)
	assert.Nil(err)
	assert.Nil(reqs)
	assert.False(reqs[0].Validate())

	empty2 := []byte(`[]`)
	reqs, err = ParseRequest(empty2)
	assert.Nil(err)
	assert.Nil(reqs)
	assert.False(reqs[0].Validate())
}

const req0 = `{"jsonrpc": "2.0", "id": 1, "method": "m", "params": { "foo": 1.23e1, "bar": { "baz": true, "abc": 12 }}}`
const req1 = `{"jsonrpc": "2.0", "id": 1, "method": "m", "params": {"bar":{"abc":12,"baz":true},"foo":12.3}}`

func TestReqToKey(t *testing.T) {
	assert := assertion.New(t)
	req, err := ParseRequest([]byte(req0))
	assert.Nil(err)
	reqSorted, err := ParseRequest([]byte(req1))
	assert.Nil(err)
	k0, e := req[0].ToCacheKey()
	assert.NoError(e)
	k1, e := reqSorted[0].ToCacheKey()
	assert.NoError(e)
	t.Log(k0, k1)
	assert.Equal(k1, k0)
}
