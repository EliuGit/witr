//go:build darwin

package proc

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/pranshuparmar/witr/pkg/model"
)

func ListOpenPorts() ([]model.OpenPort, error) {
	// lsof exits 1 when it has nothing to list (e.g. a user with no sockets);
	// its output is still valid.
	out, err := exec.Command("lsof", "-i", "-P", "-n").Output()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return nil, err
	}
	ports := parseLsofPorts(string(out))

	// Without root, lsof only sees this user's processes. netstat sees every
	// socket but not its owner, so list the rest with no owner (PID 0).
	if os.Geteuid() != 0 {
		if out, err := exec.Command("netstat", "-an").Output(); err == nil {
			ports = append(ports, unownedPorts(string(out), ports)...)
		}
	}
	return ports, nil
}

// parseLsofPorts parses `lsof -i -P -n` output.
func parseLsofPorts(out string) []model.OpenPort {
	var ports []model.OpenPort
	lines := strings.Split(out, "\n")

	startIdx := 0
	if len(lines) > 0 && strings.HasPrefix(lines[0], "COMMAND") {
		startIdx = 1
	}

	for _, line := range lines[startIdx:] {
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		pidStr := fields[1]
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		protocol := fields[7]
		if protocol != "TCP" && protocol != "UDP" {
			if strings.Contains(line, "TCP") {
				protocol = "TCP"
			} else if strings.Contains(line, "UDP") {
				protocol = "UDP"
			} else {
				protocol = "UNKNOWN"
			}
		}

		// Address:Port, or local->remote for a connected socket.
		nameField, _, _ := strings.Cut(fields[8], "->")
		state := "UNKNOWN"
		if len(fields) > 9 {
			state = strings.Trim(fields[9], "()")
		} else if protocol == "UDP" {
			state = "OPEN"
		}

		addr, port := parseNetstatAddr(nameField)
		if port == 0 {
			lastColon := strings.LastIndex(nameField, ":")
			if lastColon != -1 {
				portStr := nameField[lastColon+1:]
				if p, err := strconv.Atoi(portStr); err == nil {
					port = p
					addr = nameField[:lastColon]
					if addr == "*" {
						addr = "0.0.0.0"
					}
				}
			}
		}

		if port > 0 {
			ports = append(ports, model.OpenPort{
				PID:      pid,
				Port:     port,
				Address:  addr,
				Protocol: protocol,
				State:    state,
			})
		}
	}

	return ports
}

// unownedPorts returns the sockets in `netstat -an` output that no owned port
// accounts for, with no owner (PID 0). TIME_WAIT sockets belong to no process
// and are left out, as on Linux.
func unownedPorts(netstatOut string, owned []model.OpenPort) []model.OpenPort {
	key := func(p model.OpenPort) string {
		return fmt.Sprintf("%s|%s|%d|%s", p.Protocol, p.Address, p.Port, p.State)
	}
	seen := make(map[string]bool, len(owned))
	for _, p := range owned {
		seen[key(p)] = true
	}

	var ports []model.OpenPort
	for line := range strings.Lines(netstatOut) {
		// Proto Recv-Q Send-Q Local Foreign (state)
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		var p model.OpenPort
		switch {
		case strings.HasPrefix(fields[0], "tcp"):
			if len(fields) < 6 || fields[5] == "TIME_WAIT" {
				continue
			}
			p.Protocol, p.State = "TCP", fields[5]
		case strings.HasPrefix(fields[0], "udp"):
			p.Protocol, p.State = "UDP", "OPEN"
		default:
			continue
		}
		p.Address, p.Port = parseNetstatAddr(fields[3])
		if p.Port == 0 || seen[key(p)] {
			continue
		}
		seen[key(p)] = true
		ports = append(ports, p)
	}
	return ports
}

// parseNetstatAddr parses addresses like "*.8080", "127.0.0.1.8080", "[::1].8080"
func parseNetstatAddr(addr string) (string, int) {
	// Handle IPv6 format [::]:port or [::1]:port
	if strings.HasPrefix(addr, "[") {
		// IPv6 format
		bracketEnd := strings.LastIndex(addr, "]")
		if bracketEnd == -1 {
			return "", 0
		}
		ip := addr[1:bracketEnd]
		rest := addr[bracketEnd+1:]
		// rest should be ":port" or ".port"
		if len(rest) > 1 && (rest[0] == ':' || rest[0] == '.') {
			port, err := strconv.Atoi(rest[1:])
			if err == nil {
				if ip == "::" || ip == "" {
					return "::", port
				}
				return ip, port
			}
		}
		return "", 0
	}

	// Handle formats like "*:8080" or "*.8080"
	if strings.HasPrefix(addr, "*") {
		if len(addr) > 1 && (addr[1] == ':' || addr[1] == '.') {
			port, err := strconv.Atoi(addr[2:])
			if err == nil {
				return "0.0.0.0", port
			}
		}
		return "", 0
	}

	// Handle IPv4 format: "127.0.0.1:8080" or "127.0.0.1.8080"
	// Try colon-separated first (standard format)
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		ip := addr[:idx]
		portStr := addr[idx+1:]
		port, err := strconv.Atoi(portStr)
		if err == nil {
			return ip, port
		}
	}

	// macOS netstat uses dot-separated: "127.0.0.1.8080"
	// Find the last dot and check if what follows is a port
	if idx := strings.LastIndex(addr, "."); idx != -1 {
		portStr := addr[idx+1:]
		port, err := strconv.Atoi(portStr)
		if err == nil {
			ip := addr[:idx]
			return ip, port
		}
	}

	return "", 0
}
