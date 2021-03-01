package plugin

import (
	"github.com/pkg/errors"
	"github.com/revolution1/jsonrpc-proxy/proxyctx"
	"github.com/sirupsen/logrus"
	"sync"
)

type Processor interface {
	Plugin
	InitProcessor(config *ProcessorConfig) error
}

type processor struct {
	id          string
	contextType string
	description string

	config *ProcessorConfig

	pluginInstances []Plugin
	handler         func(next HandleFunc) HandleFunc

	logger      *logrus.Entry
	mu          sync.Mutex
	initialized bool
	info        *Info

	refCount uint32
}

func (p *processor) ID() string {
	return p.id
}

func (p *processor) New(*Config) (Plugin, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.refCount == 0 && !p.initialized {
		err := p.InitProcessor(p.config)
		if err != nil {
			return nil, err
		}
	}
	p.refCount++
	return p, nil
}

func (p *processor) InitProcessor(conf *ProcessorConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.initialized {
		panic("processor already initialized")
	} else {
		p.initialized = true
	}
	p.config = conf
	p.logger = log.WithField("processor", conf.ID)
	p.id = conf.ID
	p.description = conf.Description
	p.contextType = conf.Context
	p.info = nil
	// init child plugins
	prevCtxType := p.contextType
	for _, pluginConf := range conf.PluginConfigs {
		info, err := GetInfoOfID(pluginConf.ID)
		if err != nil {
			return err
		}
		if !info.CanHandle(prevCtxType) {
			return errors.Errorf("plugin %s cannot handle previous context %s of this processor", pluginConf.ID, prevCtxType)
		}
		plugin, err := NewPlugin(pluginConf)
		if err != nil {
			return err
		}
		p.pluginInstances = append(p.pluginInstances, plugin)
		prevCtxType, _ = info.ActualProvideContext(prevCtxType)
	}
	return nil
}

func (p *processor) Info() *Info {
	if p.info == nil {
		if !p.initialized {
			panic("processor has not been initialized")
		}
		p.info = &Info{
			ID:             p.id,
			Version:        "1",
			Description:    p.description,
			AcceptContexts: []string{p.contextType},
			ProvideContext: p.contextType,
		}
	}
	return p.info
}

func (p *processor) Handler(next HandleFunc) HandleFunc {
	f := next
	for i := len(p.pluginInstances) - 1; i >= 0; i-- {
		f = p.pluginInstances[i].Handler(f)
	}
	return func(ctx proxyctx.Context) (err error) {
		p.logger.Tracef("start handling in processor %s", p.id)
		err = f(ctx)
		p.logger.Tracef("done handling in processor %s", p.id)
		return
	}
}

func (p *processor) Destroy() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.refCount--
	if p.refCount > 0 {
		return
	}
	wg := &sync.WaitGroup{}
	wg.Add(len(p.pluginInstances))
	for _, plug := range p.pluginInstances {
		go func(plug Plugin) {
			defer wg.Done()
			p.logger.Infof("stopping child plugin: %s", plug.ID())
			p.Destroy()
			log.Infof("stopped child plugin: %s", plug.ID())
		}(plug)
	}
	wg.Wait()
}
