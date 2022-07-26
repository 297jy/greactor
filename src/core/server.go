package core

import (
	"fmt"
	"golang.org/x/sys/unix"
	"greactor/src/core/events"
	"greactor/src/core/icodecs"
	"greactor/src/core/netpoll"
	"greactor/src/core/socket"
	"greactor/src/errors"
	"net"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
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
	mainLoop     *eventLoop
	inShutdown   int32
	eventHandler events.EventHandler
	addr         *ServerAddr
}

const (
	// DefaultBufferSize is the first-time allocation on a ring-buffers.
	DefaultBufferSize   = 1024     // 1KB
	bufferGrowThreshold = 4 * 1024 // 4KB
)

var (
	allServers sync.Map

	// shutdownPollInterval is how often we poll to check whether server has been shut down during gnet.Stop().
	shutdownPollInterval = 500 * time.Millisecond
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
	numEventLoop := 1
	if s.opts.Multicore {
		numEventLoop = runtime.NumCPU()
	}

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

	if err = s.runReactors(numEventLoop); err != nil {
		s.closeEventLoops()
		fmt.Printf("gnet server is stopping with error: %v", err)
		return err
	}
	defer s.stop()

	allServers.Store(s.addr.Address, s)
	return
}

func (s *Server) runReactors(numEventLoop int) error {
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

	s.runSubReactors()

	if p, err := netpoll.OpenPoller(); err == nil {
		el := new(eventLoop)
		el.ln = s.ln
		el.idx = -1
		el.svr = s
		el.poller = p
		el.eventHandler = s.eventHandler
		if err = el.poller.AddRead(s.ln.packPollAttachment(s.accept)); err != nil {
			return err
		}
		s.mainLoop = el

		s.wg.Add(1)
		go func() {
			el.activateMainReactor()
			s.wg.Done()
		}()
	} else {
		return err
	}

	return nil
}

func (s *Server) accept(fd int, _ events.IOEvent) error {
	nfd, sa, err := unix.Accept(fd)
	if err != nil {
		if err == unix.EAGAIN {
			return nil
		}
		fmt.Printf("Accept() fails due to error: %v", err)
		return errors.ErrAcceptSocket
	}
	if err = os.NewSyscallError("fcntl nonblock", unix.SetNonblock(nfd, true)); err != nil {
		return err
	}

	remoteAddr := socket.SockaddrToTCPOrUnixAddr(sa)

	el := s.lb.next(remoteAddr)
	c := newTCPConn(nfd, el, sa, s.opts.Codec, el.ln.saddr.NetAddr, remoteAddr)

	err = el.poller.Trigger(el.register, c)
	if err != nil {
		_ = unix.Close(nfd)
		c.releaseTCP()
	}
	return nil
}

func (s *Server) runSubReactors() {

	s.lb.iterate(func(i int, loop *eventLoop) bool {
		s.wg.Add(1)
		go func() {
			loop.activateSubReactor()
			s.wg.Done()
		}()
		return true
	})
}

func (s *Server) signalShutdown() {
	s.once.Do(func() {
		s.cond.L.Lock()
		s.cond.Signal()
		s.cond.L.Unlock()
	})
}

func (s *Server) closeEventLoops() {
	s.lb.iterate(func(i int, el *eventLoop) bool {
		_ = el.poller.Close()
		return true
	})
}

func (s *Server) stop() {
	// Wait on a signal for shutdown
	s.waitForShutdown()

	s.eventHandler.OnShutdown(s)

	// Notify all loops to close by closing all listeners
	s.lb.iterate(func(i int, el *eventLoop) bool {
		err := el.poller.Trigger(func(_ interface{}) error { return errors.ErrServerShutdown }, nil)
		if err != nil {
			fmt.Printf("failed to call UrgentTrigger on sub event-loop when stopping server: %v", err)
		}
		return true
	})

	if s.mainLoop != nil {
		s.ln.close()
		err := s.mainLoop.poller.Trigger(func(_ interface{}) error { return errors.ErrServerShutdown }, nil)
		if err != nil {
			fmt.Printf("failed to call UrgentTrigger on main event-loop when stopping server: %v", err)
		}
	}

	// Wait on all loops to complete reading events
	s.wg.Wait()

	s.closeEventLoops()

	if s.mainLoop != nil {
		err := s.mainLoop.poller.Close()
		if err != nil {
			fmt.Printf("failed to close poller when stopping server: %v", err)
		}
	}

	atomic.StoreInt32(&s.inShutdown, 1)
}

func (s *Server) waitForShutdown() {
	s.cond.L.Lock()
	s.cond.Wait()
	s.cond.L.Unlock()
}
