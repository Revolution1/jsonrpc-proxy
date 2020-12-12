package main

import (
	"context"
	"github.com/fasthttp/router"
	"github.com/google/gops/agent"
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

var debugMode bool

func main() {
	rootCmd := cobra.Command{
		Use:   "jsonrpc-proxy",
		Short: "proxy for jsonrpc service",
	}
	flags := rootCmd.Flags()
	path := flags.StringP("config", "c", "proxy.yaml", "the path of config file")
	_ = rootCmd.MarkFlagFilename("config", "yaml", "yml")
	//_ = cobra.MarkFlagRequired(flags, "config")
	//ctx, cancel := context.WithCancel(context.Background())
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := agent.Listen(agent.Options{}); err != nil {
			return err
		}
		config, err := LoadConfig(*path)
		if err != nil {
			return err
		}
		config.MustValidate()
		initLog(config)
		return runMain(config)
	}
	_ = rootCmd.Execute()
}

func runMain(config *Config) error {
	CheckFdLimit()
	log.Infof("Build: %s %s %s, PID: %d", runtime.GOOS, runtime.Compiler, runtime.Version(), os.Getpid())
	r := router.New()
	//if debugMode {
	//	log.Infof("Debug Mode enabled")
	//	r.GET("/debug/pprof/{name:*}", pprofhandler.PprofHandler)
	//}
	p := NewProxy(config)
	p.RegisterHandler(r)

	serverListen := GetHostFromUrl(config.Listen)
	manageListen := GetHostFromUrl(config.Manage.Listen)

	var manageServer *fasthttp.Server
	m := NewManage(config, p)
	if serverListen == manageListen {
		log.Warn("Manage Server listens at the same address with RPC Server")
		m.registerHandler(r)
	} else {
		r := router.New()
		m.registerHandler(r)
		h := useMiddleWares(r.Handler, panicHandler, Cors, fasthttp.CompressHandler, accessLogMetricHandler("[Manage] ", config))
		manageServer = newServer("JSON-RPC Proxy Manage Server", h, log.TraceLevel, config)
	}
	h := useMiddleWares(r.Handler, panicHandler, Cors, fasthttp.CompressHandler, accessLogMetricHandler("", config))
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

func initLog(config *Config) {
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:    true,
		TimestampFormat:  time.RFC3339,
		QuoteEmptyFields: true,
	})
	level, err := log.ParseLevel(strings.ToLower(config.LogLevel))
	if err != nil {
		log.Fatal("Invalid logLevel")
	}
	debugMode, err = strconv.ParseBool(os.Getenv("DEBUG"))
	if err != nil {
		debugMode = config.Debug
	}
	if debugMode && level < log.DebugLevel {
		level = log.DebugLevel
	}
	log.SetLevel(level)
	log.Debugf("LogLevel: %s", log.GetLevel())
}

func newServer(name string, h fasthttp.RequestHandler, level log.Level, config *Config) *fasthttp.Server {
	return &fasthttp.Server{
		Name:              name,
		Handler:           h,
		ErrorHandler:      nil,
		HeaderReceived:    nil,
		ContinueHandler:   nil,
		TCPKeepalive:      true,
		ReadTimeout:       config.ReadTimeout.Duration,
		WriteTimeout:      config.WriteTimeout.Duration,
		IdleTimeout:       config.IdleTimeout.Duration,
		Concurrency:       0,
		DisableKeepalive:  false,
		ReduceMemoryUsage: false,
		LogAllErrors:      false,
		Logger:            LeveledLogger{level: level},
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
