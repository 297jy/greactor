package core

import "net"

type (
	// eventLoop负载均衡算法
	loadBalancer interface {
		register(*eventLoop)
		next(net.Addr) *eventLoop
		iterate(func(int, *eventLoop) bool)
		len() int
	}

	// roundRobinLoadBalancer with Round-Robin algorithm.
	roundRobinLoadBalancer struct {
		nextLoopIndex int
		eventLoops    []*eventLoop
		size          int
	}
)

func (lb *roundRobinLoadBalancer) register(el *eventLoop) {
	el.idx = lb.size
	lb.eventLoops = append(lb.eventLoops, el)
	lb.size++
}

// next returns the eligible event-loop based on Round-Robin algorithm.
func (lb *roundRobinLoadBalancer) next(_ net.Addr) (el *eventLoop) {
	el = lb.eventLoops[lb.nextLoopIndex]
	if lb.nextLoopIndex++; lb.nextLoopIndex >= lb.size {
		lb.nextLoopIndex = 0
	}
	return
}

func (lb *roundRobinLoadBalancer) iterate(f func(int, *eventLoop) bool) {
	for i, el := range lb.eventLoops {
		if !f(i, el) {
			break
		}
	}
}

func (lb *roundRobinLoadBalancer) len() int {
	return lb.size
}
