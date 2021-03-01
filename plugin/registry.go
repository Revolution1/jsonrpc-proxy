package plugin

import (
	"fmt"
	"github.com/pkg/errors"
)

var pluginRegistry map[string]Plugin

func RegisterPlugin(p Plugin) {
	info := p.Info()
	if _, ok := pluginRegistry[info.ID]; ok {
		panic(fmt.Sprintf("plugin %s already registered", info.ID))
	}
	pluginRegistry[info.ID] = p
}

func DeregisterPlugin(id string) {
	if _, ok := pluginRegistry[id]; ok {
		delete(pluginRegistry, id)
	}
}

func GetInfoOfID(id string) (*Info, error) {
	p, ok := pluginRegistry[id]
	if !ok {
		return nil, errors.Errorf("plugin %s not found", id)
	}
	return p.Info(), nil
}

func NewPlugin(config *Config) (Plugin, error) {
	p, ok := pluginRegistry[config.ID]
	if !ok {
		return nil, errors.Errorf("plugin %s not found", config.ID)
	}
	return p.New(config)
}
