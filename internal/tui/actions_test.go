//go:build !windows

package tui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pranshuparmar/witr/internal/proc"
	"github.com/pranshuparmar/witr/pkg/model"
)

func TestSetNiceRangeGuard(t *testing.T) {
	// Out-of-range values are rejected before any syscall is attempted.
	if err := setNice(os.Getpid(), 100); err == nil {
		t.Error("setNice(+100) should be rejected as out of range")
	}
	if err := setNice(os.Getpid(), -100); err == nil {
		t.Error("setNice(-100) should be rejected as out of range")
	}
}

// startSleeper spawns a long-lived child to receive signals.
func startSleeper(t *testing.T) *exec.Cmd {
	t.Helper()
	c := exec.Command("sleep", "60")
	if err := c.Start(); err != nil {
		t.Skipf("cannot spawn child process: %v", err)
	}
	return c
}

// waitExit reports whether the child reaps within the timeout (i.e. the signal
// actually terminated it).
func waitExit(c *exec.Cmd, timeout time.Duration) bool {
	done := make(chan struct{})
	go func() { _ = c.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func TestSignalActionsTerminate(t *testing.T) {
	t.Run("SIGTERM exits the process", func(t *testing.T) {
		c := startSleeper(t)
		if err := termProcess(c.Process.Pid); err != nil {
			t.Fatalf("termProcess: %v", err)
		}
		if !waitExit(c, 5*time.Second) {
			_ = c.Process.Kill()
			t.Fatal("process did not exit after SIGTERM")
		}
	})

	t.Run("SIGKILL exits the process", func(t *testing.T) {
		c := startSleeper(t)
		if err := killProcess(c.Process.Pid); err != nil {
			t.Fatalf("killProcess: %v", err)
		}
		if !waitExit(c, 5*time.Second) {
			t.Fatal("process did not exit after SIGKILL")
		}
	})
}

func TestSignalActionsPauseResume(t *testing.T) {
	c := startSleeper(t)
	defer func() {
		_ = c.Process.Kill()
		_ = c.Wait()
	}()

	if err := pauseProcess(c.Process.Pid); err != nil {
		t.Fatalf("pauseProcess: %v", err)
	}
	if !waitForState(c.Process.Pid, 'T', 2*time.Second) {
		t.Error("process did not reach the stopped (T) state after SIGSTOP")
	}
	if err := resumeProcess(c.Process.Pid); err != nil {
		t.Fatalf("resumeProcess: %v", err)
	}
}

func TestSignalDeadProcessErrors(t *testing.T) {
	c := startSleeper(t)
	pid := c.Process.Pid
	_ = c.Process.Kill()
	_ = c.Wait() // reap, so the PID is no longer signalable

	if err := resumeProcess(pid); err == nil {
		t.Error("signalling a reaped PID should fail")
	}
}

// waitForState polls /proc for the given process state on Linux. On other
// platforms it returns true (the /proc check is not meaningful there), so the
// pause assertion is only enforced where it can be observed.
func waitForState(pid int, want byte, timeout time.Duration) bool {
	if runtime.GOOS != "linux" {
		return true
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
			// Format: "pid (comm) state ..."; comm may contain spaces and
			// parens, so the state is the char two positions after the last ')'.
			if i := bytes.LastIndexByte(data, ')'); i >= 0 && i+2 < len(data) && data[i+2] == want {
				return true
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// listWith returns a laid-out list view showing ps, with the cursor on the first.
func listWith(t *testing.T, ps ...model.Process) MainModel {
	t.Helper()
	m, _ := step(t, InitialModel("test"), tea.WindowSizeMsg{Width: 160, Height: 40})
	m.processes = ps
	m.filterProcesses()
	m.table.SetCursor(0)
	return m
}

func readChild(t *testing.T, c *exec.Cmd) model.Process {
	t.Helper()
	p, err := proc.ReadProcess(c.Process.Pid)
	if err != nil {
		t.Skipf("cannot read child process: %v", err)
	}
	return p
}

func TestListActionSignalsCapturedTarget(t *testing.T) {
	target, decoy := startSleeper(t), startSleeper(t)
	defer func() { _ = decoy.Process.Kill() }()

	m := listWith(t, readChild(t, target), readChild(t, decoy))
	m, _ = step(t, m, keyRunes("a"))

	// The rows change under the open menu, leaving the decoy under the cursor.
	m.processes[0], m.processes[1] = m.processes[1], m.processes[0]
	m.filterProcesses()

	m, _ = step(t, m, keyRunes("k"))
	m, _ = step(t, m, keyRunes("y"))

	if !waitExit(target, 5*time.Second) {
		t.Fatal("the process captured when the menu opened should have been killed")
	}
	if waitExit(decoy, 300*time.Millisecond) {
		t.Fatal("the process that moved under the cursor must not be signalled")
	}
	if m.state != stateList || m.actionActive() || !strings.Contains(m.statusMsg, "Signal sent") {
		t.Errorf("state=%v active=%v status=%q, want back on the list with a confirmation", m.state, m.actionActive(), m.statusMsg)
	}
}

func TestListActionRefusesReusedPID(t *testing.T) {
	c := startSleeper(t)
	defer func() { _ = c.Process.Kill() }()

	// The listed start time no longer matches, as if the PID now belongs to
	// a newer process.
	p := readChild(t, c)
	p.StartedAt = p.StartedAt.Add(-time.Hour)

	m := listWith(t, p)
	for _, k := range []string{"a", "k", "y"} {
		m, _ = step(t, m, keyRunes(k))
	}

	if waitExit(c, 300*time.Millisecond) {
		t.Fatal("a process whose identity changed must not be signalled")
	}
	if !strings.Contains(m.statusMsg, "changed since selected") {
		t.Errorf("status = %q, want a PID-changed refusal", m.statusMsg)
	}
}
