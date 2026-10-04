//go:build windows

package proc

import "syscall"

// stillActive is the exit code Windows reports for a running process.
const stillActive = 259

// processGone reports whether pid no longer exists. ReadProcess only fails on
// Windows when the process has exited: one it can't open still comes back from
// the system snapshot.
func processGone(int) bool { return true }

// processExited reports whether pid has exited even though its process object
// lingers because something still holds a handle to it.
func processExited(pid int) bool {
	h, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code != stillActive
}
