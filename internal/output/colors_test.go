package output

import (
	"testing"

	"github.com/charmbracelet/colorprofile"
)

// A terminal that advertises only 16 colors ignores the bright foregrounds
// (90-97), so the palette has to fall back to the standard codes there —
// otherwise a plain TERM=xterm renders with no color at all (#232). Anything
// with 256 colors or better keeps the bright codes chosen for dark-theme
// contrast.
func TestPaletteSelectsCodesByColorProfile(t *testing.T) {
	standard := []ansiString{"\033[31m", "\033[32m", "\033[34m", "\033[36m", "\033[35m", "\033[2m", "\033[2;33m"}
	bright := []ansiString{"\033[91m", "\033[92m", "\033[94m", "\033[96m", "\033[95m", "\033[90m", "\033[93m"}

	want := map[colorprofile.Profile][]ansiString{
		colorprofile.NoTTY:     standard,
		colorprofile.ASCII:     standard,
		colorprofile.ANSI:      standard,
		colorprofile.ANSI256:   bright,
		colorprofile.TrueColor: bright,
	}

	for profile, codes := range want {
		r, g, b, c, m, d, dy := palette(profile)
		got := []ansiString{r, g, b, c, m, d, dy}
		for i := range codes {
			if got[i] != codes[i] {
				t.Errorf("palette(%v)[%d] = %q, want %q", profile, i, got[i], codes[i])
			}
		}
	}
}
