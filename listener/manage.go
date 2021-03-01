package listener

import (
	"github.com/pkg/errors"
	"github.com/revolution1/jsonrpc-proxy/server"
	"github.com/revolution1/jsonrpc-proxy/types"
	"net"
	"sync"
)

var (
	listens  map[types.ListenerID]net.Listener
	acquired map[types.ListenerID]server.ID
	lmu      sync.Mutex
)

func AcquireListener(id types.ListenerID) (net.Listener, error) {
	lmu.Lock()
	defer lmu.Unlock()
	if a, ok := acquired[id]; ok {
		return nil, errors.Errorf("Listener already acquired by server %s", a)
	}
	if l, ok := listens[id]; ok {
		return l, nil
	}
	return nil, errors.Errorf("Listener %s not found", id)
}
