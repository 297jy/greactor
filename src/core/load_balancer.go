package core

import (
	"net"
)

// 当一个新连接建立时，分配给事件循环组中的某个event-loop处理，所使用的负载均衡算法
type LoadBalancing int

const (
	RoundRobin LoadBalancing = iota

)

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

// next returns the eligible events-loop based on Round-Robin algorithm.
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
