package sockd

import (
	"github.com/HouzuoGuo/laitos/daemon/dnsd"
	"net"
	"strings"
	"testing"
)

func TestSockd_StartAndBlock(t *testing.T) {
	daemon := Daemon{}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "dns daemon") == -1 {
		t.Fatal(err)
	}
	daemon.DNSDaemon = &dnsd.Daemon{}
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "listen port") == -1 {
		t.Fatal(err)
	}
	daemon.TCPPort = 27101
	if err := daemon.Initialise(); err == nil || strings.Index(err.Error(), "password") == -1 {
		t.Fatal(err)
	}
	daemon.Password = "abcdefg"
	if err := daemon.Initialise(); err != nil || daemon.Address != "0.0.0.0" || daemon.PerIPLimit != 96 {
		t.Fatal(err)
	}

	daemon.Address = "127.0.0.1"
	daemon.TCPPort = 27101
	daemon.UDPPort = 13781
	daemon.Password = "abcdefg"
	daemon.PerIPLimit = 10

	TestSockd(&daemon, t)
}

func TestIsReservedAddr(t *testing.T) {
	notReserved := []net.IP{
		net.IPv4(8, 8, 8, 8),
		net.IPv4(193, 0, 0, 1),
		net.IPv4(1, 1, 1, 1),
		net.IPv4(54, 0, 0, 0),
	}
	for _, addr := range notReserved {
		if IsReservedAddr(addr) {
			t.Fatal(addr.String())
		}
	}

	reserved := []net.IP{
		net.IPv4(10, 0, 0, 1),
		net.IPv4(100, 64, 0, 1),
		net.IPv4(127, 0, 0, 1),
		net.IPv4(169, 254, 0, 1),
		net.IPv4(172, 16, 0, 1),
		net.IPv4(192, 0, 0, 1),
		net.IPv4(192, 0, 2, 1),
		net.IPv4(192, 168, 0, 1),
		net.IPv4(198, 18, 0, 1),
		net.IPv4(198, 51, 100, 1),
		net.IPv4(203, 0, 113, 1),
		net.IPv4(240, 0, 0, 1),
		net.IPv4(240, 0, 0, 95),
	}
	for _, addr := range reserved {
		if !IsReservedAddr(addr) {
			t.Fatal(addr.String())
		}
	}
}
