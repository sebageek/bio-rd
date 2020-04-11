package server

import (
	"net"
	"os"
	"syscall"
)

const (
	TCP_MD5SIG       = 14 // TCP MD5 Signature (RFC2385)
	IPV6_MINHOPCOUNT = 73 // Generalized TTL Security Mechanism (RFC5082)
)

// SetListenTCPTTLSockopt sets the TTL on a TCP listener
func SetListenTCPTTLSockopt(l *net.TCPListener, ttl uint8) error {
	fi, family, err := extractFileAndFamilyFromTCPListener(l)
	defer fi.Close()
	if err != nil {
		return err
	}
	return setsockoptIPTTL(int(fi.Fd()), family, ttl)
}

// SetTCPConnTTLSockopt sets the TTL on a TCP connection
func SetTCPConnTTLSockopt(c net.Conn, ttl uint8) error {
	switch c.(type) {
	case *net.TCPConn:
	default:
		return nil
	}

	fi, family, err := extractFileAndFamilyFromTCPConn(c.(*net.TCPConn))
	defer fi.Close()
	if err != nil {
		return err
	}

	return setsockoptIPTTL(int(fi.Fd()), family, ttl)
}

func setsockoptIPTTL(fd int, family int, value uint8) error {
	level := syscall.IPPROTO_IP
	name := syscall.IP_TTL
	if family == syscall.AF_INET6 {
		level = syscall.IPPROTO_IPV6
		name = syscall.IPV6_UNICAST_HOPS
	}

	return os.NewSyscallError("setsockopt", syscall.SetsockoptInt(fd, level, name, int(value)))
}
