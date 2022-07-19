package core

import (
	"golang.org/x/sys/unix"
	"greactor/src/core/events"
	"greactor/src/core/icodecs"
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
	eventHandler events.EventHandler
	addr         *ServerAddr
}

func NewServer(eventHandler events.EventHandler, protoAddr string, opts *Options) (s *Server, err error) {
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
	s.init()
	return
}

func (s *Server) init() {
	switch s.opts.LB {
	case RoundRobin:
		s.lb = new(roundRobinLoadBalancer)
	default:
		s.lb = new(roundRobinLoadBalancer)
	}

	s.cond = sync.NewCond(&sync.Mutex{})
	if s.opts.Codec == nil {
		s.opts.Codec = new(icodecs.BuiltInFrameCodec)
	}
}

func (s *Server) Run() (err error) {
	var ln *listener
	if ln, err = initListener(s.addr, s.opts); err != nil {
		return
	}
	defer ln.close()

	switch s.eventHandler.OnInitComplete(s) {
	case events.None:
	case events.Shutdown:
		return nil
	}

	return
}

func (s *Server) activeReactors(numEventLoop int) error {
	return nil
}
