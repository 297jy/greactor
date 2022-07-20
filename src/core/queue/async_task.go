package queue

import (
	"sync/atomic"
	"unsafe"
)

// 使用cas机制实现无锁队列
type AsyncTaskQueue interface {
	Enqueue(*Task)
	Dequeue() *Task
	IsEmpty() bool
}

type lockFreeQueue struct {
	head   unsafe.Pointer
	tail   unsafe.Pointer
	length int32
}

type node struct {
	value *Task
	next  unsafe.Pointer
}

func NewLockFreeQueue() AsyncTaskQueue {
	n := unsafe.Pointer(&node{})
	return &lockFreeQueue{head: n, tail: n}
}

func (q *lockFreeQueue) Enqueue(task *Task) {
	n := &node{value: task}

	for ; ; {
		tail := load(&q.tail)
		next := load(&tail.next)

		if tail == load(&q.tail) {
			if next == nil {
				if cas(&tail.next, next, n) {
					// 必须用cas更新队列的tail，因为可能别的线程也在更新队列的tail，如果直接赋值会导致并发问题
					cas(&q.tail, tail, n)
					atomic.AddInt32(&q.length, 1)
					return
				}
			} else {
				// 尝试将队列的tail节点更新为next节点
				cas(&q.tail, tail, next)
			}
		}
	}

}

func (q *lockFreeQueue) Dequeue() *Task {
	for ; ; {
		head := load(&q.head)
		tail := load(&q.tail)
		next := load(&head.next)
		if head == load(&q.head) {
			if head == tail {
				// 队列为空直接返回
				if next == nil {
					return nil
				}
				cas(&q.tail, tail, next) // 发现队列已经有新加入的next节点，将队列的tail节点设置成next节点
			} else {
				task := next.value
				if cas(&q.head, head, next) {
					atomic.AddInt32(&q.length, -1)
					return task
				}
			}
		}
	}
}

// 保证内存的可见性，判断队列是否为空
func (q *lockFreeQueue) IsEmpty() bool {
	return atomic.LoadInt32(&q.length) == 0
}

// 保证内存的可见性
func load(p *unsafe.Pointer) (n *node) {
	return (*node)(atomic.LoadPointer(p))
}

// cas机制
func cas(p *unsafe.Pointer, old, new *node) bool {
	return atomic.CompareAndSwapPointer(p, unsafe.Pointer(old), unsafe.Pointer(new))
}
