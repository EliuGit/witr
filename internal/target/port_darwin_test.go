//go:build darwin

package target

import (
	"errors"
	"slices"
	"testing"

	"github.com/pranshuparmar/witr/pkg/model"
)

func TestNetstatPortPIDs(t *testing.T) {
	tests := []struct {
		name    string
		socks   []model.OpenPort
		want    []int
		wantErr error
	}{
		{"listeners win over connections", []model.OpenPort{
			{Port: 8080, Protocol: "TCP", State: "LISTEN", PID: 120}, {Port: 8080, Protocol: "TCP", State: "LISTEN", PID: 110},
			{Port: 8080, Protocol: "TCP", State: "ESTABLISHED", PID: 300},
		}, []int{110, 120}, nil},
		{"UDP binds count", []model.OpenPort{{Port: 5353, Protocol: "UDP", PID: 90}}, []int{90}, nil},
		{"connections when nothing listens", []model.OpenPort{
			{Port: 443, Protocol: "TCP", State: "ESTABLISHED", PID: 200}, {Port: 443, Protocol: "TCP", State: "CLOSE_WAIT", PID: 200},
		}, []int{200}, nil},
		{"a listener without an owner", []model.OpenPort{{Port: 445, Protocol: "TCP", State: "LISTEN", PID: 0}}, nil, ErrSocketOwnerUnknown},
	}
	for _, tt := range tests {
		got, err := netstatPortPIDs(tt.socks, tt.socks[0].Port)
		if !slices.Equal(got, tt.want) || !errors.Is(err, tt.wantErr) {
			t.Errorf("%s: %v, %v; want %v, %v", tt.name, got, err, tt.want, tt.wantErr)
		}
	}
	if got, err := netstatPortPIDs([]model.OpenPort{{Port: 80, Protocol: "TCP", State: "LISTEN", PID: 1}}, 81); err == nil || got != nil {
		t.Errorf("nothing on the port: %v, %v; want an error", got, err)
	}
}
