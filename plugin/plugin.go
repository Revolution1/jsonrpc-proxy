package plugin

import (
	"github.com/pkg/errors"
	"github.com/revolution1/jsonrpc-proxy/proxyctx"
)

type HandleFunc func(ctx proxyctx.Context) error

//DefaultTerminator terminate a context flow
func DefaultTerminator(proxyctx.Context) error { return nil }

type Plugin interface {
	ID() string
	Info() *Info
	New(*Config) (Plugin, error)
	Handler(HandleFunc) HandleFunc
	Destroy()
}

type Info struct {
	ID             string
	Version        string
	Description    string
	AcceptContexts []string
	ProvideContext string
}

func (p *Info) CanHandle(contextType string) bool {
	for _, i := range p.AcceptContexts {
		if i == "*" {
			return true
		}
		if i == contextType {
			return true
		}
	}
	return false
}

func (p *Info) ActualProvideContext(input string) (string, error) {
	if p.ProvideContext != "" {
		return p.ProvideContext, nil
	}
	if !p.CanHandle(input) {
		return "", errors.Errorf("plugin %s cannot handle context type %s", p.ID, input)
	}
	return input, nil
}
