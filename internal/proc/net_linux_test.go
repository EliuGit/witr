//go:build linux

package proc

import (
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/pranshuparmar/witr/pkg/model"
)

func encodeProcNetTCP6(ip net.IP, port int) string {
	ip16 := ip.To16()
	if ip16 == nil {
		return ""
	}

	// /proc/net/tcp6 stores IPv6 as 4 LE 32-bit groups
	// parseAddr reverses bytes within each 4-byte group to decode
	// so we just inverse the transformation for our tests
	stored := make([]byte, 16)
	for i := 0; i < 4; i++ {
		stored[i*4+0] = ip16[i*4+3]
		stored[i*4+1] = ip16[i*4+2]
		stored[i*4+2] = ip16[i*4+1]
		stored[i*4+3] = ip16[i*4+0]
	}

	return hex.EncodeToString(stored) + ":" + fmt.Sprintf("%04X", port)
}

func TestParseAddr(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		ipv6     bool
		wantAddr string
		wantPort int
	}{
		{
			name:     "IPv4 localhost",
			raw:      "0100007F:0277",
			ipv6:     false,
			wantAddr: "127.0.0.1",
			wantPort: 631,
		},
		{
			name:     "IPv4 all interfaces",
			raw:      "00000000:0050",
			ipv6:     false,
			wantAddr: "0.0.0.0",
			wantPort: 80,
		},
		{
			name:     "IPv6 loopback ::1",
			raw:      "00000000000000000000000001000000:0277",
			ipv6:     true,
			wantAddr: "::1",
			wantPort: 631,
		},
		{
			name:     "IPv6 all interfaces ::",
			raw:      "00000000000000000000000000000000:01BB",
			ipv6:     true,
			wantAddr: "::",
			wantPort: 443,
		},
		{
			name:     "IPv6 link-local fe80::1",
			raw:      encodeProcNetTCP6(net.ParseIP("fe80::1"), 8080),
			ipv6:     true,
			wantAddr: "fe80::1",
			wantPort: 8080,
		},
		// Edge cases
		{
			name:     "Empty input",
			raw:      "",
			ipv6:     false,
			wantAddr: "",
			wantPort: 0,
		},
		{
			name:     "Missing colon separator",
			raw:      "0100007F0277",
			ipv6:     false,
			wantAddr: "",
			wantPort: 0,
		},
		{
			name:     "Invalid hex in IPv4",
			raw:      "ZZZZZZZZ:0050",
			ipv6:     false,
			wantAddr: "",
			wantPort: 80,
		},
		{
			name:     "Invalid hex in IPv6",
			raw:      "ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ:0050",
			ipv6:     true,
			wantAddr: "",
			wantPort: 80,
		},
		{
			name:     "Wrong length IPv6 (too short)",
			raw:      "0000000000000000:0277",
			ipv6:     true,
			wantAddr: "::",
			wantPort: 631,
		},
		{
			name:     "Wrong length IPv4 (too short)",
			raw:      "01007F:0277",
			ipv6:     false,
			wantAddr: "",
			wantPort: 631,
		},
		{
			name:     "Only colon",
			raw:      ":",
			ipv6:     false,
			wantAddr: "",
			wantPort: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAddr, gotPort := parseAddr(tt.raw, tt.ipv6)
			if gotAddr != tt.wantAddr {
				t.Errorf("parseAddr() gotAddr = %v, want %v", gotAddr, tt.wantAddr)
			}
			if gotPort != tt.wantPort {
				t.Errorf("parseAddr() gotPort = %v, want %v", gotPort, tt.wantPort)
			}
		})

	}
}

// An unconnected UDP socket is reported as TCP_CLOSE (07); it must read as
// OPEN like on the other platforms, or the Ports tab hides it by default.
func TestSocketState(t *testing.T) {
	tests := []struct{ proto, hex, want string }{
		{"UDP", "07", "OPEN"},
		{"UDP6", "07", "OPEN"},
		{"UDP", "01", "ESTABLISHED"},
		{"TCP", "07", "CLOSE"},
		{"TCP6", "0A", "LISTEN"},
		{"TCP", "FF", "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := socketState(tt.proto, tt.hex); got != tt.want {
			t.Errorf("socketState(%q, %q) = %q, want %q", tt.proto, tt.hex, got, tt.want)
		}
	}
}

// The uid column names the user owning each socket, even when its process is
// hidden from this user.
func TestParseProcNet(t *testing.T) {
	table := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:1538 00000000:0000 0A 00000000:00000000 00:00000000 00000000   113        0 23417 1 0000000000000000 100 0 0 10 0
   1: 0100007F:B53C 0100007F:1538 01 00000000:00000000 00:00000000 00000000  1000        0 99001 1 0000000000000000 20 4 30 10 -1
   2: short line
`
	sockets := map[string]model.Socket{}
	uids := map[string]int{}
	parseProcNet(strings.NewReader(table), "TCP", false, sockets, uids)
	if len(sockets) != 2 {
		t.Fatalf("parsed %d sockets, want 2", len(sockets))
	}
	if s := sockets["23417"]; s.Port != 5432 || s.Address != "127.0.0.1" || s.State != "LISTEN" || uids["23417"] != 113 {
		t.Errorf("listener = %+v, uid %d", s, uids["23417"])
	}
	if s := sockets["99001"]; s.RemotePort != 5432 || s.State != "ESTABLISHED" || uids["99001"] != 1000 {
		t.Errorf("connection = %+v, uid %d", s, uids["99001"])
	}
}

func TestUserName(t *testing.T) {
	if got := UserName(0); got != "root" {
		t.Errorf("UserName(0) = %q, want root", got)
	}
	if got := UserName(987654321); got != "987654321" {
		t.Errorf("an unlisted uid = %q, want the number", got)
	}
}
