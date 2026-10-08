package source

import (
	"testing"

	"github.com/pranshuparmar/witr/pkg/model"
)

// The tmux session is named by asking its server; TMUX itself only holds the
// socket path, server pid and session id.
func TestEnrichMultiplexer(t *testing.T) {
	orig := tmuxSessionName
	defer func() { tmuxSessionName = orig }()
	var asked []string
	tmuxSessionName = func(socket, id string) string {
		asked = append(asked, socket, id)
		if id == "3" {
			return "work"
		}
		return ""
	}

	chain := func(mux string, env ...string) []model.Process {
		return []model.Process{
			{PID: 1, Command: "systemd"},
			{PID: 10, Command: mux},
			{PID: 11, Command: "bash"},
			{PID: 12, Command: "node", Env: env},
		}
	}
	tests := []struct {
		name     string
		ancestry []model.Process
		want     string
	}{
		{"named session", chain("tmux: server", "TMUX=/tmp/tmux-1000/default,4242,3"), "tmux session 'work'"},
		{"server can't answer", chain("tmux: server", "TMUX=/tmp/tmux-1000/default,4242,7"), "tmux session"},
		{"no TMUX in reach", chain("tmux: server"), "tmux session"},
		{"malformed TMUX", chain("tmux", "TMUX=garbage"), "tmux session"},
		{"screen", chain("SCREEN", "STY=4321.pts-0.host"), "screen session '4321.pts-0.host'"},
		{"screen without STY", chain("screen"), "screen session"},
		{"no multiplexer", chain("sshd"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := &model.Source{Type: model.SourceShell, Name: "bash"}
			enrichMultiplexer(src, tt.ancestry)
			if src.Description != tt.want {
				t.Errorf("Description = %q, want %q", src.Description, tt.want)
			}
		})
	}
	if len(asked) < 2 || asked[0] != "/tmp/tmux-1000/default" || asked[1] != "3" {
		t.Errorf("tmux asked with %q, want the socket path and session id", asked)
	}
}
