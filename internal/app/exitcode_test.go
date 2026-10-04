package app

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pranshuparmar/witr/pkg/model"
)

// buildWitr compiles the real witr binary once so the process exit codes can be
// characterized end to end (Execute -> runApp -> os.Exit). The exit-code
// contract drives scripting and CI integrations, so it's asserted against the
// actual binary rather than internal helpers.
func buildWitr(t *testing.T) string {
	t.Helper()

	gomod, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Fatalf("go env GOMOD: %v", err)
	}
	root := filepath.Dir(strings.TrimSpace(string(gomod)))

	bin := filepath.Join(t.TempDir(), "witr")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, "./cmd/witr")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build witr: %v\n%s", err, out)
	}
	return bin
}

func runExit(t *testing.T, bin string, args ...string) int {
	t.Helper()
	err := exec.Command(bin, args...).Run()
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	t.Fatalf("run witr %v: %v", args, err)
	return -1
}

func TestJSONFailureIsJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the witr binary; skipped under -short")
	}
	bin := buildWitr(t)

	out, err := exec.Command(bin, "--pid", "2147483646", "--json").Output()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != ExitNotFound {
		t.Fatalf("exit = %v, want %d", err, ExitNotFound)
	}
	var entry struct {
		Target model.Target
		Error  string
	}
	if err := json.Unmarshal(out, &entry); err != nil || entry.Error == "" || entry.Target.Value != "2147483646" {
		t.Errorf("stdout should be a {Target, Error} JSON entry, got %q (%v)", out, err)
	}
}

func TestExitCodes(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the witr binary; skipped under -short")
	}
	bin := buildWitr(t)

	// PID 2147483646 is far above any real PID on Linux/macOS/Windows, so the
	// "not found" path is deterministic on CI runners.
	const ghostPID = "2147483646"

	tests := []struct {
		name string
		args []string
		want int
	}{
		{"invalid pid (non-numeric)", []string{"--pid", "notanumber"}, ExitInvalidInput},
		{"invalid pid (zero)", []string{"--pid", "0"}, ExitInvalidInput},
		{"invalid port (out of range)", []string{"--port", "70000"}, ExitInvalidInput},
		{"unknown flag", []string{"--no-such-flag"}, ExitInvalidInput},
		{"flag missing its value", []string{"--pid"}, ExitInvalidInput},
		{"no target without a terminal", []string{"--no-color"}, ExitInvalidInput},
		{"output mode without a target", []string{"--json"}, ExitInvalidInput},
		{"interactive without a terminal", []string{"-i", "--pid", "1"}, ExitInvalidInput},
		{"not found (ghost pid)", []string{"--pid", ghostPID}, ExitNotFound},
		{"not found with --env", []string{"--pid", ghostPID, "--env"}, ExitNotFound},
		{"no such container", []string{"--container", "witr-no-such-container"}, ExitNotFound},
		{"no such container with --env", []string{"--container", "witr-no-such-container", "--env"}, ExitNotFound},
		{"combined short flags", []string{"-sp", ghostPID}, ExitNotFound},
		{"attached short flag value", []string{"-p" + ghostPID}, ExitNotFound},
		// Multi-target exit code is the highest severity among targets, not the
		// first or last — assert with both orderings of a not-found(2) and an
		// invalid(4) target.
		{"multi: not-found then invalid", []string{"--pid", ghostPID, "--port", "70000"}, ExitInvalidInput},
		{"multi: invalid then not-found", []string{"--port", "70000", "--pid", ghostPID}, ExitInvalidInput},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := runExit(t, bin, tc.args...); got != tc.want {
				t.Errorf("witr %v exit = %d, want %d", tc.args, got, tc.want)
			}
		})
	}
}

// An output mode with nothing to explain is a usage error, in a terminal or
// not; only a bare `witr` (or one with display modifiers) opens the TUI.
func TestOutputModeNeedsATarget(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the witr binary; skipped under -short")
	}
	bin := buildWitr(t)

	tests := []struct {
		args []string
		want string
	}{
		{[]string{"--short"}, "must specify"},
		{[]string{"--tree"}, "must specify"},
		{[]string{"--json"}, "must specify"},
		{[]string{"--warnings"}, "must specify"},
		{[]string{"--verbose"}, "must specify"},
		{[]string{"--env"}, "must specify"},
		{[]string{}, "interactive mode needs a terminal"},
		{[]string{"--no-color"}, "interactive mode needs a terminal"},
	}
	for _, tc := range tests {
		out, err := exec.Command(bin, tc.args...).CombinedOutput()
		ee, ok := err.(*exec.ExitError)
		if !ok || ee.ExitCode() != ExitInvalidInput {
			t.Errorf("witr %v: exit = %v, want %d", tc.args, err, ExitInvalidInput)
		}
		if !strings.Contains(string(out), tc.want) {
			t.Errorf("witr %v: output %q, want it to mention %q", tc.args, out, tc.want)
		}
	}
}
