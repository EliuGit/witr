//go:build freebsd

package target

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func ResolvePort(port int) ([]int, error) {
	addressToPIDs, err := sockstatPortLookup(port, true)
	if err == nil && len(addressToPIDs) == 0 {
		err = fmt.Errorf("empty")
	}
	if err != nil {
		if fallback, fbErr := sockstatPortLookup(port, false); fbErr == nil && len(fallback) > 0 {
			addressToPIDs = fallback
		}
	}

	if len(addressToPIDs) == 0 {
		// Try netstat as fallback
		return resolvePortNetstat(port)
	}

	result := addressOwnerPIDs(addressToPIDs)
	if len(result) == 0 {
		return nil, ErrSocketOwnerUnknown
	}

	return result, nil
}

// sockstatPortLookup queries sockstat for the given port. When listenersOnly
// is true, only listening sockets are returned (the historical behavior). When
// false, all sockets bound to or connected on the local port are returned so
// processes with established connections become discoverable.
func sockstatPortLookup(port int, listenersOnly bool) (map[string][]int, error) {
	addressToPIDs := make(map[string][]int)

	for _, proto := range []string{"tcp", "udp"} {
		for _, flag := range []string{"-4", "-6"} {
			args := []string{flag, "-P", proto, "-p", strconv.Itoa(port)}
			if listenersOnly {
				args = append([]string{"-l"}, args...)
			}
			out, err := exec.Command("sockstat", args...).Output()
			if err != nil {
				continue
			}

			// Parse sockstat output
			// USER     COMMAND    PID   FD PROTO  LOCAL ADDRESS         FOREIGN ADDRESS
			// root     nginx      1234  6  tcp4   *:80                  *:*
			for line := range strings.Lines(string(out)) {
				fields := strings.Fields(line)
				if len(fields) < 6 {
					continue
				}
				if fields[0] == "USER" {
					continue
				}

				pid, err := strconv.Atoi(fields[2])
				if err != nil || pid <= 0 {
					continue
				}

				localAddr := fields[5]
				addressToPIDs[localAddr] = append(addressToPIDs[localAddr], pid)
			}
		}
	}

	return addressToPIDs, nil
}

func resolvePortNetstat(port int) ([]int, error) {
	portStr := fmt.Sprintf(".%d", port)
	portColonStr := fmt.Sprintf(":%d", port)

	// Check both TCP and UDP via netstat. Any match — listener or connected —
	// is enough to forward to fstat for PID resolution.
	for _, proto := range []string{"tcp", "udp"} {
		out, err := exec.Command("netstat", "-an", "-p", proto).Output()
		if err != nil {
			continue
		}

		for line := range strings.Lines(string(out)) {
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}
			if strings.HasSuffix(fields[3], portStr) || strings.HasSuffix(fields[3], portColonStr) {
				return resolvePortFstat(port)
			}
		}
	}

	return nil, fmt.Errorf("no process listening on port %d", port)
}

func resolvePortFstat(port int) ([]int, error) {
	// Use fstat to find processes with open sockets
	// This is less efficient but works as a fallback
	out, err := exec.Command("fstat").Output()
	if err != nil {
		return nil, fmt.Errorf("no process listening on port %d", port)
	}

	portSuffix := fmt.Sprintf(":%d", port)
	pidSet := make(map[int]bool)

	for line := range strings.Lines(string(out)) {
		if !strings.Contains(line, "tcp") && !strings.Contains(line, "udp") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		lastField := fields[len(fields)-1]
		if !strings.HasSuffix(lastField, portSuffix) {
			continue
		}
		pid, err := strconv.Atoi(fields[2])
		if err == nil && pid > 0 {
			pidSet[pid] = true
		}
	}

	if len(pidSet) == 0 {
		return nil, fmt.Errorf("no process listening on port %d", port)
	}

	// Return the lowest PID
	var result []int
	minPID := 0
	for pid := range pidSet {
		if minPID == 0 || pid < minPID {
			minPID = pid
		}
	}
	if minPID > 0 {
		result = append(result, minPID)
	}

	return result, nil
}
