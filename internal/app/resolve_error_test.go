package app

import (
	"bytes"
	"encoding/json"
	"errors"
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
	orig := containerByPort
	defer func() { containerByPort = orig }()
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
