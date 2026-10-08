//go:build windows

package proc

import "testing"

// A shared host is credited to DcomLaunch, which starts COM servers and
// packaged apps; otherwise to every service it runs.
func TestServiceLabel(t *testing.T) {
	tests := []struct {
		names []string
		want  string
	}{
		{[]string{"Dnscache"}, "Dnscache"},
		{[]string{"BrokerInfrastructure", "DcomLaunch", "PlugPlay", "Power", "SystemEventsBroker"}, "DcomLaunch"},
		{[]string{"RpcSs", "RpcEptMapper"}, "RpcEptMapper, RpcSs"},
	}
	for _, tt := range tests {
		if got := serviceLabel(tt.names); got != tt.want {
			t.Errorf("serviceLabel(%v) = %q, want %q", tt.names, got, tt.want)
		}
	}
}
