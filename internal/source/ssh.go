package source

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pranshuparmar/witr/pkg/model"
)

// isSSHDaemon reports whether command names the SSH server: its listener, or
// the per-connection process, whose title reads "sshd: alice@pts/3" (on
// OpenSSH 9.8 and later, sshd-session).
func isSSHDaemon(command string) bool {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	switch strings.ToLower(strings.TrimSuffix(filepath.Base(fields[0]), ":")) {
	case "sshd", "sshd.exe", "sshd-session":
		return true
	}
	return false
}

func detectSSH(ancestry []model.Process) *model.Source {
	if len(ancestry) < 2 {
		return nil
	}

	// Look for sshd in the ancestry chain (excluding the target itself)
	hasSSHD := false
	for i := 0; i < len(ancestry)-1; i++ {
		if isSSHDaemon(ancestry[i].Command) {
			hasSSHD = true
			break
		}
	}
	if !hasSSHD {
		return nil
	}

	// Extract SSH connection details from environment variables.
	// Check all processes in the chain (target first, then ancestors) because
	// su/sudo create clean login shells that don't inherit SSH_* vars.
	target := ancestry[len(ancestry)-1]
	var remoteIP, tty string

	for i := len(ancestry) - 1; i >= 0 && remoteIP == ""; i-- {
		for _, entry := range ancestry[i].Env {
			key, val, ok := strings.Cut(entry, "=")
			if !ok {
				continue
			}
			switch key {
			case "SSH_CLIENT":
				if fields := strings.Fields(val); len(fields) >= 1 {
					remoteIP = fields[0]
				}
			case "SSH_CONNECTION":
				if remoteIP == "" {
					if fields := strings.Fields(val); len(fields) >= 1 {
						remoteIP = fields[0]
					}
				}
			case "SSH_TTY":
				if tty == "" {
					tty = val
				}
			}
		}
	}

	desc := "SSH session"
	if remoteIP != "" {
		if tty != "" {
			desc = fmt.Sprintf("SSH session from %s (%s@%s)", remoteIP, target.User, strings.TrimPrefix(tty, "/dev/"))
		} else if target.User != "" {
			desc = fmt.Sprintf("SSH session from %s (%s)", remoteIP, target.User)
		} else {
			desc = fmt.Sprintf("SSH session from %s", remoteIP)
		}
	}

	return &model.Source{
		Type:        model.SourceSSH,
		Name:        "sshd",
		Description: desc,
	}
}
