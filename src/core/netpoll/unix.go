package netpoll

import "sync"

// 当epoll上监听的fd有I/O事件触发时，会调用这个回调函数
type PollEventHandler func(int, uint32) error

type PollAttachment struct {
	FD       int
	Callback PollEventHandler
}

var pollAttachmentPool = sync.Pool{New: func() interface{} { return new(PollAttachment) }}

// 尝试从缓存中获取PollAttachment对象，如果获取不到就会创建一个PollAttachment对象
func GetPollAttachment() *PollAttachment {
	return pollAttachmentPool.Get().(*PollAttachment)
}

// 将不用的PollAttachment对象放回缓存中，方便后面复用
func PutPollAttachment(pa *PollAttachment) {
	if pa == nil {
		return
	}
	pa.FD, pa.Callback = 0, nil
	pollAttachmentPool.Put(pa)
}