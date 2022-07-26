package core

import (
	"fmt"
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

func (el *eventLoop) activateMainReactor() {
	defer el.svr.signalShutdown()

	err := el.poller.Polling(func(fd int, ev uint32) error { return el.svr.accept(fd, ev) })
	if err == errors.ErrServerShutdown {
		fmt.Printf("main reactor is exiting in terms of the demand from user, %v", err)
	} else if err != nil {
		fmt.Printf("main reactor is exiting due to error: %v", err)
	}
}

func (el *eventLoop) activateSubReactor() {
	defer func() {
		el.closeAllSockets()
		el.svr.signalShutdown()
	}()

	err := el.poller.Polling(func(fd int, ev uint32) error {
		if c, ack := el.connections[fd]; ack {
			if ev&netpoll.OutEvents != 0 && !c.outboundBuffer.IsEmpty() {
				if err := el.write(c, []byte{}); err != nil {
					return err
				}
			}
			if ev&netpoll.InEvents != 0 && (ev&netpoll.OutEvents == 0 || c.outboundBuffer.IsEmpty()) {
				return el.read(c)
			}
		}
		return nil
	})

	if err == errors.ErrServerShutdown {
		fmt.Printf("event-loop(%d) is exiting in terms of the demand from user, %v", el.idx, err)
	} else if err != nil {
		fmt.Printf("event-loop(%d) is exiting due to error: %v", el.idx, err)
	}
}

func (el *eventLoop) closeAllSockets() {
	// Close loops and all outstanding connections
	for _, c := range el.connections {
		_ = el.closeConn(c, nil)
	}
}

func (el *eventLoop) closeConn(c *conn, err error) (rerr error) {
	rerr = c.Close(err)
	if rerr != nil {
		return
	}
	if el.eventHandler.OnClosed(c, err) == events.Shutdown {
		rerr = errors.ErrServerShutdown
	}
	return
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

func (el *eventLoop) register(itf interface{}) error {
	c := itf.(*conn)
	if err := el.poller.AddRead(c.pollAttachment); err != nil {
		_ = unix.Close(c.fd)
		c.releaseTCP()
		return err
	}
	el.connections[c.fd] = c
	return el.open(c)
}

func (el *eventLoop) open(c *conn) error {
	c.opened = true
	el.addConn(1)

	out, action := el.eventHandler.OnOpened(c)
	if out != nil {
		if err := c.Open(out); err != nil {
			return err
		}
	}

	if !c.outboundBuffer.IsEmpty() {
		if err := el.poller.AddWrite(c.pollAttachment); err != nil {
			return err
		}
	}

	return el.handleAction(c, action)
}

func (el *eventLoop) handleAction(c *conn, action events.Action) error {
	switch action {
	case events.None:
		return nil
	case events.Close:
		return el.closeConn(c, nil)
	case events.Shutdown:
		return errors.ErrServerShutdown
	default:
		return nil
	}
}
