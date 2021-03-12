package statistic

import "testing"

func TestCollector(t *testing.T) {
	c := NewCollector()
	go c.Run()
	c.Stop()
}
