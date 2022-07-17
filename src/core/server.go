package core

import (
	"golang.org/x/sys/unix"
	"greactor/src/core/event"
	"greactor/src/core/socket"
	"net"
	"sync"
)

type ServerAddr struct {
	Sa      unix.Sockaddr
	NetAddr net.Addr
	Address string
	Network string
	Family  int // 协议族
}

type Server struct {
	ln           *listener
	lb           loadBalancer
	wg           sync.WaitGroup
	opts         *Options
	once         sync.Once
	cond         *sync.Cond
	bossLoop     *eventLoop
	inShutdown   int32
	eventHandler event.EventHandler
	addr         *ServerAddr
}

func NewServer(eventHandler event.EventHandler, protoAddr string, opts *Options) (s *Server, err error) {
	var (
		family  int
		sa      unix.Sockaddr
		netAddr net.Addr
	)
	network, address := socket.ParseProtoAddr(protoAddr)
	if sa, family, netAddr, _, err = socket.GetTCPSockAddr(network, address); err != nil {
		return nil, err
	}
	addr := &ServerAddr{Sa: sa, NetAddr: netAddr, Address: address, Family: family, Network: network}
	s = &Server{eventHandler: eventHandler, addr: addr, opts: opts}
	return
}

func (s *Server) Run() (err error) {
	var ln *listener
	if ln, err = initListener(s.addr, s.opts); err != nil {
		return
	}
	defer ln.close()
	return
}
