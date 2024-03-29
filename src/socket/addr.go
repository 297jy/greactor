package socket

import (
	"golang.org/x/sys/unix"
	"greactor/src/buffers"
	"greactor/src/errors"
	"net"
	"strings"
	"sync"
)

type ServerAddr struct {
	Sa      unix.Sockaddr
	NetAddr net.Addr
	Address string
	Network string
	Family  int // 协议族
}

var ipv4AddrPool = sync.Pool{New: func() interface{} {
	bs := new(buffers.ByteBuffer)
	bs.B = make([]byte, 16)
	return bs
}}

func GetIpv4AddrByteBuffer() *buffers.ByteBuffer {
	return ipv4AddrPool.Get().(*buffers.ByteBuffer)
}

func PutIpv4AddrByteBuffer(bb *buffers.ByteBuffer) {
	if bb == nil {
		return
	}
	bb.B = bb.B[:0]
	ipv4AddrPool.Put(bb)
}

func ParseProtoAddr(addr string) (network, address string) {
	network = "tcp"
	address = strings.ToLower(addr)
	if strings.Contains(address, "://") {
		pair := strings.Split(address, "://")
		network = pair[0]
		address = pair[1]
	}
	return
}

func GetTCPSockAddr(proto, addr string) (sa unix.Sockaddr, family int, tcpAddr *net.TCPAddr, ipv6only bool, err error) {
	var tcpVersion string

	tcpAddr, err = net.ResolveTCPAddr(proto, addr)
	if err != nil {
		return
	}

	tcpVersion, err = determineTCPProto(proto, tcpAddr)
	if err != nil {
		return
	}

	switch tcpVersion {
	case "tcp4":
		sa4 := &unix.SockaddrInet4{Port: tcpAddr.Port}

		if tcpAddr.IP != nil {
			if len(tcpAddr.IP) == 16 {
				copy(sa4.Addr[:], tcpAddr.IP[12:16]) // copy last 4 bytes of slice to array
			} else {
				copy(sa4.Addr[:], tcpAddr.IP) // copy all bytes of slice to array
			}
		}

		sa, family = sa4, unix.AF_INET
	case "tcp6":
		ipv6only = true
		fallthrough
	case "tcp":
		sa6 := &unix.SockaddrInet6{Port: tcpAddr.Port}

		if tcpAddr.IP != nil {
			copy(sa6.Addr[:], tcpAddr.IP) // copy all bytes of slice to array
		}

		if tcpAddr.Zone != "" {
			var iface *net.Interface
			iface, err = net.InterfaceByName(tcpAddr.Zone)
			if err != nil {
				return
			}

			sa6.ZoneId = uint32(iface.Index)
		}

		sa, family = sa6, unix.AF_INET6
	default:
		err = errors.ErrUnsupportedProtocol
	}

	return
}

// 根据ip判断具体的tcp协议
func determineTCPProto(proto string, addr *net.TCPAddr) (string, error) {
	// 尝试将ip转为IPV4地址
	if addr.IP.To4() != nil {
		return "tcp4", nil
	}

	// 尝试将ip转为IPV6地址
	if addr.IP.To16() != nil {
		return "tcp6", nil
	}

	switch proto {
	case "tcp", "tcp4", "tcp6":
		return proto, nil
	}

	return "", errors.ErrUnsupportedTCPProtocol
}

func SockaddrToTCPOrUnixAddr(sa unix.Sockaddr) net.Addr {
	switch sa := sa.(type) {
	case *unix.SockaddrInet4:
		ip := sockaddrInet4ToIP(sa)
		return &net.TCPAddr{IP: ip, Port: sa.Port}
	case *unix.SockaddrUnix:
		return &net.UnixAddr{Name: sa.Name, Net: "unix"}
	}
	return nil
}

func sockaddrInet4ToIP(sa *unix.SockaddrInet4) net.IP {
	ip := GetIpv4AddrByteBuffer().B
	// V4InV6Prefix
	ip[10] = 0xff
	ip[11] = 0xff
	copy(ip[12:16], sa.Addr[:])
	return ip
}
