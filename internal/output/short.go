package output

import (
	"fmt"
	"io"

	"github.com/pranshuparmar/witr/pkg/model"
)

func RenderShort(w io.Writer, r model.Result, colorEnabled bool) {
	p := NewPrinter(w)

	for i, proc := range r.Ancestry {
		if i > 0 {
			if colorEnabled {
				p.Printf("%s → %s", ColorMagenta, ColorReset)
			} else {
				p.Print(" → ")
			}
		}
		if gap := ParentGap(r.Ancestry, i); gap != "" {
			if colorEnabled {
				p.Printf("%s%s%s%s → %s", ColorDimYellow, gap, ColorReset, ColorMagenta, ColorReset)
			} else {
				p.Printf("%s → ", gap)
			}
		}

		if colorEnabled {
			nameColor := ansiString("")
			if i == len(r.Ancestry)-1 {
				nameColor = ColorGreen
			}
			p.Printf("%s%s%s (%spid %d%s)", nameColor, ChainName(proc), ColorReset, ColorDim, proc.PID, ColorReset)
		} else {
			p.Printf("%s (pid %d)", ChainName(proc), proc.PID)
		}
	}
	p.Println()
}

// ParentGap returns the placeholder shown just above chain[i] when the process
// that started it has exited, or "" when the chain is intact there. At the top
// of a chain the exited parent's PID is still known; further down, the process
// was adopted and its original parent is unknown.
func ParentGap(chain []model.Process, i int) string {
	p := chain[i]
	if !p.ParentExited {
		return ""
	}
	if i == 0 && p.PPID > 0 {
		return fmt.Sprintf("? (parent pid %d exited)", p.PPID)
	}
	return "? (original parent exited)"
}

// ChainName returns a display name for an ancestry or child node, falling back
// to the command line and then a placeholder when the process name couldn't be
// read (e.g. a protected or already-exited Windows ancestor that exposes
// neither an image name nor a command line).
func ChainName(p model.Process) string {
	if p.Command != "" {
		return p.Command
	}
	if p.Cmdline != "" {
		return p.Cmdline
	}
	return "(unknown)"
}
