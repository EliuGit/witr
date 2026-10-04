package source

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pranshuparmar/witr/pkg/model"
)

// isShell reports whether name (a lowercased command basename) is an interactive
// shell or desktop launcher — a signal that a process was started by a user or
// script rather than a service manager. Shared by detectShell and detectInit so
// both agree on the definition.
func isShell(name string) bool {
	switch name {
	case "bash", "zsh", "sh", "fish", "csh", "tcsh", "ksh", "dash", "ash",
		"cmd.exe", "powershell.exe", "pwsh.exe", "explorer.exe":
		return true
	}
	return false
}

var userTools = map[string]bool{
	// Runtimes
	"python":  true,
	"python3": true,
	"node":    true,
	"ruby":    true,
	"perl":    true,
	"php":     true,
	"go":      true,
	"java":    true,
	"cargo":   true,
	"npm":     true,
	"yarn":    true,
	"make":    true,

	// Editors / IDEs
	"code":   true,
	"cursor": true,
	"vim":    true,
	"nvim":   true,
	"emacs":  true,
	"nano":   true,

	// Terminals
	"gnome-terminal-": true,
	"kitty":           true,
	"alacritty":       true,
	"wezterm":         true,
	"konsole":         true,
}

func detectShell(ancestry []model.Process) *model.Source {
	// Scan from the end (target) backwards to find the closest shell OR user tool
	// This ensures we get the direct parent rather than an ancestor
	for i := len(ancestry) - 2; i >= 0; i-- {
		cmd := ancestry[i].Command
		base := filepath.Base(cmd)

		// Windows reports executables with inconsistent casing (e.g.
		// "Explorer.EXE", "PowerShell.exe"), so match shell names
		// case-insensitively.
		if isShell(strings.ToLower(base)) {
			src := &model.Source{
				Type: model.SourceShell,
				Name: base,
			}
			enrichMultiplexer(src, ancestry)
			return src
		}

		// A command run directly in a tmux or screen window, with no shell in
		// between, was started by the multiplexer.
		if name := multiplexer(base); name != "" {
			src := &model.Source{
				Type: model.SourceShell,
				Name: name,
			}
			enrichMultiplexer(src, ancestry)
			return src
		}

		// Normalize for Windows by stripping common executable extensions for the map lookup
		lookupName := base
		lowerBase := strings.ToLower(base)
		for _, ext := range []string{".exe", ".cmd", ".bat", ".com"} {
			if strings.HasSuffix(lowerBase, ext) {
				lookupName = strings.TrimSuffix(lowerBase, ext)
				break
			}
		}

		if userTools[lookupName] {
			src := &model.Source{
				Type: model.SourceShell,
				Name: base,
			}
			enrichMultiplexer(src, ancestry)
			return src
		}

		// Prefix matches for interpreters with versions or paths
		if strings.HasPrefix(base, "python") || strings.HasPrefix(base, "node") {
			src := &model.Source{
				Type: model.SourceShell,
				Name: base,
			}
			enrichMultiplexer(src, ancestry)
			return src
		}
	}
	return nil
}

// multiplexer returns "tmux" or "screen" when base is that multiplexer's
// process name (the tmux server shows as "tmux: server"), or "".
func multiplexer(base string) string {
	switch {
	case base == "tmux" || strings.HasPrefix(base, "tmux:"):
		return "tmux"
	case base == "screen" || strings.HasPrefix(base, "SCREEN"):
		return "screen"
	}
	return ""
}

// enrichMultiplexer checks if tmux or screen is in the ancestry and adds
// session details to the source description.
func enrichMultiplexer(src *model.Source, ancestry []model.Process) {
	for i := 0; i < len(ancestry)-1; i++ {
		base := filepath.Base(ancestry[i].Command)

		switch multiplexer(base) {
		case "tmux":
			desc := "tmux session"
			// TMUX holds the server's socket, its pid and the session id
			// ("/tmp/tmux-1000/default,12345,3"), not the session's name.
			parts := strings.Split(findEnvVar(ancestry, "TMUX"), ",")
			if len(parts) == 3 && parts[0] != "" && parts[2] != "" {
				if name := tmuxSessionName(parts[0], parts[2]); name != "" {
					desc = fmt.Sprintf("tmux session '%s'", name)
				}
			}
			src.Description = desc
			return
		case "screen":
			session := findEnvVar(ancestry, "STY")
			desc := "screen session"
			if session != "" {
				desc = fmt.Sprintf("screen session '%s'", session)
			}
			src.Description = desc
			return
		}
	}
}

// tmuxSessionName asks the tmux server on socket for the name of session id,
// or returns "" when it can't.
var tmuxSessionName = func(socket, id string) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tmux", "-S", socket, "display-message", "-p", "-t", "$"+id, "#{session_name}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// findEnvVar searches the ancestry chain (target first) for an environment variable.
func findEnvVar(ancestry []model.Process, key string) string {
	for i := len(ancestry) - 1; i >= 0; i-- {
		for _, entry := range ancestry[i].Env {
			k, v, ok := strings.Cut(entry, "=")
			if ok && k == key {
				return v
			}
		}
	}
	return ""
}
