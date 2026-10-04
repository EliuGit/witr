package model

type OpenPort struct {
	PID      int
	Port     int
	Address  string
	Protocol string
	State    string
	// The other end of a connected socket; empty for listeners.
	RemoteAddress string
	RemotePort    int
}
