package source

import (
	"testing"

	"github.com/pranshuparmar/witr/pkg/model"
)

func TestDetectSSH(t *testing.T) {
	sshd := model.Process{PID: 900, Command: "sshd: alice@pts/3"}
	tests := []struct {
		name     string
		ancestry []model.Process
		want     string // "" for no SSH source
	}{
		{"not under sshd", []model.Process{{PID: 1, Command: "systemd"}, {PID: 5, Command: "bash"}}, ""},
		{"the target itself is sshd", []model.Process{{PID: 1, Command: "systemd"}, {PID: 900, Command: "sshd"}}, ""},
		{"client, tty and user on the target", []model.Process{
			{PID: 1, Command: "systemd"}, sshd,
			{PID: 950, Command: "vim", User: "alice", Env: []string{"SSH_CLIENT=203.0.113.5 50000 22", "SSH_TTY=/dev/pts/3"}},
		}, "SSH session from 203.0.113.5 (alice@pts/3)"},
		{"connection only, no tty", []model.Process{
			{PID: 1, Command: "systemd"}, {PID: 900, Command: "/usr/sbin/sshd"},
			{PID: 950, Command: "rsync", User: "bob", Env: []string{"SSH_CONNECTION=198.51.100.7 40000 10.0.0.2 22"}},
		}, "SSH session from 198.51.100.7 (bob)"},
		{"variables on an ancestor (sudo -i)", []model.Process{
			{PID: 1, Command: "systemd"}, sshd,
			{PID: 920, Command: "bash", Env: []string{"SSH_CLIENT=203.0.113.9 50001 22", "SSH_TTY=/dev/pts/4"}},
			{PID: 930, Command: "sudo"},
			{PID: 950, Command: "top", User: "root"},
		}, "SSH session from 203.0.113.9 (root@pts/4)"},
		{"no user known", []model.Process{
			{PID: 1, Command: "systemd"}, sshd,
			{PID: 950, Command: "x", Env: []string{"SSH_CONNECTION=192.0.2.1 1 192.0.2.2 22"}},
		}, "SSH session from 192.0.2.1"},
		{"no variables in reach", []model.Process{{PID: 1, Command: "systemd"}, sshd, {PID: 950, Command: "x"}}, "SSH session"},
		{"OpenSSH 9.8 session process", []model.Process{{PID: 1, Command: "systemd"}, {PID: 900, Command: "sshd-session"}, {PID: 950, Command: "x"}}, "SSH session"},
		{"a name that only starts with sshd", []model.Process{{PID: 1, Command: "systemd"}, {PID: 900, Command: "sshdump"}, {PID: 950, Command: "x"}}, ""},
		{"Windows OpenSSH", []model.Process{
			{PID: 4, Command: "services.exe"}, {PID: 900, Command: "sshd.exe"},
			{PID: 950, Command: "pwsh.exe", User: `HOST\alice`, Env: []string{"SSH_CLIENT=10.0.0.9 50000 22"}},
		}, `SSH session from 10.0.0.9 (HOST\alice)`},
	}
	for _, tt := range tests {
		src := detectSSH(tt.ancestry)
		switch {
		case tt.want == "" && src != nil:
			t.Errorf("%s: got %+v, want no SSH source", tt.name, *src)
		case tt.want != "" && src == nil:
			t.Errorf("%s: no SSH source, want %q", tt.name, tt.want)
		case src != nil && (src.Type != model.SourceSSH || src.Name != "sshd" || src.Description != tt.want):
			t.Errorf("%s: got %+v, want description %q", tt.name, *src, tt.want)
		}
	}
}
