package core

import (
	"golang.org/x/sys/unix"
	"greactor/src/core/events"
	"greactor/src/core/netpoll"
	"greactor/src/errors"
	"os"
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
	return c.Close(err)
}

func (el *eventLoop) write(c *conn, buf []byte) (err error) {
	defer c.loop.eventHandler.AfterWrite(c, buf)

	el.eventHandler.PreWrite(c)
	err = c.Write(buf)
	switch err {
	case nil:
	case unix.EAGAIN:
		return nil
	default:
		return el.closeConn(c, os.NewSyscallError("write", err))
	}

	if c.outboundBuffer.IsEmpty() {
		_ = el.poller.ModRead(c.pollAttachment)
	}

	return
}

func (el *eventLoop) read(c *conn) (err error) {
	for packet, _ := c.Read(); packet != nil; packet, _ = c.Read() {
		out, action := el.eventHandler.React(packet, c)
		if out != nil {
			if err = el.write(c, out); err != nil {
				return err
			}
		}
		switch action {
		case events.None:
		case events.Close:
			return el.closeConn(c, nil)
		case events.Shutdown:
			return errors.ErrServerShutdown
		}

		if !c.opened {
			return nil
		}
	}
	return
}

func (el *eventLoop) addConn(delta int32) {
	atomic.AddInt32(&el.connCount, delta)
}
