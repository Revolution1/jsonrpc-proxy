package main

import (
	"github.com/ghodss/yaml"
	assertion "github.com/stretchr/testify/assert"
	"testing"
)

func TestConfig(t *testing.T) {
	assert := assertion.New(t)
	conf, err := LoadConfig("./proxy.yaml")
	assert.NoError(err)
	out, _ := yaml.Marshal(conf)
	t.Log(string(out))

	cc := conf.Search("GetTxBlock")
	assert.NotNil(cc)
}
