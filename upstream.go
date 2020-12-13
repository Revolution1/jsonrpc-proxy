package main

import (
	"github.com/savsgio/gotils/nocopy"
	"github.com/valyala/fasthttp"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// BalancingClient is the interface for clients, which may be passed
// to UpstreamManager.Clients.
type BalancingClient interface {
	DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error
}

type HealthChecker func(req *fasthttp.Request, resp *fasthttp.Response, err error) bool

const (
	DefaultMaxAttempts       = 3
	DefaultReadTimeout       = 10 * time.Second
	DefaultConnTimeout       = 5 * time.Second
	DefaultMaxConnectionLife = 30 * time.Second
)

// UpstreamManager balances requests among available UpstreamManager.Clients.
//
// It has the following features:
//
//   - Balances load among available clients using 'least loaded' + 'least total'
//     hybrid technique.
//   - Dynamically decreases load on unhealthy clients.
//
// It is forbidden copying UpstreamManager instances. Create new instances instead.
//
// It is safe calling UpstreamManager methods from concurrently running goroutines.
type UpstreamManager struct {
	noCopy nocopy.NoCopy //nolint:unused,structcheck
	// HealthCheck is a callback called after each request.
	//
	// The request, response and the error returned by the client
	// is passed to HealthCheck, so the callback may determine whether
	// the client is healthy.
	//
	// Load on the current client is decreased if HealthCheck returns false.
	//
	// By default HealthCheck returns false if err != nil.
	HealthCheck HealthChecker

	// Timeout is the request timeout used when calling UpstreamManager.Do.
	//
	// DefaultLBClientTimeout is used by default.
	Timeout time.Duration

	// MaxAttempts is the max attempt count when trying get response from upstream
	MaxAttempts int
	maxAttempts int

	// KeepAlive represent whether we should open new connection for every request
	// false means yes
	KeepAlive bool

	upstreams []*upstream

	once sync.Once
}

func NewUpstreamManager(upstreams []string) *UpstreamManager {
	um := &UpstreamManager{MaxAttempts: DefaultMaxAttempts}
	if len(upstreams) == 0 {
		panic("upstreams of UpstreamManager cannot be empty")
	}
	for _, h := range upstreams {
		um.upstreams = append(um.upstreams, newUpstream(defaultHealthChecker, h))
	}
	um.setMaxAttempts()
	return um
}

//func (um *UpstreamManager) AddUpstream(upstream string) {
//
//}

func defaultHealthChecker(req *fasthttp.Request, resp *fasthttp.Response, err error) bool {
	return true
}

// DefaultLBClientTimeout is the default request timeout used by UpstreamManager
// when calling UpstreamManager.Do.
//
// The timeout may be overridden via UpstreamManager.Timeout.
const DefaultLBClientTimeout = time.Second

// DoDeadline calls DoDeadline on the least loaded client
func (um *UpstreamManager) DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error {
	return retry(req, resp, deadline, um.get().DoDeadline, um.maxAttempts)
}

// DoTimeout calculates deadline and calls DoDeadline on the least loaded client
func (um *UpstreamManager) DoTimeout(req *fasthttp.Request, resp *fasthttp.Response, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	return retry(req, resp, deadline, um.get().DoDeadline, um.maxAttempts)
}

func (um *UpstreamManager) setMaxAttempts() {
	if um.MaxAttempts > len(um.upstreams) {
		um.maxAttempts = len(um.upstreams)
	}
	um.maxAttempts = um.MaxAttempts
}

func retry(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time, f func(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error, max int) (err error) {
	if max <= 1 {
		return f(req, resp, deadline)
	}
	for a := 0; a < max; a++ {
		resp.Reset()
		err = f(req, resp, deadline)
		if err == nil && resp.StatusCode() == fasthttp.StatusOK && len(resp.Body()) == 0 {
			continue
		}
		if err == nil {
			return
		}
	}
	return
}

// Do calls calculates deadline using UpstreamManager.Timeout and calls DoDeadline
// on the least loaded client.
func (um *UpstreamManager) Do(req *fasthttp.Request, resp *fasthttp.Response) error {
	timeout := um.Timeout
	if timeout <= 0 {
		timeout = DefaultLBClientTimeout
	}
	return um.DoTimeout(req, resp, timeout)
}

func (um *UpstreamManager) get() *upstream {
	cs := um.upstreams
	minC := cs[0]
	minN := minC.PendingRequests()
	minT := atomic.LoadUint64(&minC.total)
	for _, c := range cs[1:] {
		n := c.PendingRequests()
		t := atomic.LoadUint64(&c.total)
		if n < minN || (n == minN && t < minT) {
			minC = c
			minN = n
			minT = t
		}
	}
	return minC
}

type upstream struct {
	c           BalancingClient
	healthCheck func(req *fasthttp.Request, resp *fasthttp.Response, err error) bool
	penalty     uint32

	keepalive bool

	scheme     string
	host       string
	requestURI string
	// total amount of requests handled.
	total uint64

	pendingRequests int32
}

func newUpstream(hc HealthChecker, host string) *upstream {
	u, err := url.Parse(host)
	if err != nil {
		panic(err)
	}
	requestURI := u.RequestURI()
	host = u.Host
	if (u.Scheme == "http" && u.Port() == "80") || (u.Scheme == "https" && u.Port() == "443") {
		host = u.Hostname()
	}
	//isTLS := u.Scheme == "https"
	//c := &fasthttp.Client{
	//	Name:                      host,
	//	MaxConnsPerHost:           0,
	//	MaxIdleConnDuration:       DefaultMaxConnectionLife,
	//	MaxIdemponentCallAttempts: DefaultMaxAttempts,
	//	ReadTimeout:               DefaultReadTimeout,
	//	WriteTimeout:              0,
	//	MaxConnWaitTimeout:        DefaultConnTimeout,
	//	RetryIf:                   nil,
	//}
	//c := &fasthttp.HostClient{
	//	Addr:               u.Host,
	//	Name:               host,
	//	IsTLS:              isTLS,
	//	TLSConfig:          nil,
	//	ReadTimeout:        DefaultReadTimeout,
	//	WriteTimeout:       0,
	//	MaxConnWaitTimeout: DefaultConnTimeout,
	//	RetryIf: func(req *fasthttp.Request) bool {
	//		return req.Header.IsGet() || req.Header.IsHead() || req.Header.IsPut()
	//	},
	//}
	return &upstream{
		c:           defaultFastStdHttpClient,
		healthCheck: hc,
		scheme:      u.Scheme,
		host:        host,
		requestURI:  requestURI,
	}
}

func (u *upstream) HostString() string {
	return u.scheme + "://" + u.host + u.requestURI
}

func (u *upstream) replaceReqHeaders(req *fasthttp.Request) {
	req.URI().SetScheme(u.scheme)
	req.URI().SetHost(u.host)
	req.Header.SetHost(u.scheme + "://" + u.host)
	req.Header.SetRequestURI(u.requestURI)
	if !u.keepalive {
		req.Header.SetConnectionClose()
	}
}

const maxRedirectsCount = 8

func (u *upstream) DoDeadline(_req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error {
	var err error
	r := fasthttp.AcquireRequest()
	_req.CopyTo(r)
	u.replaceReqHeaders(r)
	redirectsCount := 0
	var statusCode int
	var redirectURL string
	atomic.AddInt32(&u.pendingRequests, 1)
	for {
		err = u.c.DoDeadline(r, resp, deadline)
		if !u.isHealthy(r, resp, err) && u.incPenalty() {
			// Penalize the client returning error, so the next requests
			// are routed to another clients.
			time.AfterFunc(penaltyDuration, u.decPenalty)
			break
		} else {
			atomic.AddUint64(&u.total, 1)
		}
		statusCode = resp.Header.StatusCode()
		if !fasthttp.StatusCodeIsRedirect(statusCode) {
			break
		}
		redirectsCount++
		if redirectsCount > maxRedirectsCount {
			err = fasthttp.ErrTooManyRedirects
			break
		}
		location := resp.Header.Peek(fasthttp.HeaderLocation)
		if len(location) == 0 {
			err = fasthttp.ErrMissingLocation
			break
		}
		redirectURL = getRedirectURL(r.URI(), location)
		r.SetRequestURI(redirectURL)
	}
	atomic.AddInt32(&u.pendingRequests, -1)
	fasthttp.ReleaseRequest(r)
	return err
}

func (u *upstream) PendingRequests() int {
	n := atomic.LoadInt32(&u.pendingRequests)
	m := atomic.LoadUint32(&u.penalty)
	return int(n) + int(m)
}

func (u *upstream) isHealthy(req *fasthttp.Request, resp *fasthttp.Response, err error) bool {
	if u.healthCheck == nil {
		return err == nil
	}
	return u.healthCheck(req, resp, err)
}

func (u *upstream) incPenalty() bool {
	m := atomic.AddUint32(&u.penalty, 1)
	if m > maxPenalty {
		u.decPenalty()
		return false
	}
	return true
}

func (u *upstream) decPenalty() {
	atomic.AddUint32(&u.penalty, ^uint32(0))
}

const (
	maxPenalty      = 300
	penaltyDuration = 3 * time.Second
)

func getRedirectURL(uri *fasthttp.URI, location []byte) string {
	u := fasthttp.AcquireURI()
	uri.CopyTo(u)
	u.UpdateBytes(location)
	redirectURL := u.String()
	fasthttp.ReleaseURI(u)
	return redirectURL
}
