package types

import (
	"net/url"
)

type UpstreamID string
type UpstreamType int
type Endpoint url.URL

const (
	TCPUpstream UpstreamType = iota
	HTTPUpstream
)

type UpstreamConfig struct {
	ID               UpstreamID        `json:"id"`
	Type             UpstreamType      `json:"type"`
	Endpoints        []*Endpoint       `json:"endpoints"`
	ServiceDiscovery *ServiceDiscovery `json:"serviceDiscovery"`
}

type K8sServiceDiscover struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Port      int    `json:"port"`
}

type DNSDiscover struct {
	Name       string `json:"name"`
	EnableIPv6 bool   `json:"enableIPv6"`
}

type ServiceDiscovery struct {
	K8sService  *K8sServiceDiscover `json:"k8sService"`
	DNSDiscover *DNSDiscover        `json:"dnsRecord"`
}

//type HealthCheck struct {
//	Interval Duration
//	Timeout  Duration
//	HTTPGet  struct {
//		Port  int
//		Path  string
//		Codes []int
//	}
//	TCPSocket struct {
//	}
//}
