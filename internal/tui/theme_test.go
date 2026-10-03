package tui

import (
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/muesli/termenv"
)

// Below 256 colors, and under NO_COLOR (ASCII) with its colorless palette, the
// TUI must render as ANSI so the reverse-video selection still shows (#232).
func TestLipglossProfileFollowsDetectedProfile(t *testing.T) {
	want := map[colorprofile.Profile]termenv.Profile{
		colorprofile.NoTTY:     termenv.ANSI,
		colorprofile.ASCII:     termenv.ANSI,
		colorprofile.ANSI:      termenv.ANSI,
		colorprofile.ANSI256:   termenv.ANSI256,
		colorprofile.TrueColor: termenv.TrueColor,
	}
	for in, out := range want {
		if got := lipglossProfile(in); got != out {
			t.Errorf("lipglossProfile(%v) = %v, want %v", in, got, out)
		}
	}
}
