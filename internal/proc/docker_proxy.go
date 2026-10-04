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

// portForwarders are processes that hold host ports for containers, each
// serving every port it publishes, so only the runtime can say which
// container a port belongs to: Docker Desktop's backend on Windows and macOS
// (older releases: vpnkit and com.docker.proxy) and, on Windows, the relay
// for a WSL2 VM's ports; rootless Podman's rootlessport and pasta; and
// RootlessKit, behind rootless nerdctl and Docker's rootless mode.
var portForwarders = map[string]bool{
	"com.docker.backend.exe": true,
	"com.docker.proxy.exe":   true,
	"vpnkit.exe":             true,
	"wslrelay.exe":           true,
	"com.docker.backend":     true,
	"com.docker.vpnkit":      true,
	"vpnkit-bridge":          true,
	"rootlessport":           true,
	"rootlesskit":            true,
	"pasta":                  true,
}

// isPortForwarder reports whether a process name is one of portForwarders;
// pasta also runs as a CPU-specific build such as pasta.avx2.
func isPortForwarder(name string) bool {
	name = strings.ToLower(name)
	return portForwarders[name] || strings.HasPrefix(name, "pasta.")
}

// How a process is read and a published port's container found; variables so
// tests can stand in for live processes and runtimes.
var (
	cmdlineOf       = GetCmdline
	imageNameOf     = imageName
	containerByPort = ResolveContainerByPort
)

// PublishedContainer returns the container behind port when every process in
// pids only publishes container ports on the host (docker-proxy, or one of
// portForwarders), with each process's name. It returns nil when any of them
// is an ordinary listener, or when no container publishes the port: the WSL
// relay, for one, also forwards ports of plain WSL servers.
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
		name := imageNameOf(pid)
		if !isPortForwarder(name) {
			return nil, nil
		}
		names[i] = name
	}
	// Forwarders don't say which protocol they carry, so proto stays "" (either)
	// unless a docker-proxy named it.
	match := containerByPort(port, proto)
	if match == nil {
		return nil, nil
	}
	return match, names
}

// DockerProxyProto reports whether pid is a docker-proxy publishing port, and
// over which protocol.
func DockerProxyProto(pid, port int) (string, bool) {
	cmdline := cmdlineOf(pid)
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
