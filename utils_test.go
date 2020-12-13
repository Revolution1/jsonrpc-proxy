package main

import (
	assertion "github.com/stretchr/testify/assert"
	"testing"
)

func TestIsAscii(t *testing.T) {
	assert := assertion.New(t)
	a := []byte("hello")
	b := []byte("哈喽")
	assert.True(isASCII(a))
	assert.False(isASCII(b))
}
