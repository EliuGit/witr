//go:build windows

package proc

import (
	"github.com/pranshuparmar/witr/pkg/model"
)

// GetSocketStateForPort returns the most relevant TCP socket state for a port.
func GetSocketStateForPort(port int) *model.SocketInfo {
	socks, err := ListSockets()
	if err != nil {
		return nil
	}

	var states []model.SocketInfo
	for _, s := range socks {
		if s.Protocol != "TCP" || s.LocalPort != port {
			continue
		}
		info := model.SocketInfo{
			Port:       port,
			State:      s.State,
			LocalAddr:  s.LocalIP,
			RemoteAddr: s.RemoteIP,
		}
		addStateExplanation(&info)
		states = append(states, info)
	}

	if len(states) == 0 {
		return nil
	}

	// Prioritize problematic states
	for _, s := range states {
		if s.State == "TIME_WAIT" || s.State == "CLOSE_WAIT" || s.State == "FIN_WAIT_1" || s.State == "FIN_WAIT_2" {
			return &s
		}
	}

	// Return LISTEN
	for _, s := range states {
		if s.State == "LISTEN" {
			return &s
		}
	}

	return &states[0]
}

func addStateExplanation(info *model.SocketInfo) {
	switch info.State {
	case "LISTEN":
		info.Explanation = "Actively listening for connections"
	case "TIME_WAIT":
		info.Explanation = "Connection closed, waiting for delayed packets"
		info.Workaround = "Wait for timeout (usually 60-240s) or reuse port"
	case "CLOSE_WAIT":
		info.Explanation = "Remote side closed connection, local side still has it open"
		info.Workaround = "Check if application is leaking connections or hanging"
	case "ESTABLISHED":
		info.Explanation = "Active connection established"
	case "SYN_SENT":
		info.Explanation = "Attempting to establish connection"
		info.Workaround = "Check firewall or if remote host is up"
	case "SYN_RECEIVED":
		info.Explanation = "Received connection request, sending ack"
	}
}
