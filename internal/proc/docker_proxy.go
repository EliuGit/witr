package proc

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pranshuparmar/witr/pkg/model"
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

// dockerDesktopForwarders are the processes Docker Desktop publishes container
// ports with on Windows and macOS: its backend (older releases: vpnkit and
// com.docker.proxy) and, on Windows, the relay that forwards a WSL2 VM's
// ports. Each serves every published port, so only the runtime can say which
// container a port belongs to.
var dockerDesktopForwarders = map[string]bool{
	"com.docker.backend.exe": true,
	"com.docker.proxy.exe":   true,
	"vpnkit.exe":             true,
	"wslrelay.exe":           true,
	"com.docker.backend":     true,
	"com.docker.vpnkit":      true,
	"vpnkit-bridge":          true,
}

// PublishedContainer returns the container behind port when every process in
// pids only publishes container ports on the host (docker-proxy, or Docker
// Desktop's forwarders), with each process's name. It returns nil when any of
// them is an ordinary listener, or when no container publishes the port: the
// WSL relay also forwards ports of plain WSL servers.
func PublishedContainer(port int, pids []int) (*model.ContainerMatch, []string) {
	if len(pids) == 0 {
		return nil, nil
	}
	names := make([]string, len(pids))
	proto := ""
	for i, pid := range pids {
		if p, ok := DockerProxyProto(pid, port); ok {
			names[i], proto = "docker-proxy", p
			continue
		}
		name := imageName(pid)
		if !dockerDesktopForwarders[strings.ToLower(name)] {
			return nil, nil
		}
		names[i] = name
	}
	var match *model.ContainerMatch
	if proto != "" {
		match = ResolveContainerByPort(port, proto)
	} else {
		// Docker Desktop's forwarders don't say which protocol they carry.
		if match = ResolveContainerByPort(port, "tcp"); match == nil {
			match = ResolveContainerByPort(port, "udp")
		}
	}
	if match == nil {
		return nil, nil
	}
	return match, names
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
