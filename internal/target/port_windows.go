//go:build windows

package target

import (
	"fmt"

	procpkg "github.com/pranshuparmar/witr/internal/proc"
)

func ResolvePort(port int) ([]int, error) {
	socks, err := procpkg.ListSockets()
	if err != nil {
		return nil, err
	}
	return portOwnerPIDs(socks, port)
}

// portOwnerPIDs picks the processes behind port from the socket tables: those
// listening on it (TCP) or bound to it (UDP), else those with a connection on
// it at either end. A listener without an owner (PID 0) means the owner isn't
// visible.
func portOwnerPIDs(socks []procpkg.WinSocket, port int) ([]int, error) {
	var pids, fallbackPIDs []int
	seen := make(map[int]bool)
	fallbackSeen := make(map[int]bool)
	sawListenNoOwner := false

	for _, s := range socks {
		matchesLocal := s.LocalPort == port

		if s.Protocol == "UDP" {
			if matchesLocal && s.PID != 0 && !seen[s.PID] {
				pids = append(pids, s.PID)
				seen[s.PID] = true
			}
			continue
		}

		if !matchesLocal && s.RemotePort != port {
			continue
		}
		isListen := matchesLocal && s.State == "LISTEN"
		if s.PID == 0 {
			if isListen {
				sawListenNoOwner = true
			}
			continue
		}
		if isListen {
			if !seen[s.PID] {
				pids = append(pids, s.PID)
				seen[s.PID] = true
			}
		} else if !fallbackSeen[s.PID] {
			fallbackPIDs = append(fallbackPIDs, s.PID)
			fallbackSeen[s.PID] = true
		}
	}

	if len(pids) > 0 {
		return pids, nil
	}
	if len(fallbackPIDs) > 0 {
		return fallbackPIDs, nil
	}
	if sawListenNoOwner {
		return nil, ErrSocketOwnerUnknown
	}
	return nil, fmt.Errorf("no process found listening on port %d", port)
}
