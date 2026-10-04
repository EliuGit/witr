package output

import (
	"io"
	"strings"

	"github.com/pranshuparmar/witr/pkg/model"
)

func PrintTree(w io.Writer, chain []model.Process, children []model.Process, colorEnabled bool) {
	p := NewPrinter(w)

	depth := 0
	branch := func() {
		if depth > 0 {
			indent := strings.Repeat("  ", depth)
			if colorEnabled {
				p.Printf("%s%s└─ %s", indent, ColorMagenta, ColorReset)
			} else {
				p.Printf("%s└─ ", indent)
			}
		}
		depth++
	}

	for i, proc := range chain {
		if gap := ParentGap(chain, i); gap != "" {
			branch()
			if colorEnabled {
				p.Printf("%s%s%s\n", ColorDimYellow, gap, ColorReset)
			} else {
				p.Printf("%s\n", gap)
			}
		}
		branch()

		if colorEnabled {
			cmdColor := ansiString("")
			if i == len(chain)-1 {
				cmdColor = ColorGreen
			}
			p.Printf("%s%s%s (%spid %d%s)\n", cmdColor, ChainName(proc), ColorReset, ColorDim, proc.PID, ColorReset)
		} else {
			p.Printf("%s (pid %d)\n", ChainName(proc), proc.PID)
		}
	}

	if len(children) == 0 {
		return
	}

	baseIndent := strings.Repeat("  ", depth)

	limit := 10
	count := len(children)
	for i, child := range children {
		if i >= limit {
			remaining := count - limit
			if colorEnabled {
				p.Printf("%s%s└─ %s... and %d more\n", baseIndent, ColorMagenta, ColorReset, remaining)
			} else {
				p.Printf("%s└─ ... and %d more\n", baseIndent, remaining)
			}
			break
		}

		connector := "├─ "
		isLast := (i == count-1) || (i == limit-1 && count <= limit)
		if isLast {
			connector = "└─ "
		}

		if colorEnabled {
			p.Printf("%s%s%s%s%s (%spid %d%s)\n", baseIndent, ColorMagenta, connector, ColorReset, ChainName(child), ColorDim, child.PID, ColorReset)
		} else {
			p.Printf("%s%s%s (pid %d)\n", baseIndent, connector, ChainName(child), child.PID)
		}
	}
}
