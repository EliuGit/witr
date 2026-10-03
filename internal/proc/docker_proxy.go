package proc

import (
	"path/filepath"
	"strconv"
	"strings"
)

// resolveDockerProxyContainer describes the container a docker-proxy process
// forwards to. The container is found by the host port the proxy publishes,
// which works on any Docker network, including the per-project networks
// Compose creates.
func resolveDockerProxyContainer(cmdline string) string {
	port, ok := dockerProxyHostPort(cmdline)
	if !ok {
		return ""
	}
	c := ResolveContainerByPort(port)
	if c == nil {
		return ""
	}
	return "forwards to docker: " + c.Name + " (id " + shortID(c.ID) + ")"
}

// IsDockerProxyFor reports whether pid is a docker-proxy publishing port.
func IsDockerProxyFor(pid, port int) bool {
	cmdline := GetCmdline(pid)
	fields := strings.Fields(cmdline)
	if len(fields) == 0 || filepath.Base(fields[0]) != "docker-proxy" {
		return false
	}
	p, ok := dockerProxyHostPort(cmdline)
	return ok && p == port
}

// dockerProxyHostPort returns the -host-port argument of a docker-proxy
// command line.
func dockerProxyHostPort(cmdline string) (int, bool) {
	parts := strings.Fields(cmdline)
	for i, part := range parts {
		if part == "-host-port" && i+1 < len(parts) {
			port, err := strconv.Atoi(parts[i+1])
			return port, err == nil && port > 0
		}
	}
	return 0, false
}
