package main

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/revolution1/jsonrpc-proxy/metrics"
)

var version = "0.0.3"

var (
	commit    = ""
	branch    = ""
	tag       = ""
	buildInfo = ""
	date      = ""
)

func printVersion() string {
	return fmt.Sprintf("%s (commit='%s', branch='%s', tag='%s', date='%s', build='%s')", version, commit, branch, tag, date, buildInfo)
}

func init() {
	versionGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metrics.Namespace,
		Name:      "version",
		Help:      "version info of jsonrpc proxy",
		ConstLabels: prometheus.Labels{
			"version":   version,
			"branch":    branch,
			"tag":       tag,
			"buildinfo": buildInfo,
			"date":      date,
		},
	})
	prometheus.MustRegister(versionGauge)
	versionGauge.Set(1)
}
