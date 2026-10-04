package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/pranshuparmar/witr/internal/output"
	"github.com/pranshuparmar/witr/pkg/model"
)

func TestPrintMultiMatch(t *testing.T) {
	for _, color := range []bool{false, true} {
		var buf bytes.Buffer
		outp := output.NewPrinter(&buf)
		// Anchor on our own PID so ReadProcess succeeds deterministically.
		printMultiMatch(outp, []int{os.Getpid()}, color, "witr --pid 1234")
		out := buf.String()
		if !strings.Contains(out, "Multiple matching processes") || !strings.Contains(out, "witr --pid 1234") {
			t.Errorf("color=%v: printMultiMatch output wrong:\n%s", color, out)
		}
		if strings.Contains(out, `\n`) {
			t.Errorf("color=%v: layout newline escaped to a literal \\n:\n%s", color, out)
		}
	}
}

func TestPrintContainerMultiMatch(t *testing.T) {
	matches := []*model.ContainerMatch{
		{Name: "web", Image: "nginx:latest", Runtime: "docker", Status: "Up 3 min", Ports: "0.0.0.0:80->80/tcp"},
	}
	for _, color := range []bool{false, true} {
		var buf bytes.Buffer
		outp := output.NewPrinter(&buf)
		printContainerMultiMatch(outp, matches, color)
		out := buf.String()
		if !strings.Contains(out, "Multiple matching containers") || !strings.Contains(out, "web") || !strings.Contains(out, "nginx:latest") {
			t.Errorf("color=%v: printContainerMultiMatch output wrong:\n%s", color, out)
		}
		if strings.Contains(out, `\n`) {
			t.Errorf("color=%v: layout newline escaped to a literal \\n:\n%s", color, out)
		}
	}
}

// Under --json, stdout must stay one JSON document when a name matches several
// processes: the candidates come back as data for a script to re-run with
// --pid, not as a list for people. Two real sleepers make the name ambiguous.
func TestJSONAmbiguousMatchStaysJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the witr binary; skipped under -short")
	}
	bin := buildWitr(t)

	name, args := "sleep", []string{"60"}
	if runtime.GOOS == "windows" {
		name, args = "ping", []string{"-n", "60", "127.0.0.1"}
	}
	var children []int
	for range 2 {
		c := exec.Command(name, args...)
		if err := c.Start(); err != nil {
			t.Skipf("spawn %s: %v", name, err)
		}
		t.Cleanup(func() { _ = c.Process.Kill(); _, _ = c.Process.Wait() })
		children = append(children, c.Process.Pid)
	}

	type entry struct {
		Error   string
		Matches []struct{ PID int }
	}
	run := func(args ...string) ([]byte, int) {
		out, err := exec.Command(bin, args...).Output()
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return out, ee.ExitCode()
		}
		if err != nil {
			t.Fatalf("witr %v: %v", args, err)
		}
		return out, 0
	}
	listsChildren := func(e entry) bool {
		found := 0
		for _, m := range e.Matches {
			for _, c := range children {
				if m.PID == c {
					found++
				}
			}
		}
		return e.Error != "" && found == len(children)
	}

	out, code := run(name, "--json")
	var single entry
	if err := json.Unmarshal(out, &single); err != nil || code != ExitInvalidInput || !listsChildren(single) {
		t.Errorf("witr %s --json: exit %d, err %v, want exit %d and both children listed:\n%s", name, code, err, ExitInvalidInput, out)
	}

	self := strconv.Itoa(os.Getpid())
	for _, extra := range [][]string{nil, {"--env"}} {
		args := append([]string{name, "--pid", self, "--json"}, extra...)
		out, code := run(args...)
		var all []entry
		if err := json.Unmarshal(out, &all); err != nil || code != ExitInvalidInput || len(all) != 2 || !listsChildren(all[0]) {
			t.Errorf("witr %v: exit %d, err %v, want a 2-entry array listing both children:\n%s", args, code, err, out)
		}
	}
}
