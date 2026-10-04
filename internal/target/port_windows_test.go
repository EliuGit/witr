//go:build windows

package target

import (
	"errors"
	"slices"
	"testing"

	procpkg "github.com/pranshuparmar/witr/internal/proc"
)

func TestPortOwnerPIDs(t *testing.T) {
	tcp := func(local, remote int, state string, pid int) procpkg.WinSocket {
		return procpkg.WinSocket{Protocol: "TCP", LocalPort: local, RemotePort: remote, State: state, PID: pid}
	}
	tests := []struct {
		name    string
		socks   []procpkg.WinSocket
		want    []int
		wantErr error
	}{
		{"listeners win over connections", []procpkg.WinSocket{
			tcp(8080, 0, "LISTEN", 100), tcp(8080, 0, "LISTEN", 100), // IPv4 and IPv6
			tcp(8080, 51000, "ESTABLISHED", 100), tcp(51000, 8080, "ESTABLISHED", 200),
		}, []int{100}, nil},
		{"UDP binds count, owned ones only", []procpkg.WinSocket{
			{Protocol: "UDP", LocalPort: 5353, PID: 0}, {Protocol: "UDP", LocalPort: 5353, PID: 300},
		}, []int{300}, nil},
		{"connections at either end when nothing listens", []procpkg.WinSocket{
			tcp(443, 60000, "ESTABLISHED", 200), tcp(51000, 443, "ESTABLISHED", 210), tcp(51001, 443, "TIME_WAIT", 0),
		}, []int{200, 210}, nil},
		{"a listener without an owner", []procpkg.WinSocket{tcp(445, 0, "LISTEN", 0)}, nil, ErrSocketOwnerUnknown},
	}
	for _, tt := range tests {
		got, err := portOwnerPIDs(tt.socks, tt.socks[0].LocalPort)
		if !slices.Equal(got, tt.want) || !errors.Is(err, tt.wantErr) {
			t.Errorf("%s: %v, %v; want %v, %v", tt.name, got, err, tt.want, tt.wantErr)
		}
	}
	if got, err := portOwnerPIDs([]procpkg.WinSocket{tcp(80, 0, "LISTEN", 4)}, 81); err == nil || got != nil {
		t.Errorf("nothing on the port: %v, %v; want an error", got, err)
	}
}
