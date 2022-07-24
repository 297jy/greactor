package netpoll

import (
	"fmt"
	"golang.org/x/sys/unix"
	"greactor/src/core/queue"
	"greactor/src/errors"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

const (
	readEvents      = unix.EPOLLPRI | unix.EPOLLIN
	writeEvents     = unix.EPOLLOUT
	readWriteEvents = readEvents | writeEvents

	ErrEvents = unix.EPOLLERR | unix.EPOLLHUP | unix.EPOLLRDHUP
	// OutEvents combines EPOLLOUT event and some exceptional events.
	OutEvents = ErrEvents | unix.EPOLLOUT
	// InEvents combines EPOLLIN/EPOLLPRI events and some exceptional events.
	InEvents = ErrEvents | unix.EPOLLIN | unix.EPOLLPRI
)

var (
	u uint16 = 1
	b        = (*(*[8]byte)(unsafe.Pointer(&u)))[:]
)

// 当epoll上监听的fd有I/O事件触发时，会调用这个回调函数
type PollEventHandler func(int, uint32) error

type PollAttachment struct {
	FD       int
	Callback PollEventHandler
}

type Poller struct {
	fd             int    // epoll 文件描述符
	wfd            int    // 事件队列 文件描述符，后面用于当异步事件触发时，唤醒轮训器进行事件处理
	wfdBuf         []byte // 事件队列缓冲区
	netpollWakeSig int32
	asyncTaskQueue queue.AsyncTaskQueue // 异步事件队列
}

func OpenPoller() (poller *Poller, err error) {
	poller = new(Poller)
	if poller.fd, err = unix.EpollCreate1(unix.EPOLL_CLOEXEC); err != nil {
		poller = nil
		err = os.NewSyscallError("epoll_create1", err)
		return
	}
	if poller.wfd, err = unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC); err != nil {
		_ = poller.Close()
		poller = nil
		err = os.NewSyscallError("eventfd", err)
		return
	}

	poller.wfdBuf = make([]byte, 8)
	if err = poller.AddRead(&PollAttachment{FD: poller.wfd}); err != nil {
		_ = poller.Close()
		poller = nil
		return
	}

	poller.asyncTaskQueue = queue.NewLockFreeQueue()
	return
}

type epollevent = unix.EpollEvent

type eventList struct {
	size   int
	events []epollevent
}

func newEventList(size int) *eventList {
	return &eventList{size, make([]epollevent, size)}
}

func (el *eventList) expand() {
	if newSize := el.size << 1; newSize <= MaxPollEventsCap {
		el.size = newSize
		el.events = make([]epollevent, newSize)
	}
}

func (el *eventList) shrink() {
	if newSize := el.size >> 1; newSize >= MinPollEventsCap {
		el.size = newSize
		el.events = make([]epollevent, newSize)
	}
}

const (
	// 轮询器初始的事件列表数量
	InitPollEventsCap = 128
	// 轮询器事件列表最大数量
	MaxPollEventsCap = 1024
	// 轮询器事件列表最小数量
	MinPollEventsCap = 32
	// 轮询器被唤醒一次，最多执行的异步任务个数
	MaxAsyncTasksAtOneTime = 256
)

func (p *Poller) Polling(callback func(fd int, ev uint32) error) error {
	el := newEventList(InitPollEventsCap)
	var wakenUp bool

	msec := -1
	for {
		n, err := unix.EpollWait(p.fd, el.events, msec)
		if n == 0 || (n < 0 && err == unix.EINTR) {
			msec = -1
			runtime.Gosched()
			continue
		} else if err != nil {
			fmt.Printf("error occurs in epoll: %v", os.NewSyscallError("epoll_wait", err))
			return err
		}
		msec = 0

		for i := 0; i < n; i++ {
			ev := &el.events[i]
			if fd := int(ev.Fd); fd != p.wfd {
				switch err = callback(fd, ev.Events); err {
				case nil:
				case errors.ErrAcceptSocket, errors.ErrServerShutdown:
					return err
				default:
					fmt.Printf("error occurs in event-loop: %v", err)
				}
			} else {
				// 轮训器被唤醒，开始执行异步任务队列的任务
				wakenUp = true
				_, _ = unix.Read(p.wfd, p.wfdBuf)
			}
		}

		if wakenUp {
			wakenUp = false
			for i := 0; i < MaxAsyncTasksAtOneTime; i++ {
				var task *queue.Task
				if task = p.asyncTaskQueue.Dequeue(); task == nil {
					break
				}
				// 执行异步任务队列里的任务
				switch err = task.Run(task.Arg); err {
				case nil:
				case errors.ErrServerShutdown:
					return err
				default:
					fmt.Printf("error occurs in user-defined function, %v", err)
				}
				queue.PutTask(task)
			}

			atomic.StoreInt32(&p.netpollWakeSig, 0)
			// 保证线程安全，可能出现多个线程往缓冲区写数据的情况
			if !p.asyncTaskQueue.IsEmpty() && atomic.CompareAndSwapInt32(&p.netpollWakeSig, 0, 1) {
				for _, err = unix.Write(p.wfd, b); err == unix.EINTR || err == unix.EAGAIN; _, err = unix.Write(p.wfd, b) {
				}
			}
		}

		if n == el.size {
			el.expand()
		} else if n < el.size>>1 {
			el.shrink()
		}
	}
}

func (p *Poller) AddRead(pa *PollAttachment) error {
	return os.NewSyscallError("epoll_ctl add",
		unix.EpollCtl(p.fd, unix.EPOLL_CTL_ADD, pa.FD, &unix.EpollEvent{Fd: int32(pa.FD), Events: readEvents}))
}

func (p *Poller) AddWrite(pa *PollAttachment) error {
	return os.NewSyscallError("epoll_ctl add",
		unix.EpollCtl(p.fd, unix.EPOLL_CTL_ADD, pa.FD, &unix.EpollEvent{Fd: int32(pa.FD), Events: writeEvents}))
}

func (p *Poller) ModReadWrite(pa *PollAttachment) error {
	return os.NewSyscallError("epoll_ctl mod",
		unix.EpollCtl(p.fd, unix.EPOLL_CTL_MOD, pa.FD, &unix.EpollEvent{Fd: int32(pa.FD), Events: readWriteEvents}))
}

func (p *Poller) ModRead(pa *PollAttachment) error {
	return os.NewSyscallError("epoll_ctl mod",
		unix.EpollCtl(p.fd, unix.EPOLL_CTL_MOD, pa.FD, &unix.EpollEvent{Fd: int32(pa.FD), Events: readEvents}))
}

func (p *Poller) Delete(fd int) error {
	return os.NewSyscallError("epoll_ctl del", unix.EpollCtl(p.fd, unix.EPOLL_CTL_DEL, fd, nil))
}

func (p *Poller) Close() error {
	if err := os.NewSyscallError("close", unix.Close(p.fd)); err != nil {
		return err
	}
	return os.NewSyscallError("close", unix.Close(p.wfd))
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
