package proc

import (
	"encoding/json"
	"testing"
)

const lxdListJSON = `[
  {"name": "web", "status": "Running", "created_at": "2026-09-01T10:00:00.5Z",
   "config": {"image.description": "Ubuntu 24.04 LTS amd64"},
   "state": {"pid": 4242}},
  {"name": "db", "status": "Stopped", "created_at": "bad",
   "config": {"image.os": "Debian", "image.release": "bookworm"}}
]`

func TestParseLXDLikeList(t *testing.T) {
	got := parseLXDLikeList([]byte(lxdListJSON), "incus")
	if len(got) != 2 {
		t.Fatalf("parsed %d instances, want 2", len(got))
	}
	if m := got[0]; m.Runtime != "incus" || m.ID != "web" || m.Name != "web" || m.Image != "Ubuntu 24.04 LTS amd64" ||
		m.State != "running" || m.Status != "Running" || m.CreatedAt.IsZero() {
		t.Errorf("first instance = %+v", *m)
	}
	if m := got[1]; m.Image != "Debian bookworm" || m.State != "stopped" || !m.CreatedAt.IsZero() {
		t.Errorf("second instance = %+v", *m)
	}
	if parseLXDLikeList([]byte("not json"), "lxd") != nil {
		t.Error("unreadable output parsed")
	}
}

// Instances list interfaces and devices as JSON objects, which Go iterates in
// random order; the lines must still read the same on every run.
func TestFormatLXDLikeNetworksAndMounts(t *testing.T) {
	var networks map[string]lxdLikeNetworkEntry
	if err := json.Unmarshal([]byte(`{
		"lo":   {"addresses": [{"family": "inet", "address": "127.0.0.1", "scope": "local"}]},
		"eth1": {"addresses": [{"family": "inet", "address": "10.1.0.5", "scope": "global"}]},
		"eth0": {"addresses": [
			{"family": "inet", "address": "10.0.0.5", "scope": "global"},
			{"family": "inet6", "address": "fe80::1", "scope": "link"},
			{"family": "inet6", "address": "fd00::5", "scope": "global"}]}
	}`), &networks); err != nil {
		t.Fatal(err)
	}
	devices := map[string]map[string]string{
		"root":  {"type": "disk", "path": "/", "pool": "default"},
		"data":  {"type": "disk", "source": "/srv/data", "path": "/data"},
		"certs": {"type": "disk", "source": "/etc/ssl", "path": "/ssl", "readonly": "true"},
		"eth0":  {"type": "nic", "network": "incusbr0"},
	}
	for i := 0; i < 20; i++ {
		if got, want := formatLXDLikeNetworks(networks), "eth0: 10.0.0.5, eth0: fd00::5, eth1: 10.1.0.5"; got != want {
			t.Fatalf("networks = %q, want %q", got, want)
		}
		if got, want := formatLXDLikeMounts(devices), "certs: /etc/ssl → /ssl (ro), data: /srv/data → /data"; got != want {
			t.Fatalf("mounts = %q, want %q", got, want)
		}
	}
}

func TestParseLXCList(t *testing.T) {
	got := parseLXCList([]byte(`[
		{"name": "a", "state": "RUNNING", "ipv4": "10.0.3.5", "ipv6": "fd00::5"},
		{"name": "b", "state": "STOPPED"},
		{"name": "c", "state": "RUNNING", "ipv6": "fd00::6"}
	]`))
	if len(got) != 3 {
		t.Fatalf("parsed %d containers, want 3", len(got))
	}
	if m := got[0]; m.Runtime != "lxc" || m.Name != "a" || m.State != "running" || m.Networks != "10.0.3.5, fd00::5" {
		t.Errorf("a = %+v", *m)
	}
	if got[1].Networks != "" || got[1].State != "stopped" || got[2].Networks != "fd00::6" {
		t.Errorf("b, c = %+v, %+v", *got[1], *got[2])
	}
	if parseLXCList([]byte("[")) != nil {
		t.Error("unreadable output parsed")
	}
}
