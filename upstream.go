package main

import "net/url"

type Upstream struct {
	url.URL
}

type UpstreamManager struct {
	Upstreams []Upstream
}

