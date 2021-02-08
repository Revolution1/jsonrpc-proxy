package main

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
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
	versionGuage := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: MetricsNs,
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
	prometheus.MustRegister(versionGuage)
	versionGuage.Set(1)
}
