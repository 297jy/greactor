package core

import (
	"golang.org/x/sys/unix"
	"greactor/src/core/events"
	"greactor/src/core/icodecs"
	"greactor/src/core/netpoll"
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

const (
	// DefaultBufferSize is the first-time allocation on a ring-buffers.
	DefaultBufferSize   = 1024     // 1KB
	bufferGrowThreshold = 4 * 1024 // 4KB
)

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
	for i := 0; i < numEventLoop; i++ {
		if p, err := netpoll.OpenPoller(); err == nil {
			el := new(eventLoop)
			el.ln = s.ln
			el.svr = s
			el.poller = p
			el.buffer = make([]byte, DefaultBufferSize)
			el.connections = make(map[int]*conn)
			el.eventHandler = s.eventHandler
			s.lb.register(el)
		} else {
			return err
		}
	}
	return nil
}
