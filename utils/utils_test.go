package utils

import (
	assertion "github.com/stretchr/testify/assert"
	"testing"
)

func TestIsAscii(t *testing.T) {
	assert := assertion.New(t)
	a := []byte("hello")
	b := []byte("哈喽")
	assert.True(IsASCII(a))
	assert.False(IsASCII(b))
}

func TestCastToStruct(t *testing.T) {
	assert := assertion.New(t)
	s := struct {
		A string `json:"a"`
		B string `json:"b"`
	}{}
	m := map[string]string{
		"A": "a",
		"B": "b",
	}
	assert.NoError(CastToStruct(m, &s))
	assert.Equal("a", s.A)
}
