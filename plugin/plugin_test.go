package plugin

import (
	assertion "github.com/stretchr/testify/assert"
	"testing"
)


const conf1 = `
- id: basic-auth
  username: admin
  password: password
- id: manage
`

const conf2 = `
- id: basic-auth
  username: admin
  password: password
- random: a
`

func TestLoadPluginConfig(t *testing.T) {
	assert := assertion.New(t)
	config, err := LoadPluginsConfig([]byte(conf1))
	assert.NoError(err)
	assert.Equal("basic-auth", string(config.ID))
	assert.Equal("admin", config.Spec["username"])
	assert.Equal("manage", string(config.Next.ID))
	assert.Nil(config.Next.Next)

	c2, err := LoadPluginsConfig([]byte(conf2))
	assert.Nil(c2)
	assert.Error(err)
}
