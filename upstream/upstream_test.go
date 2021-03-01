package upstream

import (
	assertion "github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"net"
	"net/url"
	"testing"
)

func TestUpstreamManager(t *testing.T) {
	assert := assertion.New(t)
	uri := fasthttp.AcquireURI()
	e := uri.Parse([]byte(":8080/aaa"), nil)
	assert.NoError(e)
	t.Log("fasthttp.URI", string(uri.Scheme()), string(uri.Host()), string(uri.Path()), uri.String())
	t.Log(net.ParseIP("111111"), net.ParseIP("11.11.11.11"))
	u, _ :=url.Parse("http://aaa.com/dasd?a=b#/dsd")
	t.Log("net/url", u.Scheme, u.Host, u.Port(), u.RequestURI())
}
