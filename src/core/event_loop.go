package core

import (
	"greactor/src/core/events"
	"greactor/src/core/netpoll"
	"sync/atomic"
)

type eventLoop struct {
	ln *listener // listener
	// 在事件循环线程组中的索引
	idx          int
	svr          *Server
	poller       *netpoll.Poller
	buffer       []byte
	connCount    int32
	connections  map[int]*conn
	eventHandler events.EventHandler
}

func (el *eventLoop) runSubReactor() {

}

func (el *eventLoop) closeAllSockets() {
	// Close loops and all outstanding connections
	for _, c := range el.connections {
		_ = el.closeConn(c, nil)
	}
}

func (el *eventLoop) closeConn(c *conn, err error) (rerr error) {
	if !c.opened {
		return
	}
}

func (el *eventLoop) write(c *conn) error {

}

func (el *eventLoop) read(c *conn) error {
}

func (el *eventLoop) addConn(delta int32) {
	atomic.AddInt32(&el.connCount, delta)
}
