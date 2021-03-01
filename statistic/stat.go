package statistic

const (
	statBufSize = 64 * 1024
)

type stat struct {
	address   string
	method    string
	params    interface{}
	useragent string
}

type StatCollector struct {
	ch   chan *stat
	stop chan struct{}
}
