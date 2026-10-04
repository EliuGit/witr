//go:build darwin

package target

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	procpkg "github.com/pranshuparmar/witr/internal/proc"
)

func ResolvePort(port int) ([]int, error) {
	pidSet := make(map[int]bool)

	// Query TCP listeners: lsof -i TCP:<port> -s TCP:LISTEN -n -P -t
	if out, err := exec.Command("lsof", "-i", fmt.Sprintf("TCP:%d", port), "-s", "TCP:LISTEN", "-n", "-P", "-t").Output(); err == nil {
		for _, pidStr := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if pid, err := strconv.Atoi(strings.TrimSpace(pidStr)); err == nil && pid > 0 {
				pidSet[pid] = true
			}
		}
	}

	// Query UDP bound sockets: lsof -i UDP:<port> -n -P -t
	// UDP is connectionless so there is no LISTEN state to filter on.
	if out, err := exec.Command("lsof", "-i", fmt.Sprintf("UDP:%d", port), "-n", "-P", "-t").Output(); err == nil {
		for _, pidStr := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if pid, err := strconv.Atoi(strings.TrimSpace(pidStr)); err == nil && pid > 0 {
				pidSet[pid] = true
			}
		}
	}

	if len(pidSet) == 0 {
		// Fallback: include processes with connected (non-listening) sockets
		// on this port — `lsof -i :port` matches either side of the connection.
		if out, err := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-n", "-P", "-t").Output(); err == nil {
			for _, pidStr := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if pid, err := strconv.Atoi(strings.TrimSpace(pidStr)); err == nil && pid > 0 {
					pidSet[pid] = true
				}
			}
		}
		if len(pidSet) == 0 {
			// Try alternative: netstat fallback
			return resolvePortNetstat(port)
		}
	}

	// collect all owning pids so callers can handle multi-owner sockets
	result := make([]int, 0, len(pidSet))
	for pid := range pidSet {
		result = append(result, pid)
	}
	sort.Ints(result)

	return result, nil
}

func resolvePortNetstat(port int) ([]int, error) {
	pidSet := make(map[int]bool)
	fallbackSet := make(map[int]bool)
	sawListenNoOwner := false

	// Listening TCP and bound UDP sockets first; connected TCP sockets only
	// when nothing listens.
	for _, s := range procpkg.NetstatSockets() {
		if s.Port != port {
			continue
		}
		listening := s.Protocol == "UDP" || s.State == "LISTEN"
		switch {
		case s.PID <= 0:
			if s.State == "LISTEN" {
				sawListenNoOwner = true
			}
		case listening:
			pidSet[s.PID] = true
		default:
			fallbackSet[s.PID] = true
		}
	}

	if len(pidSet) == 0 && len(fallbackSet) > 0 {
		pidSet = fallbackSet
	}

	result := make([]int, 0, len(pidSet))
	for pid := range pidSet {
		result = append(result, pid)
	}
	sort.Ints(result)
	if len(result) > 0 {
		return result, nil
	}
	if sawListenNoOwner {
		return nil, ErrSocketOwnerUnknown
	}

	return nil, fmt.Errorf("no process listening on port %d", port)
}
