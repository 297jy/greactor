package core

import (
	"fmt"
	"golang.org/x/sys/unix"
	"greactor/src/buffers"
	"greactor/src/core/icodecs"
	"greactor/src/core/netpoll"
	"net"
	"os"
)

type Conn interface {
	Open(buf []byte) error

	Read() ([]byte, error)

	Write(buf []byte) (err error)

	Close(err error) (rerr error)
}

type conn struct {
	fd             int
	ctx            interface{}
	peer           unix.Sockaddr
	loop           *eventLoop
	codec          icodecs.ICodec
	buffer         []byte
	opened         bool
	localAddr      net.Addr
	remoteAddr     net.Addr
	inboundBuffer  *buffers.ByteBuffer
	outboundBuffer *buffers.ByteBuffer
	pollAttachment *netpoll.PollAttachment
}

func newTCPConn(fd int, el *eventLoop, sa unix.Sockaddr, codec icodecs.ICodec, localAddr, remoteAddr net.Addr) (c *conn) {
	c = &conn{
		fd:             fd,
		peer:           sa,
		loop:           el,
		codec:          codec,
		localAddr:      localAddr,
		remoteAddr:     remoteAddr,
		inboundBuffer:  buffers.GetByteBuffer(),
		outboundBuffer: buffers.GetByteBuffer(),
	}
	c.pollAttachment = netpoll.GetPollAttachment()
	c.pollAttachment.FD, c.pollAttachment.Callback = fd, c.handleEvents
	return
}

func (c *conn) handleEvents(_ int, ev uint32) error {
	if ev&netpoll.OutEvents != 0 && !c.outboundBuffer.IsEmpty() {
		if err := c.loop.write(c, nil); err != nil {
			return err
		}
	}

	if ev&netpoll.InEvents != 0 && (ev&netpoll.OutEvents == 0 || c.outboundBuffer.IsEmpty()) {
		return c.loop.read(c)
	}
	return nil
}

func (c *conn) Close(err error) (rerr error) {
	if !c.opened {
		return nil
	}

	if c.outboundBuffer.IsNotEmpty() {
		c.outboundBuffer.Reset()
	}

	err0, err1 := c.loop.poller.Delete(c.fd), unix.Close(c.fd)
	if err0 != nil {
		rerr = fmt.Errorf("failed to delete fd=%d from poller in event-loop(%d): %v", c.fd, c.loop.idx, err0)
	}
	if err1 != nil {
		err1 = fmt.Errorf("failed to close fd=%d in event-loop(%d): %v", c.fd, c.loop.idx, os.NewSyscallError("close", err1))
	}
	c.loop.addConn(-1)

	c.releaseTCP()
	return nil
}

func (c *conn) releaseTCP() {
	c.opened = false
	c.peer = nil
	c.ctx = nil
	c.buffer = nil

	c.localAddr = nil
	c.remoteAddr = nil
	buffers.PutByteBuffer(c.inboundBuffer)
	buffers.PutByteBuffer(c.outboundBuffer)
	netpoll.PutPollAttachment(c.pollAttachment)
	c.pollAttachment = nil
}

func (c *conn) Read() ([]byte, error) {
	n, err := unix.Read(c.fd, c.buffer)
	if n == 0 || err != nil {
		if err == unix.EAGAIN {
			return nil, nil
		}
		return nil, c.Close(os.NewSyscallError("read", err))
	}
	c.buffer = c.buffer[:n]
	c.inboundBuffer.Append(c.buffer)
	// 进行解码
	data, err := c.codec.Decode(c.inboundBuffer.Bytes())
	if err != nil {
		return nil, err
	}

	c.inboundBuffer.ShiftN(len(data))
	return data, nil
}

func (c *conn) Open(buf []byte) (err error) {
	defer c.loop.eventHandler.AfterWrite(c, buf)

	c.loop.eventHandler.PreWrite(c)

	return c.Write(buf)
}

func (c *conn) Write(buf []byte) (err error) {

	c.outboundBuffer.Append(buf)

	var packet []byte
	if packet, err = c.codec.Encode(c.outboundBuffer.Bytes()); err != nil {
		return
	}

	var n int
	if n, err = unix.Write(c.fd, packet); err != nil {
		// 这个错误说明 写事件操作还没完成
		if err == unix.EAGAIN {
			c.outboundBuffer.Append(packet)
			// 让轮询器 监听写事件就绪
			err = c.loop.poller.ModReadWrite(c.pollAttachment)
			return
		}
		return c.Close(err)
	}

	c.outboundBuffer.ShiftN(n)
	if n < len(packet) {
		// 让轮询器 监听写事件就绪
		err = c.loop.poller.ModReadWrite(c.pollAttachment)
	}
	if c.outboundBuffer.IsEmpty() {
		err = c.loop.poller.ModRead(c.pollAttachment)
	}
	return
}
