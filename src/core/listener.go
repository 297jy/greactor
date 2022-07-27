package core

import (
	"fmt"
	"golang.org/x/sys/unix"
	"greactor/src/core/netpoll"
	"greactor/src/errors"
	"greactor/src/socket"
	"os"
	"sync"
)

type listener struct {
	once           sync.Once
	fd             int
	saddr          *socket.ServerAddr
	sockOpts       []socket.Option
	pollAttachment *netpoll.PollAttachment // listener attachment for poller
}

func (ln *listener) packPollAttachment(handler netpoll.PollEventHandler) *netpoll.PollAttachment {
	ln.pollAttachment = &netpoll.PollAttachment{FD: ln.fd, Callback: handler}
	return ln.pollAttachment
}

func (ln *listener) prepare() (err error) {
	switch ln.saddr.Network {
	case "tcp", "tcp4", "tcp6":
		ln.fd, err = socket.TCPSocket(ln.saddr, true, ln.sockOpts...)
	default:
		err = errors.ErrUnsupportedProtocol
	}
	return
}

func initListener(addr *socket.ServerAddr, options *Options) (l *listener, err error) {
	var sockOpts []socket.Option
	l = &listener{saddr: addr, sockOpts: sockOpts}
	err = l.prepare()
	return
}

func (ln *listener) close() {
	ln.once.Do(
		func() {
			if ln.fd > 0 {
				fmt.Println(os.NewSyscallError("close", unix.Close(ln.fd)).Error())
			}
			if ln.saddr.Network == "unix" {
				fmt.Println(os.RemoveAll(ln.saddr.Address).Error())
			}
		})
}
