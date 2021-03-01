package types

type Config struct {
	Version    string                `json:"version"`
	Listeners  []*ListenerConfig     `json:"listeners"`
	Upstreams  []*UpstreamConfig     `json:"upstreams"`
	Servers    []*RawServerConfig    `json:"servers"`
	Processors []*RawProcessorConfig `json:"processors"`
}

type RawProcessorConfig struct {
	ID            string            `json:"ID"`
	Description   string            `json:"description"`
	Context       string            `json:"context"`
	PluginConfigs []RawPluginConfig `json:"plugins"`
}

type RawPluginConfig map[string]interface{}

type RawServerConfig map[string]interface{}
