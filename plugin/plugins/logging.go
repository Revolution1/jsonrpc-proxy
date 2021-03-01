package plugins

import (
	"fmt"
	realip "github.com/Ferluci/fast-realip"
	"github.com/pkg/errors"
	"github.com/revolution1/jsonrpc-proxy/plugin"
	"github.com/revolution1/jsonrpc-proxy/proxyctx"
	"github.com/revolution1/jsonrpc-proxy/utils"
	"github.com/sirupsen/logrus"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	FormatText = "text"
	FormatJson = "json"
)

type LoggerConfig struct {
	Disabled bool   `json:"disabled"`
	Format   string `json:"format"`
	Verbose  int    `json:"verbose"`
	Stream   string `json:"stream"`
	//MaxSize string `json:"max_size"`
}

type LoggingConfig struct {
	AccessLog LoggerConfig
	ErrorLog  LoggerConfig
}

type LoggingPlugin struct {
	info *plugin.Info

	config *LoggingConfig

	accessLog     *logrus.Logger
	accessLogfile *os.File

	errorLog     *logrus.Logger
	errorLogfile *os.File

	mu sync.Mutex
}

func (l *LoggingPlugin) ID() string {
	return "logging"
}

func (l *LoggingPlugin) Info() *plugin.Info {
	if l.info == nil {
		l.info = &plugin.Info{
			ID:          l.ID(),
			Version:     "1",
			Description: "Request logging util",
			AcceptContexts: []string{
				//"tcp",
				"http",
				"jsonrpc",
				//"websocket",
			},
			ProvideContext: "",
		}
	}
	return l.info
}

func (l *LoggingPlugin) New(config *plugin.Config) (plugin.Plugin, error) {
	lc := &LoggingConfig{}
	if err := utils.CastToStruct(config.Spec, lc); err != nil {
		return nil, err
	}
	p := &LoggingPlugin{
		info:   l.Info(),
		config: lc,
	}
	l.accessLog = logrus.New()
	l.errorLog = logrus.New()
	l.preSetupLogger(l.accessLog, &lc.AccessLog)
	l.preSetupLogger(l.errorLog, &lc.ErrorLog)

	switch lc.AccessLog.Stream {
	case "stdout":
		l.accessLog.SetOutput(os.Stdout)
	case "stderr":
		l.accessLog.SetOutput(os.Stderr)
	default:
		file, err := os.OpenFile(lc.AccessLog.Stream, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
		if err != nil {
			panic(err)
		}
		p.accessLogfile = file
		l.accessLog.SetOutput(file)
	}
	switch lc.ErrorLog.Stream {
	case "stdout":
		l.errorLog.SetOutput(os.Stdout)
	case "stderr":
		l.errorLog.SetOutput(os.Stderr)
	default:
		if lc.ErrorLog.Stream == lc.AccessLog.Stream {
			l.errorLog.SetOutput(l.accessLogfile)
		} else {
			file, err := os.OpenFile(lc.ErrorLog.Stream, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
			if err != nil {
				panic(err)
			}
			p.errorLogfile = file
			l.errorLog.SetOutput(file)
		}
	}
	return p, nil
}

func (l *LoggingPlugin) Handler(handleFunc plugin.HandleFunc) plugin.HandleFunc {
	return func(ctx proxyctx.Context) error {
		start := time.Now()
		err := handleFunc(ctx)
		if err != nil {
			return err
		}
		switch ctx.Type() {
		case proxyctx.ContextHTTP:
			return l.HandleHTTP(ctx.(*proxyctx.HTTPContext), start)
		case proxyctx.ContextJSONRPC:
			return l.HandleJSONRPC(ctx.(*proxyctx.JSONRPCContext), start)
		}
		return nil
	}
}

func (l *LoggingPlugin) HandleHTTP(ctx *proxyctx.HTTPContext, start time.Time) error {
	duration := time.Now().Sub(start)
	status := ctx.Ctx.Response.StatusCode()
	method := string(ctx.Ctx.Method())
	scheme := strings.ToUpper(string(ctx.Ctx.URI().Scheme()))
	uri := string(ctx.Ctx.RequestURI())
	//path := string(ctx.Ctx.URI().Path())
	ua := string(ctx.Ctx.UserAgent())
	ip := realip.FromRequest(ctx.Ctx)
	var reqSize, resSize int
	if ctx.Ctx.Request.IsBodyStream() {
		reqSize = len(ctx.Ctx.Request.Header.RawHeaders()) + ctx.Ctx.Request.Header.ContentLength() + 4
	} else {
		reqSize = len(ctx.Ctx.Request.Header.RawHeaders()) + len(ctx.Ctx.Request.Body()) + 4
	}
	if ctx.Ctx.Response.IsBodyStream() {
		resSize = ctx.Ctx.Response.Header.Len() + ctx.Ctx.Response.Header.ContentLength() + 4
	} else {
		resSize = ctx.Ctx.Response.Header.Len() + len(ctx.Ctx.Response.Body()) + 4
	}
	var printf = l.accessLog.Infof
	if guessIsHealthChecker(ua) {
		printf = l.accessLog.Tracef
	}
	if status < 200 && status >= 400 {
		printf = l.errorLog.Errorf
	}
	printf(
		`%s - %s - "%s %s" %d %d %d "%s" %s`+"\n",
		scheme, ip, method, uri, status, reqSize, resSize, ua, duration,
	)
	return nil
}

func (l *LoggingPlugin) HandleJSONRPC(ctx *proxyctx.JSONRPCContext, start time.Time) error {
	var extra []string
	methods := make([]string, len(ctx.Requests))
	status := make([]string, len(ctx.Responses))
	duration := time.Now().Sub(start)
	if ctx.Parent().Type() == proxyctx.ContextHTTP {
		httpReq := ctx.Parent().(*proxyctx.HTTPContext).Ctx
		extra = append(extra, fmt.Sprintf("(%s)", string(httpReq.UserAgent())))
	}
	for _, m := range ctx.Requests {
		methods = append(methods, m.Method)
	}
	for _, r := range ctx.Responses {
		if r.Success() {
			status = append(status, "OK")
		} else {
			status = append(status, r.Error.Name())
		}
	}
	l.accessLog.Infof(
		`JSONRPC - %s - %s "%s(%s)" %s`,
		ctx.FromIP, ctx.FromPath, strings.Join(methods, ","), strings.Join(status, ","), duration,
	)
	return nil
}

func (l *LoggingPlugin) Destroy() {
	if l.accessLogfile != nil {
		l.accessLogfile.Close()
		l.accessLogfile = nil
	}
	if l.errorLogfile != nil {
		l.errorLogfile.Close()
		l.errorLogfile = nil
	}
}

func (*LoggingPlugin) preSetupLogger(l *logrus.Logger, c *LoggerConfig) {
	if c.Disabled {
		l.SetLevel(logrus.FatalLevel)
		return
	}
	switch c.Format {
	case FormatJson:
		l.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat:   time.RFC3339,
			DisableTimestamp:  false,
			DisableHTMLEscape: true,
		})
	case FormatText:
		l.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: time.RFC3339,
		})
	default:
		panic(errors.New("unknown logging format, should be json|text"))
	}
}

var WellKnownHealthCheckerUserAgentPrefixes = []string{
	"ELB-HealthChecker",
	"kube-probe",
	"Prometheus",
}

func guessIsHealthChecker(ua string) bool {
	for _, p := range WellKnownHealthCheckerUserAgentPrefixes {
		if strings.HasPrefix(ua, p) {
			return true
		}
	}
	return false
}

func init() {
	plugin.RegisterPlugin(&LoggingPlugin{})
}
