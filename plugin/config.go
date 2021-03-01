package plugin

import (
	"github.com/pkg/errors"
	"github.com/revolution1/jsonrpc-proxy/types"
)

//Config the config of plugin instance
type Config struct {
	ID   string
	Spec map[string]interface{}
}

func LoadPluginConfig(raw types.RawPluginConfig) (*Config, error) {
	conf := &Config{}
	if rawID, ok := raw["id"]; !ok {
		return nil, errors.New("missing required field 'id' in plugin config")
	} else if id, ok := rawID.(string); !ok {
		return nil, errors.New("field 'id' of plugin config should be string")
	} else {
		conf.ID = id
	}
	conf.Spec = make(map[string]interface{})
	for k, v := range raw {
		if k != "id" {
			conf.Spec[k] = v
		}
	}
	return conf, nil
}

func LoadPluginConfigs(raws []types.RawPluginConfig) (configs []*Config, err error) {
	for _, raw := range raws {
		conf, err := LoadPluginConfig(raw)
		if err != nil {
			return nil, err
		}
		configs = append(configs, conf)
	}
	return
}
