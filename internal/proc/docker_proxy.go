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
	port, proto, ok := dockerProxyPublish(cmdline)
	if !ok {
		return ""
	}
	c := ResolveContainerByPort(port, proto)
	if c == nil {
		return ""
	}
	return "forwards to docker: " + c.Name + " (id " + shortID(c.ID) + ")"
}

// DockerProxyProto reports whether pid is a docker-proxy publishing port, and
// over which protocol.
func DockerProxyProto(pid, port int) (string, bool) {
	cmdline := GetCmdline(pid)
	fields := strings.Fields(cmdline)
	if len(fields) == 0 || filepath.Base(fields[0]) != "docker-proxy" {
		return "", false
	}
	p, proto, ok := dockerProxyPublish(cmdline)
	return proto, ok && p == port
}

// dockerProxyPublish returns the -host-port and -proto arguments of a
// docker-proxy command line; the protocol defaults to tcp.
func dockerProxyPublish(cmdline string) (port int, proto string, ok bool) {
	proto = "tcp"
	parts := strings.Fields(cmdline)
	for i := 0; i+1 < len(parts); i++ {
		switch parts[i] {
		case "-host-port":
			if n, err := strconv.Atoi(parts[i+1]); err == nil && n > 0 {
				port, ok = n, true
			}
		case "-proto":
			proto = parts[i+1]
		}
	}
	return port, proto, ok
}
