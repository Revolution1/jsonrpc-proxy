package types

import (
	"fmt"
	"net"
	"strings"
)

type ListenerID string

type ListenerConfig struct {
	ID      ListenerID `json:"id"`
	Address string     `json:"address"`
	Scheme  string     `json:"scheme"`
	TLS     *struct {
		Enabled    bool   `json:"enabled"`
		Cert       string `json:"cert"`
		Key        string `json:"key"`
		CommonName string `json:"commonName"`
	} `json:"tls"`
}

func (c ListenerConfig) Listen() (net.Listener, error) {
	var network = strings.ToLower(c.Scheme)
	switch network {
	case "tcp", "unix":
		return net.Listen(c.Scheme, c.Address)
	default:
		panic(fmt.Sprintf("listener scheme %s not supported", network))
	}
}
