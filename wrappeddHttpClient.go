package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"github.com/certifi/gocertifi"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

var defaultFastStdHttpClient = &FastStdHttpClient{}

type FastStdHttpClient struct {
	*http.Client
	once sync.Once
}

var ErrUpstreamTimeout = errors.New("upstream request timeout")

func (f *FastStdHttpClient) DoDeadline(fastReq *fasthttp.Request, fastResp *fasthttp.Response, deadline time.Time) error {
	f.init()
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, string(fastReq.Header.Method()), fastReq.URI().String(), bytes.NewReader(fastReq.Body()))
	if err != nil {
		return err
	}
	fastReq.Header.VisitAll(func(key, value []byte) {
		req.Header.Set(string(key), string(value))
	})
	if log.IsLevelEnabled(log.TraceLevel) {
		log.Tracef("requesting to upstream: %s\n%s\n", req.RequestURI, fastReq.String())
	}
	resp, err := f.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ErrUpstreamTimeout
		}
		return err
	}
	fastResp.SetStatusCode(resp.StatusCode)
	for k, _ := range resp.Header {
		fastResp.Header.Set(k, resp.Header.Get(k))
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	encodings := resp.Header.Values(fasthttp.HeaderContentEncoding)
	var d interface{}
	err = json.Unmarshal(body, &d)
	if err != nil {
		log.Debug(string(body))
	}
	if len(encodings) == 0 && !isASCII(body) {
		log.Info(encodings)
	}
	fastResp.SetBodyRaw(body)
	return nil
}

//init initialize client settings
func (f *FastStdHttpClient) init() {
	f.once.Do(func() {
		// init client
		certPool, err := gocertifi.CACerts()
		if err != nil {
			panic(err)
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{RootCAs: certPool}
		f.Client = &http.Client{Transport: transport}

		// disable redirect following for net/http.Client cause it cannot follow POST request
		if f.CheckRedirect == nil {
			f.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			}
		}
	})
}
