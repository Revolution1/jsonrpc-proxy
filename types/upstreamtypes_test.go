package types

import (
	"encoding/json"
	assertion "github.com/stretchr/testify/assert"
	"testing"
)

func TestUpstreamType(t *testing.T) {
	assert := assertion.New(t)
	u := UpstreamType(0)
	assert.NoError(json.Unmarshal([]byte(`"http"`), &u))
	assert.Equal(HTTPUpstream, u)

	assert.Error(json.Unmarshal([]byte(`1`), &u))
	assert.Error(json.Unmarshal([]byte(`"xxx"`), &u))

	u = TCPUpstream
	d, err := json.Marshal(u)
	assert.NoError(err)
	assert.Equal(`"TCP"`, string(d))
}

func TestEndpoint(t *testing.T) {
	assert := assertion.New(t)
	data := `"https://admin@api.zilliqa.com"`
	e := new(Endpoint)
	assert.NoError(json.Unmarshal([]byte(data), e))
	assert.Equal("https", e.Scheme)
	assert.Equal("api.zilliqa.com", e.Host)
	assert.Equal("admin", e.User.Username())
	m, err := json.Marshal(e)
	assert.NoError(err)
	assert.Equal(data, string(m))
	assert.NoError(e.Parse("http://dev/"))
	assert.Equal("http", e.Scheme)
	assert.Equal("dev", e.Host)
	assert.Equal("/", e.Path)
}
