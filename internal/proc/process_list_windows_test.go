//go:build windows

package proc

import "testing"

// PID 0 is the System Idle Process placeholder; the TUI list leaves it out.
func TestListProcessesSkipsIdleProcess(t *testing.T) {
	procs, err := ListProcesses()
	if err != nil {
		t.Fatalf("ListProcesses: %v", err)
	}
	for _, p := range procs {
		if p.PID == 0 {
			t.Fatalf("ListProcesses includes PID 0 (%s)", p.Command)
		}
	}
	if len(procs) == 0 {
		t.Fatal("ListProcesses returned nothing")
	}
}
