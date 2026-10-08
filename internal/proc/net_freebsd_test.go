//go:build freebsd

package proc

import (
	"testing"

	"github.com/pranshuparmar/witr/pkg/model"
)

func TestParseSockstatAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		proto    string
		wantAddr string
		wantPort int
	}{
		// IPv4 happy paths.
		{name: "ipv4 specific", raw: "127.0.0.1:8080", proto: "tcp4", wantAddr: "127.0.0.1", wantPort: 8080},
		{name: "ipv4 wildcard colon", raw: "*:443", proto: "tcp4", wantAddr: "0.0.0.0", wantPort: 443},
		{name: "ipv4 wildcard via '*' ip", raw: "*:53", proto: "udp4", wantAddr: "0.0.0.0", wantPort: 53},

		// IPv6 happy paths — protocol disambiguates wildcards.
		{name: "ipv6 wildcard via tcp6 proto", raw: "*:443", proto: "tcp6", wantAddr: "::", wantPort: 443},
		{name: "ipv6 bracketed loopback", raw: "[::1]:8080", proto: "tcp6", wantAddr: "::1", wantPort: 8080},
		{name: "ipv6 bracketed link-local", raw: "[fe80::1]:22", proto: "tcp6", wantAddr: "fe80::1", wantPort: 22},

		// Dot-separated fallback (older FreeBSD output).
		{name: "ipv4 dot-separated", raw: "127.0.0.1.8080", proto: "tcp4", wantAddr: "127.0.0.1", wantPort: 8080},

		// Garbage / malformed.
		{name: "empty", raw: "", proto: "tcp4", wantAddr: "", wantPort: 0},
		{name: "unterminated bracket", raw: "[::1:8080", proto: "tcp6", wantAddr: "", wantPort: 0},
		{name: "ipv6 missing port separator", raw: "[::1]8080", proto: "tcp6", wantAddr: "", wantPort: 0},
		{name: "non-numeric port", raw: "127.0.0.1:abc", proto: "tcp4", wantAddr: "", wantPort: 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotAddr, gotPort := parseSockstatAddr(tt.raw, tt.proto)
			if gotAddr != tt.wantAddr || gotPort != tt.wantPort {
				t.Errorf("parseSockstatAddr(%q, %q) = (%q, %d), want (%q, %d)",
					tt.raw, tt.proto, gotAddr, gotPort, tt.wantAddr, tt.wantPort)
			}
		})
	}
}

// Connections accepted on one port stay separate and keep their remote end;
// a listener has none.
func TestParseSockstatOutputRemote(t *testing.T) {
	out := `USER     COMMAND    PID   FD PROTO  LOCAL ADDRESS         FOREIGN ADDRESS
www      nginx      812   6  tcp4   *:80                  *:*
www      nginx      812   7  tcp4   10.0.0.5:80           10.0.0.9:51000
www      nginx      812   8  tcp4   10.0.0.5:80           10.0.0.9:51002
www      nginx      812   9  tcp4   10.0.0.5:41000        10.0.0.20:5432
`
	sockets := map[string]model.Socket{}
	parseSockstatOutput(out, sockets)
	if len(sockets) != 4 {
		t.Fatalf("parsed %d sockets, want 4: %+v", len(sockets), sockets)
	}
	var accepted, outbound int
	for _, s := range sockets {
		switch {
		case s.State == "LISTEN" && s.RemoteAddress != "":
			t.Errorf("listener has a remote end: %+v", s)
		case s.Port == 80 && s.State == "ESTABLISHED" && s.RemoteAddress == "10.0.0.9":
			accepted++
		case s.Port == 41000 && s.RemoteAddress == "10.0.0.20" && s.RemotePort == 5432:
			outbound++
		}
	}
	if accepted != 2 || outbound != 1 {
		t.Errorf("accepted=%d outbound=%d, want 2 and 1", accepted, outbound)
	}
}
