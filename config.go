package main

import (
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/url"
	"sigs.k8s.io/yaml"
	"sort"
	"strings"
)

type Config struct {
	LogLevel               string        `json:"logLevel"`
	LogForceColors         bool          `json:"logForceColors"`
	Debug                  bool          `json:"debug"`
	AccessLog              bool          `json:"accessLog"`
	Manage                 ManageConfig  `json:"manage"`
	Upstreams              []string      `json:"upstreams"`
	Listen                 string        `json:"listen"`
	Path                   string        `json:"path"`
	KeepAlive              string        `json:"keepAlive"`
	UpstreamRequestTimeout Duration      `json:"upstreamRequestTimeout"`
	ReadTimeout            Duration      `json:"readTimeout"`
	WriteTimeout           Duration      `json:"writeTimeout"`
	IdleTimeout            Duration      `json:"idleTimeout"`
	ErrFor                 Duration      `json:"errFor"`
	CacheConfigs           []CacheConfig `json:"cacheConfigs"`
}

type ManageConfig struct {
	Listen      string `json:"listen"`
	Path        string `json:"path"`
	MetricsPath string `json:"metricsPath"`
}

type CacheConfig struct {
	Methods []string `json:"methods"`
	For     Duration `json:"for"`
	ErrFor  Duration `json:"errFor"`
}

func (cc *CacheConfig) Sort() {
	sort.Strings(cc.Methods)
}

func LoadConfig(path string) (conf *Config, err error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	conf = new(Config)
	err = yaml.UnmarshalStrict(content, conf)
	for _, c := range conf.CacheConfigs {
		c.Sort()
	}
	return
}

func (c *Config) Search(method string) *CacheConfig {
	for _, cc := range c.CacheConfigs {
		i := sort.SearchStrings(cc.Methods, method)
		if i < len(cc.Methods) && cc.Methods[i] == method {
			return &cc
		}
	}
	return nil
}

func validateListen(l string) error {
	if strings.Index(l, "://") == -1 {
		l = "http://" + l
	}
	u, err := url.Parse(l)
	if err != nil {
		return err
	}
	if u.Hostname() == "" || u.Port() == "" {
		return errors.New("Invalid listen address")
	}
	return nil
}

func (c Config) Validate() error {
	err := validateListen(c.Listen)
	if err != nil {
		return errors.Wrap(err, "config.listen address is not valid")
	}
	if !strings.HasPrefix(c.Path, "/") {
		return errors.New("config.path is not valid")
	}
	if c.Manage.Path != "" && !strings.HasPrefix(c.Manage.Path, "/") {
		return errors.New("config.manage.Path is not valid")
	}
	if c.Manage.MetricsPath != "" && !strings.HasPrefix(c.Manage.MetricsPath, "/") {
		return errors.New("config.manage.metricsPath is not valid")
	}
	if len(c.Upstreams) == 0 {
		return errors.New("config.upstreams is empty")
	}
	return nil
}

func (c Config) MustValidate() {
	if err := c.Validate(); err != nil {
		log.WithError(err).Fatal("invalid config")
	}
}

func ParseUpstream(addr string) string {
	u, err := url.Parse(addr)
	if err != nil {
		log.Fatal(errors.Wrap(err, fmt.Sprintf("unable to parse upstream %s", addr)))
	}
	return u.Host
}
