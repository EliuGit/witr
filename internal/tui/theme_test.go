package tui

import (
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/muesli/termenv"
)

// Below 256 colors the TUI must render as ANSI so the basic palette and the
// reverse-video selection still show (#232); only NO_COLOR (ASCII) drops
// styling entirely.
func TestLipglossProfileFollowsDetectedProfile(t *testing.T) {
	want := map[colorprofile.Profile]termenv.Profile{
		colorprofile.NoTTY:     termenv.ANSI,
		colorprofile.ASCII:     termenv.Ascii,
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
