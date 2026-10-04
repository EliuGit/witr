package app

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/pranshuparmar/witr/pkg/model"
)

func sampleResult() model.Result {
	return model.Result{
		Process:  model.Process{PID: 1234, Command: "nginx", Cmdline: "nginx -g daemon off;"},
		Ancestry: []model.Process{{PID: 1, Command: "systemd"}, {PID: 1234, Command: "nginx"}},
		Source:   model.Source{Type: model.SourceSystemd, Name: "nginx.service"},
		Warnings: []string{"Process is running as root"},
	}
}

// An untraceable cause gets its own exit code in every output mode; Windows
// excluded, where an unknown source is routine.
func TestRenderResultExitCodes(t *testing.T) {
	t.Parallel()
	untraced := sampleResult()
	untraced.Source = model.Source{Type: model.SourceUnknown}
	wantUntraced := ExitCauseUnknown
	if runtime.GOOS == "windows" {
		wantUntraced = ExitWarnings
	}
	clean := sampleResult()
	clean.Warnings = nil

	for _, f := range []appFlags{{}, {short: true}, {tree: true}, {warn: true}, {json: true}} {
		var b bytes.Buffer
		var jr []string
		if got := renderResult(&b, untraced, f, false, &jr); got != wantUntraced {
			t.Errorf("%+v: untraced exit = %d, want %d", f, got, wantUntraced)
		}
		if got := renderResult(&b, sampleResult(), f, false, &jr); got != ExitWarnings {
			t.Errorf("%+v: warnings exit = %d, want %d", f, got, ExitWarnings)
		}
		if got := renderResult(&b, clean, f, false, &jr); got != ExitOK {
			t.Errorf("%+v: clean exit = %d, want %d", f, got, ExitOK)
		}
	}
}

// TestRenderResultDispatch pins the output-mode routing in renderResult: which
// renderer each flag selects, and that multi-target JSON accumulates into the
// shared slice instead of writing to the output stream.
func TestRenderResultDispatch(t *testing.T) {
	t.Parallel()
	res := sampleResult()

	// Standard (no flags) writes a human report containing the process name.
	var std bytes.Buffer
	var jr []string
	renderResult(&std, res, appFlags{}, false, &jr)
	if !strings.Contains(std.String(), "nginx") {
		t.Errorf("standard output missing process name:\n%s", std.String())
	}

	// JSON single-target writes serialized output containing the PID.
	var js bytes.Buffer
	renderResult(&js, res, appFlags{json: true}, false, &jr)
	if !strings.Contains(js.String(), "1234") {
		t.Errorf("json output missing pid:\n%s", js.String())
	}

	// JSON multi-target accumulates into jsonResults and does not write to outw.
	jr = nil
	var jm bytes.Buffer
	renderResult(&jm, res, appFlags{json: true}, true, &jr)
	if len(jr) != 1 {
		t.Errorf("multi-target json: got %d accumulated results, want 1", len(jr))
	}
	if jm.Len() != 0 {
		t.Errorf("multi-target json must not write to outw, got: %s", jm.String())
	}

	// Each non-JSON mode produces some output.
	modes := map[string]appFlags{
		"short":    {short: true},
		"tree":     {tree: true},
		"warnings": {warn: true},
	}
	for name, f := range modes {
		var b bytes.Buffer
		renderResult(&b, res, f, false, &jr)
		if b.Len() == 0 {
			t.Errorf("%s mode produced no output", name)
		}
	}
}
