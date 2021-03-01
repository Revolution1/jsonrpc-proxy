package middleware

import (
	"github.com/valyala/fasthttp"
	"strings"
)

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

type MiddleWare func(h fasthttp.RequestHandler) fasthttp.RequestHandler

func UseMiddleWares(handler fasthttp.RequestHandler, middleware ...MiddleWare) fasthttp.RequestHandler {
	for _, m := range middleware {
		handler = m(handler)
	}
	return handler
}
