package quote

import (
	"strconv"
	"testing"
)

func TestQuote(t *testing.T) {
	const line = "aa\\nbb"
	t.Log(line)
	s, err := strconv.Unquote(line)
	t.Log("unquoted:", s, "err:", err)

	v, mb, tail, err := strconv.UnquoteChar(`\"Fran & Freddie's Diner\"`, '"')
	if err != nil {
		t.Log(err)
	}

	t.Log("value:", string(v))
	t.Log("multibyte:", mb)
	t.Log("tail:", tail)
}
