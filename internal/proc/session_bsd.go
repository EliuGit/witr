//go:build darwin || freebsd

package proc

import "golang.org/x/sys/unix"

// sessionID returns pid's session ID, or 0 when it can't be read.
func sessionID(pid int) int {
	sid, err := unix.Getsid(pid)
	if err != nil {
		return 0
	}
	return sid
}
