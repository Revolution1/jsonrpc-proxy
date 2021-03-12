package statistic

import (
	realip "github.com/Ferluci/fast-realip"
	jsoniter "github.com/json-iterator/go"
	"github.com/revolution1/jsonrpc-proxy/jsonrpc"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"reflect"
	"sync"
	"time"
)

const collectInterval = 10 * time.Second

var pool = sync.Pool{New: func() interface{} { return &Stat{} }}

type Stat struct {
	IP        string
	UserAgent string
	Method    string
	Params    string
	Start     time.Time
	End       time.Time
}

func newStat(ctx *fasthttp.RequestCtx, req *jsonrpc.RpcRequest, start, end time.Time) *Stat {
	s := pool.Get().(*Stat)
	s.IP = realip.FromRequest(ctx)
	s.UserAgent = string(ctx.UserAgent())
	s.Method = req.Method
	params := reflect.ValueOf(req.Params)
	if (params.Kind() == reflect.Array || params.Kind() == reflect.Slice) && params.Len() <= 2 {
		data, _ := jsoniter.Marshal(req.Params)
		s.Params = string(data)
	}
	s.Start = start
	s.End = end
	return s
}

type Collector struct {
	ch chan *Stat
	mu sync.Mutex

	statCount map[string]struct {
		IP        string
		UserAgent string
		Method    string
		Params    string
		count     int64
	}
	ipCount           map[string]int64
	methodCount       map[string]int64
	uaCount           map[string]int64
	methodParamsCount map[string]struct {
		Method string
		Params string
		count  int64
	}

	recording chan struct{}
	stop      chan struct{}
	close     chan struct{}
}

func NewCollector() *Collector {
	return &Collector{ch: make(chan *Stat, 1024*8), close: make(chan struct{})}
}

func (c *Collector) Run() {

}

func (c *Collector) runTicker() {
	ticker := time.NewTicker(collectInterval)
Loop:
	for {
		c.mu.Lock()
		select {
		case <-c.stop:
			close(c.close)
			break Loop
		case stat := <-c.ch:
			c.ProcessStat(stat)
		case <-c.recording:
		}
		c.mu.Unlock()
	}
	ticker.Stop()
}

func (c *Collector) runConsumer() {
Loop:
	for {
		c.mu.Lock()
		select {
		case <-c.stop:
			close(c.close)
			break Loop
		case stat := <-c.ch:
			c.ProcessStat(stat)
		case <-c.recording:
		}
		c.mu.Unlock()
	}
}

func (c *Collector) PushStat(ctx *fasthttp.RequestCtx, req *jsonrpc.RpcRequest, start, end time.Time) {
	stat := newStat(ctx, req, start, end)
	select {
	case c.ch <- stat:
	default:
		log.Debugf("dropped stat of ip %s method %s", stat.IP, stat.Method)
		pool.Put(stat)
	}
}

func (c *Collector) ProcessStat(stat *Stat) {
}

func (c *Collector) WriteStats() {
	c.recording <- struct{}{}
	c.mu.Lock()
	defer c.mu.Unlock()

}

func (c *Collector) Stop() {
	close(c.stop)
	<-c.close
}
