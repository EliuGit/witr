//go:build windows

package proc

import (
	"net"
	"os"
	"testing"
)

// The socket tables are binary, so a wrong row size or byte order shows up as
// garbage ports and addresses. Open real sockets and find them by port, PID,
// address and state — in any Windows display language.
func TestListSocketsFindsOwnSockets(t *testing.T) {
	self := os.Getpid()

	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	tcpPort := ln.Addr().(*net.TCPAddr).Port

	conn, err := net.Dial("tcp4", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	clientPort := conn.LocalAddr().(*net.TCPAddr).Port

	udp, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = udp.Close() }()
	udpPort := udp.LocalAddr().(*net.UDPAddr).Port

	want := []WinSocket{
		{Protocol: "TCP", LocalIP: "127.0.0.1", LocalPort: tcpPort, State: "LISTEN", PID: self},
		{Protocol: "TCP", LocalIP: "127.0.0.1", LocalPort: clientPort, RemoteIP: "127.0.0.1", RemotePort: tcpPort, State: "ESTABLISHED", PID: self},
		{Protocol: "UDP", LocalIP: "127.0.0.1", LocalPort: udpPort, State: "OPEN", PID: self},
	}
	if ln6, err := net.Listen("tcp6", "[::1]:0"); err == nil {
		defer ln6.Close()
		want = append(want, WinSocket{Protocol: "TCP", LocalIP: "::1", LocalPort: ln6.Addr().(*net.TCPAddr).Port, RemoteIP: "::", State: "LISTEN", PID: self})
	}

	socks, err := ListSockets()
	if err != nil {
		t.Fatalf("ListSockets: %v", err)
	}
	for _, w := range want {
		found := false
		for _, s := range socks {
			if s.Protocol == w.Protocol && s.LocalPort == w.LocalPort && s.PID == w.PID && s.State == w.State {
				found = s.LocalIP == w.LocalIP && (w.RemoteIP == "" || (s.RemoteIP == w.RemoteIP && s.RemotePort == w.RemotePort))
				if !found {
					t.Errorf("socket %+v decoded as %+v", w, s)
				}
				break
			}
		}
		if !found {
			t.Errorf("ListSockets missing %+v", w)
		}
	}

	if info := GetSocketStateForPort(tcpPort); info == nil || info.State != "LISTEN" || info.Explanation == "" {
		t.Errorf("GetSocketStateForPort(%d) = %+v, want an explained LISTEN", tcpPort, info)
	}

	own := GetSocketsForPID(self)
	for _, port := range []int{tcpPort, udpPort} {
		found := false
		for _, s := range own {
			found = found || s.Port == port
		}
		if !found {
			t.Errorf("GetSocketsForPID(self) = %+v, missing port %d", own, port)
		}
	}
}
