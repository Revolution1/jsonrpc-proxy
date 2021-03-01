package main

import (
	"fmt"
	"github.com/valyala/fasthttp"
	"golang.org/x/sync/errgroup"
	"net"
)

func main() {
	listener1, _ := net.Listen("tcp4", "")
	listener2, _ := net.Listen("tcp4", "")
	server := fasthttp.Server{Name: "test", Handler: func(ctx *fasthttp.RequestCtx) {
		fmt.Printf("Got request of Host %s, URI: %s\n", ctx.Host(), ctx.URI())
		ctx.SuccessString("text/html", "ok")
	}}
	defer server.Shutdown()
	eg := errgroup.Group{}
	eg.Go(func() error {
		return server.Serve(listener1)
	})
	eg.Go(func() error {
		return server.Serve(listener2)
	})
	fmt.Printf("listener1: http://%s\n", listener1.Addr().String())
	fmt.Printf("listener2: http://%s\n", listener2.Addr().String())
	eg.Wait()
	fmt.Println("exit")
}
