package types

import (
	"encoding/json"
	assertion "github.com/stretchr/testify/assert"
	"testing"
)

func TestSize(t *testing.T) {
	assert := assertion.New(t)
	cases := []struct {
		data  string
		bytes int
	}{
		{`"1"`, 1},
		{`1`, 1},
		{`"1b"`, 1},
		{`"1kb"`, 1000},
		{`"1kib"`, 1024},
	}

	for _, c := range cases{
		var s Size
		err := json.Unmarshal([]byte(c.data), &s)
		assert.NoError(err)
		assert.Equal(Size(c.bytes), s)
	}
	s := Size(1024)
	data, err := json.Marshal(&s)
	assert.NoError(err)
	assert.Equal(`"1.0 KiB"`, string(data))
}
