package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/pranshuparmar/witr/internal/output"
	"github.com/pranshuparmar/witr/internal/target"
	"github.com/pranshuparmar/witr/pkg/model"
	"github.com/spf13/cobra"
)

func TestHandleResolveError(t *testing.T) {
	newCmd := func() *cobra.Command {
		cmd := &cobra.Command{}
		var errBuf bytes.Buffer
		cmd.SetErr(&errBuf)
		cmd.SetOut(&errBuf)
		return cmd
	}

	t.Run("generic not-found maps to ExitNotFound", func(t *testing.T) {
		var outw bytes.Buffer
		var jsonResults []string
		code := handleResolveError(newCmd(), &outw, output.NewPrinter(&outw),
			model.Target{Type: model.TargetName, Value: "ghost"},
			errors.New("no matching process found"),
			appFlags{}, false, &jsonResults)
		if code != ExitNotFound {
			t.Errorf("code = %d, want %d (ExitNotFound)", code, ExitNotFound)
		}
	})

	t.Run("unsupported target maps to ExitInvalidInput", func(t *testing.T) {
		var outw bytes.Buffer
		var jsonResults []string
		code := handleResolveError(newCmd(), &outw, output.NewPrinter(&outw),
			model.Target{Type: model.TargetFile, Value: "/x"},
			target.ErrUnsupported,
			appFlags{}, false, &jsonResults)
		if code != ExitInvalidInput {
			t.Errorf("code = %d, want %d (ExitInvalidInput)", code, ExitInvalidInput)
		}
	})

	t.Run("multi-mode JSON appends an error entry", func(t *testing.T) {
		var outw bytes.Buffer
		var jsonResults []string
		handleResolveError(newCmd(), &outw, output.NewPrinter(&outw),
			model.Target{Type: model.TargetName, Value: "ghost"},
			errors.New("no matching process found"),
			appFlags{json: true}, true, &jsonResults)
		if len(jsonResults) != 1 {
			t.Errorf("expected 1 JSON error entry, got %d", len(jsonResults))
		}
	})
}

// A port no visible process holds is explained by the container publishing it
// when there is one; otherwise it is a permission problem (exit 3), with the
// sudo hint in text and a {Target, Error} entry under --json.
func TestHandleResolveErrorOwnerUnknown(t *testing.T) {
	orig, origHidden := containerByPort, ownersHidden
	defer func() { containerByPort, ownersHidden = orig, origHidden }()
	ownersHidden = func() bool { return true }
	lookups := 0
	containerByPort = func(port int, proto string) *model.ContainerMatch {
		if port != 18090 || proto != "" {
			t.Errorf("container lookup for %d/%q, want 18090 over either protocol", port, proto)
		}
		lookups++
		return nil
	}
	port := model.Target{Type: model.TargetPort, Value: "18090"}
	run := func(err error, flags appFlags, multi bool) (int, string, string, []string) {
		cmd := &cobra.Command{}
		var out, errOut bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		var jsonResults []string
		code := handleResolveError(cmd, &out, output.NewPrinter(&out), port, err, flags, multi, &jsonResults)
		return code, out.String(), errOut.String(), jsonResults
	}

	if code, _, errOut, _ := run(target.ErrSocketOwnerUnknown, appFlags{}, false); code != ExitPermission || !strings.Contains(errOut, "Try running with sudo") {
		t.Errorf("text: exit %d, stderr %q", code, errOut)
	}
	code, out, _, _ := run(target.ErrSocketOwnerUnknown, appFlags{json: true}, false)
	var entry struct {
		Target model.Target
		Error  string
	}
	if code != ExitPermission || json.Unmarshal([]byte(out), &entry) != nil || entry.Target.Value != "18090" || !strings.Contains(entry.Error, "try sudo") {
		t.Errorf("json: exit %d, stdout %q", code, out)
	}
	if code, out, _, _ := run(target.ErrSocketOwnerUnknown, appFlags{}, true); code != ExitPermission || !strings.Contains(out, "Error: socket found but owning process not detected") {
		t.Errorf("multi-target: exit %d, output %q", code, out)
	}
	if code, _, _, results := run(target.ErrSocketOwnerUnknown, appFlags{json: true}, true); code != ExitPermission || len(results) != 1 {
		t.Errorf("multi-target json: exit %d, %d entries", code, len(results))
	}
	if code, _, _, _ := run(errors.New("no process listening on port 18090"), appFlags{}, false); code != ExitNotFound {
		t.Errorf("nothing on the port: exit %d, want %d", code, ExitNotFound)
	}
	if lookups != 5 {
		t.Errorf("container lookups = %d, want one per port failure", lookups)
	}

	// A name target never goes looking for containers.
	lookups = 0
	cmd := &cobra.Command{}
	cmd.SetErr(&bytes.Buffer{})
	var buf bytes.Buffer
	handleResolveError(cmd, &buf, output.NewPrinter(&buf), model.Target{Type: model.TargetName, Value: "ghost"}, errors.New("no matching process found"), appFlags{}, false, nil)
	if lookups != 0 {
		t.Errorf("a name target looked up containers")
	}
}

// When the port's socket shows whose it is, every output mode says so.
func TestHandleResolveErrorNamesHiddenOwner(t *testing.T) {
	orig, origHidden := containerByPort, ownersHidden
	defer func() { containerByPort, ownersHidden = orig, origHidden }()
	ownersHidden = func() bool { return true }
	containerByPort = func(int, string) *model.ContainerMatch { return nil }
	err := fmt.Errorf("%w; it belongs to postgres", target.ErrSocketOwnerUnknown)
	port := model.Target{Type: model.TargetPort, Value: "5432"}

	cmd := &cobra.Command{}
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if code := handleResolveError(cmd, &out, output.NewPrinter(&out), port, err, appFlags{}, false, nil); code != ExitPermission || !strings.Contains(errOut.String(), "it belongs to postgres") || !strings.Contains(errOut.String(), "sudo") {
		t.Errorf("text: exit %d, stderr %q", code, errOut.String())
	}

	out.Reset()
	var entry struct{ Error string }
	if code := handleResolveError(cmd, &out, output.NewPrinter(&out), port, err, appFlags{json: true}, false, nil); code != ExitPermission || json.Unmarshal(out.Bytes(), &entry) != nil ||
		entry.Error != "socket found but owning process not detected; it belongs to postgres (try sudo)" {
		t.Errorf("json: exit %d, stdout %q", code, out.String())
	}
}

// Run as root (or on Windows), a socket no process holds isn't hidden: it's
// held from outside this system, so the answer is "not found here", not sudo.
func TestHandleResolveErrorHeldOutside(t *testing.T) {
	orig, origHidden, origWSL := containerByPort, ownersHidden, onWSL
	defer func() { containerByPort, ownersHidden, onWSL = orig, origHidden, origWSL }()
	containerByPort = func(int, string) *model.ContainerMatch { return nil }
	ownersHidden = func() bool { return false }
	err := fmt.Errorf("%w; it belongs to root", target.ErrSocketOwnerUnknown)
	port := model.Target{Type: model.TargetPort, Value: "6443"}
	run := func(flags appFlags) (int, string, string) {
		cmd := &cobra.Command{}
		var out, errOut bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		code := handleResolveError(cmd, &out, output.NewPrinter(&out), port, err, flags, false, nil)
		return code, out.String(), errOut.String()
	}

	for _, wsl := range []bool{true, false} {
		onWSL = func() bool { return wsl }
		code, _, errOut := run(appFlags{})
		if code != ExitNotFound || strings.Contains(errOut, "sudo") || !strings.Contains(errOut, "it belongs to root") || !strings.Contains(errOut, "No process") {
			t.Errorf("wsl=%v: exit %d, stderr %q", wsl, code, errOut)
		}
		if wsl != strings.Contains(errOut, "another distro") {
			t.Errorf("wsl=%v: stderr %q", wsl, errOut)
		}
	}
	code, out, _ := run(appFlags{json: true})
	var entry struct{ Error string }
	if code != ExitNotFound || json.Unmarshal([]byte(out), &entry) != nil ||
		entry.Error != "socket found but owning process not detected; it belongs to root (no process on this system holds it)" {
		t.Errorf("json: exit %d, stdout %q", code, out)
	}
}
