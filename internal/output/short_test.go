package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pranshuparmar/witr/pkg/model"
)

// shortFixture exercises every branch of RenderShort: a first node (no
// separator), middle nodes (with arrow separator), and a final node (last
// element of the chain).
func shortFixture() model.Result {
	return model.Result{
		Ancestry: []model.Process{
			{PID: 1, Command: "systemd"},
			{PID: 1234, Command: "bash"},
			{PID: 5678, Command: "nginx"},
		},
	}
}

func TestRenderShort(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	RenderShort(&buf, shortFixture(), false)
	got := strings.TrimSpace(buf.String())

	want := "systemd (pid 1) → bash (pid 1234) → nginx (pid 5678)"
	if got != want {
		t.Errorf("RenderShort = %q, want %q", got, want)
	}
}

func TestRenderChainShowsExitedParent(t *testing.T) {
	t.Parallel()

	adopted := model.Result{Ancestry: []model.Process{
		{PID: 1, Command: "systemd"},
		{PID: 300, PPID: 1, Command: "sleep", ParentExited: true},
	}}
	truncated := model.Result{Ancestry: []model.Process{
		{PID: 300, PPID: 100, Command: "worker", ParentExited: true},
	}}

	tests := []struct {
		name   string
		render func(*bytes.Buffer, model.Result)
		r      model.Result
		want   string
	}{
		{"short, adopted", func(b *bytes.Buffer, r model.Result) { RenderShort(b, r, false) }, adopted,
			"systemd (pid 1) → ? (original parent exited) → sleep (pid 300)"},
		{"short, truncated", func(b *bytes.Buffer, r model.Result) { RenderShort(b, r, false) }, truncated,
			"? (parent pid 100 exited) → worker (pid 300)"},
		{"standard, adopted", func(b *bytes.Buffer, r model.Result) { RenderStandard(b, r, false, false) }, adopted,
			"systemd (pid 1) → ? (original parent exited) → sleep (pid 300)"},
		{"tree, adopted", func(b *bytes.Buffer, r model.Result) { PrintTree(b, r.Ancestry, nil, false) }, adopted,
			"systemd (pid 1)\n  └─ ? (original parent exited)\n    └─ sleep (pid 300)"},
	}
	for _, tt := range tests {
		var buf bytes.Buffer
		tt.render(&buf, tt.r)
		if !strings.Contains(buf.String(), tt.want) {
			t.Errorf("%s: output missing %q\ngot:\n%s", tt.name, tt.want, buf.String())
		}
	}
}

func TestRenderShortSingleProcess(t *testing.T) {
	t.Parallel()

	r := model.Result{Ancestry: []model.Process{{PID: 1, Command: "init"}}}

	var buf bytes.Buffer
	RenderShort(&buf, r, false)
	got := strings.TrimSpace(buf.String())

	want := "init (pid 1)"
	if got != want {
		t.Errorf("RenderShort single process = %q, want %q", got, want)
	}
}

// TestRenderShortUnknownAncestor covers the issue #205 case: a protected or
// already-exited ancestor (e.g. userinit.exe) exposes neither an image name nor
// a command line. It must render as "(unknown)" instead of a blank name.
func TestRenderShortUnknownAncestor(t *testing.T) {
	t.Parallel()

	r := model.Result{
		Ancestry: []model.Process{
			{PID: 3540},
			{PID: 15116, Command: "Explorer.EXE"},
			{PID: 6324, Command: "Claude.exe"},
		},
	}

	var buf bytes.Buffer
	RenderShort(&buf, r, false)
	got := strings.TrimSpace(buf.String())

	want := "(unknown) (pid 3540) → Explorer.EXE (pid 15116) → Claude.exe (pid 6324)"
	if got != want {
		t.Errorf("RenderShort with unreadable root = %q, want %q", got, want)
	}
}

func TestChainName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		p    model.Process
		want string
	}{
		{"command present", model.Process{Command: "nginx"}, "nginx"},
		{"falls back to cmdline", model.Process{Cmdline: "/usr/bin/foo --bar"}, "/usr/bin/foo --bar"},
		{"unknown when both empty", model.Process{PID: 3540}, "(unknown)"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ChainName(tt.p); got != tt.want {
				t.Errorf("ChainName(%+v) = %q, want %q", tt.p, got, tt.want)
			}
		})
	}
}

func TestRenderShortEmptyAncestry(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	RenderShort(&buf, model.Result{}, false)

	// Empty ancestry should produce just a newline, not panic or omit anything.
	if got := buf.String(); got != "\n" {
		t.Errorf("RenderShort with empty ancestry = %q, want a single newline", got)
	}
}
