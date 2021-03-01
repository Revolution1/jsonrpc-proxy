package main

import (
	"context"
	"fmt"
	"github.com/fasthttp/router"
	"github.com/google/gops/agent"
	"github.com/revolution1/jsonrpc-proxy/manage"
	"github.com/revolution1/jsonrpc-proxy/middleware"
	"github.com/revolution1/jsonrpc-proxy/oldconfig"
	"github.com/revolution1/jsonrpc-proxy/types"
	"github.com/revolution1/jsonrpc-proxy/proxy"
	"github.com/revolution1/jsonrpc-proxy/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/valyala/fasthttp"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	rootCmd := cobra.Command{
		Use:   "jsonrpc-proxy",
		Short: "proxy for jsonrpc service",
	}
	flags := rootCmd.Flags()
	printVer := flags.BoolP("version", "v", false, "print version")
	path := flags.StringP("config", "c", "proxy.yaml", "the path of config file")
	_ = rootCmd.MarkFlagFilename("config", "yaml", "yml")
	//_ = cobra.MarkFlagRequired(flags, "config")
	//ctx, cancel := context.WithCancel(context.Background())
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if *printVer {
			fmt.Println(printVersion())
			return nil
		}
		// gops agent
		if err := agent.Listen(agent.Options{}); err != nil {
			return err
		}
		log.Infof("Loading config from %s", *path)
		log.Infof("Version: %s", printVersion())
		conf, err := types.
		if err != nil {
			return err
		}
		//conf.MustValidate()
		initLog(conf)
		return runMain(conf)
	}
	_ = rootCmd.Execute()
}

func runMain(config *oldconfig.Config) error {
	utils.CheckFdLimit()
	log.Infof("Build: %s %s %s, PID: %d", runtime.GOOS, runtime.Compiler, runtime.Version(), os.Getpid())
	r := router.New()
	//if debugMode {
	//	log.Infof("Debug Mode enabled")
	//	r.GET("/debug/pprof/{name:*}", pprofhandler.PprofHandler)
	//}
	p := proxy.NewProxy(config)
	p.RegisterHandler(r)

	serverListen := utils.GetHostFromUrl(config.Listen)
	manageListen := utils.GetHostFromUrl(config.Manage.Listen)

	var manageServer *fasthttp.Server
	m := manage.NewManage(config, p)
	if serverListen == manageListen {
		log.Warn("Manage Server listens at the same address with RPC Server")
		m.RegisterHandler(r)
	} else {
		r := router.New()
		m.RegisterHandler(r)
		h := middleware.UseMiddleWares(
			r.Handler,
			middleware.PanicHandler,
			middleware.Cors,
			fasthttp.CompressHandler,
			middleware.AccessLogMetricHandler("[Manage] ", config),
		)
		manageServer = newServer("JSON-RPC Proxy Manage Server", h, log.TraceLevel, config)
	}
	h := middleware.UseMiddleWares(
		r.Handler,
		middleware.PanicHandler,
		middleware.Cors,
		fasthttp.CompressHandler,
		middleware.AccessLogMetricHandler("", config),
	)
	server := newServer("JSON-RPC Proxy Server", h, log.TraceLevel, config)

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go runServer(ctx, server, serverListen, wg)

	if manageServer != nil {
		wg.Add(1)
		go runServer(ctx, manageServer, manageListen, wg)
	}

	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, os.Interrupt, os.Kill, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Infof("received signal '%s', shutting down server...", strings.ToUpper(sig.String()))
		cancel()
	}()
	wg.Wait()
	return nil
}

func initLog(conf *oldconfig.Config) {
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:    true,
		TimestampFormat:  time.RFC3339,
		QuoteEmptyFields: true,
		ForceColors:      conf.LogForceColors,
	})
	level, err := log.ParseLevel(strings.ToLower(conf.LogLevel))
	if err != nil {
		log.Fatal("Invalid logLevel")
	}
	oldconfig.DebugMode, err = strconv.ParseBool(os.Getenv("DEBUG"))
	if err != nil {
		oldconfig.DebugMode = conf.Debug
	}
	if oldconfig.DebugMode && level < log.DebugLevel {
		level = log.DebugLevel
	}
	log.SetLevel(level)
	log.Debugf("LogLevel: %s", log.GetLevel())
}

func newServer(name string, h fasthttp.RequestHandler, level log.Level, conf *oldconfig.Config) *fasthttp.Server {
	return &fasthttp.Server{
		Name:              name,
		Handler:           h,
		ErrorHandler:      nil,
		HeaderReceived:    nil,
		ContinueHandler:   nil,
		TCPKeepalive:      true,
		ReadTimeout:       conf.ReadTimeout.Duration,
		WriteTimeout:      conf.WriteTimeout.Duration,
		IdleTimeout:       conf.IdleTimeout.Duration,
		Concurrency:       0,
		DisableKeepalive:  false,
		ReduceMemoryUsage: false,
		LogAllErrors:      false,
		Logger:            middleware.LeveledLogger{Level: level},
	}
}

func runServer(ctx context.Context, server *fasthttp.Server, listen string, wg *sync.WaitGroup) {
	defer wg.Done()
	if listen == "" {
		log.Errorf("empty listen address for %s", server.Name)
	}

	errCh := make(chan error)
	go func() {
		defer close(errCh)
		log.Infof("%s listening at %s", server.Name, listen)
		if err := server.ListenAndServe(listen); err != nil {
			errCh <- err
		}
	}()
	select {
	case err := <-errCh:
		log.Infof("%s exited with error %s...", server.Name, err.Error())
	case <-ctx.Done():
		if err := server.Shutdown(); err != nil {
			log.WithError(err).WithField("name", server.Name).Error("error while shutting down server")
		}
		log.Infof("shutting down %s...", server.Name)
	}
}
