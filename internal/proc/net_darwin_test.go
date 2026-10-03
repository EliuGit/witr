//go:build darwin

package proc

import (
	"slices"
	"testing"

	"github.com/pranshuparmar/witr/pkg/model"
)

func TestParseNetstatAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		wantAddr string
		wantPort int
	}{
		// macOS lsof / netstat use dot-separated format by default.
		{name: "ipv4 dot-separated", raw: "127.0.0.1.8080", wantAddr: "127.0.0.1", wantPort: 8080},
		{name: "ipv4 colon-separated", raw: "127.0.0.1:8080", wantAddr: "127.0.0.1", wantPort: 8080},

		// Wildcard means "all interfaces" → 0.0.0.0.
		{name: "wildcard dot", raw: "*.443", wantAddr: "0.0.0.0", wantPort: 443},
		{name: "wildcard colon", raw: "*:443", wantAddr: "0.0.0.0", wantPort: 443},

		// IPv6 forms.
		{name: "ipv6 any-address bracketed dot", raw: "[::].443", wantAddr: "::", wantPort: 443},
		{name: "ipv6 any-address bracketed colon", raw: "[::]:443", wantAddr: "::", wantPort: 443},
		{name: "ipv6 loopback bracketed", raw: "[::1].8080", wantAddr: "::1", wantPort: 8080},
		{name: "ipv6 specific bracketed", raw: "[fe80::1].22", wantAddr: "fe80::1", wantPort: 22},

		// Garbage / malformed.
		{name: "empty", raw: "", wantAddr: "", wantPort: 0},
		{name: "bare star", raw: "*", wantAddr: "", wantPort: 0},
		{name: "unterminated bracket", raw: "[::1.8080", wantAddr: "", wantPort: 0},
		{name: "non-numeric port", raw: "127.0.0.1.abc", wantAddr: "", wantPort: 0},
		{name: "ipv6 with non-numeric port", raw: "[::1].abc", wantAddr: "", wantPort: 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotAddr, gotPort := parseNetstatAddr(tt.raw)
			if gotAddr != tt.wantAddr || gotPort != tt.wantPort {
				t.Errorf("parseNetstatAddr(%q) = (%q, %d), want (%q, %d)",
					tt.raw, gotAddr, gotPort, tt.wantAddr, tt.wantPort)
			}
		})
	}
}

const sampleLsof = `COMMAND     PID   USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
rapportd    512  alice    4u  IPv4 0x1234567890abcdef      0t0  TCP *:49152 (LISTEN)
rapportd    512  alice    5u  IPv6 0x1234567890abcdee      0t0  TCP *:49152 (LISTEN)
Google      700  alice   20u  IPv4 0x1234567890abcdea      0t0  TCP 192.168.1.10:51234->17.57.146.20:443 (ESTABLISHED)
Google      700  alice   21u  IPv6 0x1234567890abcde9      0t0  TCP [2001:db8::10]:51235->[2001:db8::20]:443 (ESTABLISHED)
mDNSResp    300  alice    8u  IPv4 0x1234567890abcde8      0t0  UDP *:5353
`

const sampleNetstat = `Active Internet connections (including servers)
Proto Recv-Q Send-Q  Local Address          Foreign Address        (state)
tcp4       0      0  192.168.1.10.51234     17.57.146.20.443       ESTABLISHED
tcp4       0      0  192.168.1.10.51000     17.1.1.1.443           TIME_WAIT
tcp46      0      0  *.49152                *.*                    LISTEN
tcp6       0      0  *.22                   *.*                    LISTEN
tcp4       0      0  *.22                   *.*                    LISTEN
tcp4       0      0  127.0.0.1.631          *.*                    LISTEN
tcp6       0      0  ::1.631                *.*                    LISTEN
udp4       0      0  *.5353                 *.*
udp4       0      0  *.123                  *.*
udp6       0      0  *.123                  *.*
Active LOCAL (UNIX) domain sockets
Address          Type   Recv-Q Send-Q            Inode             Conn             Refs          Nextref Addr
2bd2e58f8c9a19d3 stream      0      0                0 2bd2e58f8c9a1a9b                0                0
`

// A connected socket's lsof NAME is local->remote; the local side is the port
// the socket holds.
func TestParseLsofPorts(t *testing.T) {
	want := []model.OpenPort{
		{PID: 512, Port: 49152, Address: "0.0.0.0", Protocol: "TCP", State: "LISTEN"},
		{PID: 512, Port: 49152, Address: "0.0.0.0", Protocol: "TCP", State: "LISTEN"},
		{PID: 700, Port: 51234, Address: "192.168.1.10", Protocol: "TCP", State: "ESTABLISHED"},
		{PID: 700, Port: 51235, Address: "2001:db8::10", Protocol: "TCP", State: "ESTABLISHED"},
		{PID: 300, Port: 5353, Address: "0.0.0.0", Protocol: "UDP", State: "OPEN"},
	}
	if got := parseLsofPorts(sampleLsof); !slices.Equal(got, want) {
		t.Errorf("parseLsofPorts =\n%+v\nwant\n%+v", got, want)
	}
}

// Without root, lsof misses other users' sockets; netstat fills them in with
// no owner, once each, skipping sockets lsof already accounts for and
// TIME_WAIT sockets that no process owns.
func TestUnownedPorts(t *testing.T) {
	want := []model.OpenPort{
		{Port: 22, Address: "0.0.0.0", Protocol: "TCP", State: "LISTEN"},
		{Port: 631, Address: "127.0.0.1", Protocol: "TCP", State: "LISTEN"},
		{Port: 631, Address: "::1", Protocol: "TCP", State: "LISTEN"},
		{Port: 123, Address: "0.0.0.0", Protocol: "UDP", State: "OPEN"},
	}
	if got := unownedPorts(sampleNetstat, parseLsofPorts(sampleLsof)); !slices.Equal(got, want) {
		t.Errorf("unownedPorts =\n%+v\nwant\n%+v", got, want)
	}
}
