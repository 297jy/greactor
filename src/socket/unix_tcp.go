package socket

import (
	"golang.org/x/sys/unix"
	"os"
)

// backlog参数的最大值
var listenerBacklogMaxSize = unix.SOMAXCONN

func TCPSocket(addr *ServerAddr, passive bool, sockOpts ...Option) (int, error) {
	return tcpSocket(addr, passive, sockOpts...)
}

func tcpSocket(addr *ServerAddr, passive bool, sockOpts ...Option) (fd int, err error) {

	if fd, err = sysSocket(addr.Family, unix.SOCK_STREAM, unix.IPPROTO_TCP); err != nil {
		err = os.NewSyscallError("socket", err)
		return
	}
	defer func() {
		if err != nil {
			_ = unix.Close(fd)
		}
	}()

	for _, sockOpt := range sockOpts {
		if err = sockOpt.SetSockOpt(fd, sockOpt.Opt); err != nil {
			return
		}
	}

	if err = os.NewSyscallError("bind", unix.Bind(fd, addr.Sa)); err != nil {
		return
	}

	if passive {
		err = os.NewSyscallError("listen", unix.Listen(fd, listenerBacklogMaxSize))
	} else {
		err = os.NewSyscallError("connect", unix.Connect(fd, addr.Sa))
	}

	return
}

func sysSocket(family, sotype, proto int) (int, error) {
	return unix.Socket(family, sotype|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, proto)
}
