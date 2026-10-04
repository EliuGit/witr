package proc

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"unicode"

	"github.com/pranshuparmar/witr/pkg/model"
)

// ResolveContainerByPort queries the Docker CLI for a container publishing
// the given port over proto ("" or "tcp" for TCP). Returns nil if Docker is
// unavailable or no container matches.
func ResolveContainerByPort(port int, proto string) *model.ContainerMatch {
	filter := fmt.Sprintf("publish=%d", port)
	if proto != "" && proto != "tcp" {
		filter += "/" + proto
	}
	if ms := dockerLikeList("docker", "docker", "--filter", filter); len(ms) > 0 {
		return ms[0]
	}
	return nil
}

// ContainerByID returns the Docker, Podman or nerdctl container with the given
// ID, or nil when the runtime can't be queried or doesn't know it.
func ContainerByID(id, runtime string) *model.ContainerMatch {
	label, ok := dockerLikeRuntimeLabels[runtime]
	if !ok || !isValidContainerID(id) {
		return nil
	}
	if ms := dockerLikeList(runtime, label, "--filter", "id="+id); len(ms) == 1 {
		return ms[0]
	}
	return nil
}

// dockerLikeRuntimeLabels maps each docker-compatible CLI to the runtime name
// its containers are reported under.
var dockerLikeRuntimeLabels = map[string]string{
	"docker":  "docker",
	"podman":  "podman",
	"nerdctl": "containerd",
}

// ContainerDetails returns the container a process runs in and whether it has
// a healthcheck: "present", "absent", or "" when that can't be told. known, if
// set, is the container already looked up, sparing another runtime query.
func ContainerDetails(id, runtime string, known *model.ContainerMatch) (*model.ContainerMatch, string) {
	c := known
	if c == nil {
		// The list scan has no restart count, policy or start time.
		if c = ContainerByID(id, runtime); c != nil {
			EnrichContainer(c)
		}
	}
	switch {
	case c == nil:
		return nil, ContainerHealthcheckStatus(id, runtime)
	case runtime != "docker" && runtime != "podman":
		return c, ""
	case c.Health != "":
		// `ps` reports a health state exactly when a healthcheck is set.
		return c, "present"
	}
	return c, "absent"
}

// isValidContainerID reports whether id is a safe container identifier to hand
// to a runtime CLI: a non-empty token of [A-Za-z0-9_.-] that begins with an
// alphanumeric. Rejecting a leading dash (and any other metacharacter) keeps a
// malformed, cgroup-derived id from being parsed as a CLI option.
func isValidContainerID(id string) bool {
	if id == "" {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case (c == '_' || c == '.' || c == '-') && i > 0:
		default:
			return false
		}
	}
	return true
}

// resolveContainerName attempts to resolve a container ID to a name using the specified runtime CLI.
func resolveContainerName(id, runtime string) string {
	if !isValidContainerID(id) {
		return ""
	}
	var cmd *exec.Cmd
	var prefix string

	ctx := context.Background()
	switch runtime {
	case "docker":
		if _, err := exec.LookPath("docker"); err != nil {
			return ""
		}
		cmd = exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Name}}|{{index .Config.Labels \"com.docker.compose.project\"}}|{{index .Config.Labels \"com.docker.compose.service\"}}", "--", id)
		prefix = "docker: "
	case "podman":
		if _, err := exec.LookPath("podman"); err != nil {
			return ""
		}
		cmd = commandAsOriginalUser(ctx, "podman", "inspect", "--format", "{{.Name}}", "--", id)
		prefix = "podman: "
	case "crictl":
		if _, err := exec.LookPath("crictl"); err != nil {
			return ""
		}
		cmd = exec.CommandContext(ctx, "crictl", "inspect", id, "-o", "go-template", "--template", "{{.status.metadata.name}}")
		prefix = "" // crictl names are usually clean
	case "nerdctl":
		if _, err := exec.LookPath("nerdctl"); err != nil {
			return ""
		}
		cmd = commandAsOriginalUser(ctx, "nerdctl", "inspect", id, "--format", "{{.Name}}")
		prefix = "containerd: "
	default:
		return ""
	}

	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	output := strings.TrimSpace(string(out))

	if runtime == "docker" {
		parts := strings.Split(output, "|")
		if len(parts) == 3 {
			name := strings.TrimPrefix(parts[0], "/")
			project := parts[1]
			service := parts[2]

			if project != "" && service != "" {
				return "docker: " + project + "/" + service + " (" + name + ")"
			}
			if name != "" {
				return "docker: " + name
			}
			return ""
		}
	}

	name := strings.TrimPrefix(output, "/")
	if name != "" {
		if prefix != "" {
			return prefix + name
		}
		return name
	}
	return ""
}

// ContainerHealthcheckStatus reports whether the container runtime has a
// healthcheck configured: "present", "absent", or "" when undeterminable
// (runtime unavailable, inspect error, or unsupported runtime).
func ContainerHealthcheckStatus(id, runtime string) string {
	if !isValidContainerID(id) || (runtime != "docker" && runtime != "podman") {
		return ""
	}
	if _, err := exec.LookPath(runtime); err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), runtimeQueryTimeout)
	defer cancel()
	out, err := runtimeCommand(ctx, runtime, "inspect", "--format", "{{if .Config.Healthcheck}}present{{else}}absent{{end}}", "--", id).Output()
	if err != nil {
		return ""
	}
	switch strings.TrimSpace(string(out)) {
	case "present":
		return "present"
	case "absent":
		return "absent"
	}
	return ""
}

// findLongHexID searches for a 64-character hexadecimal string in the input.
func findLongHexID(s string) string {
	for i := 0; i <= len(s)-64; i++ {
		if s[i] < '0' || (s[i] > '9' && s[i] < 'a') {
			continue
		}
		sub := s[i : i+64]
		isHex := true
		for j := 0; j < 64; j++ {
			c := sub[j]
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				isHex = false
				break
			}
		}
		if isHex {
			return sub
		}
	}
	return ""
}

// shortID returns the first 12 characters of a container ID, or the full
// string if it is shorter than 12 characters.
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// extractFlagValue extracts the value of a specific flag from a command line string.
func extractFlagValue(cmdline string, flags ...string) string {
	args := splitCmdline(cmdline)
	for i, arg := range args {
		for _, flag := range flags {
			if arg == flag && i+1 < len(args) {
				return args[i+1]
			}
		}
	}
	return ""
}

// splitCmdline splits a command line string into arguments, handling quotes and escapes.
func splitCmdline(cmdline string) []string {
	var args []string
	var current strings.Builder
	var quote rune
	escaped := false
	for _, r := range cmdline {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '"' || r == '\'':
			if quote == 0 {
				quote = r
				continue
			}
			if quote == r {
				quote = 0
				continue
			}
			current.WriteRune(r)
		case unicode.IsSpace(r) && quote == 0:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}
