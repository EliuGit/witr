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
	// User owns the socket when its process isn't visible (PID 0); Linux
	// records it even for other users' sockets.
	User string
}
