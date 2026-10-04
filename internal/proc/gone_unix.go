//go:build !windows

package proc

import (
	"errors"

	"golang.org/x/sys/unix"
)

// processGone reports whether pid no longer exists. A process we can't see or
// signal (another user's under a hidden /proc) still exists, and PID 1 never
// exits even when it is invisible from inside a jail.
func processGone(pid int) bool {
	return pid != 1 && errors.Is(unix.Kill(pid, 0), unix.ESRCH)
}
