package plugin

import (
	"github.com/revolution1/jsonrpc-proxy/types"
)

type ProcessorConfig struct {
	ID          string `json:"ID"`
	Description string `json:"description"`

	//Context the name of accept and provide context of this processor
	Context       string    `json:"context"`
	PluginConfigs []*Config `json:"plugins"`
}

func LoadProcessorConfig(raw types.RawProcessorConfig) (*ProcessorConfig, error) {
	var err error
	conf := &ProcessorConfig{ID: raw.ID, Description: raw.Description, Context: raw.Context}
	conf.PluginConfigs, err = LoadPluginConfigs(raw.PluginConfigs)
	if err != nil {
		return nil, err
	}
	return conf, nil
}
