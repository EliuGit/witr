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

func TestDetectProfileHonorsNoColor(t *testing.T) {
	color := []string{"TERM=xterm-256color"}
	tests := []struct {
		name    string
		environ []string
		args    []string
		want    bool
	}{
		{"default", color, nil, false},
		{"NO_COLOR=1", append(color, "NO_COLOR=1"), nil, true},
		{"NO_COLOR=yes", append(color, "NO_COLOR=yes"), nil, true},
		{"empty NO_COLOR", append(color, "NO_COLOR="), nil, false},
		{"--no-color", color, []string{"-i", "--no-color"}, true},
		{"--no-color=false", color, []string{"--no-color=false"}, false},
		{"after --", color, []string{"--", "--no-color"}, false},
	}
	for _, tt := range tests {
		if got := detectProfile(tt.environ, tt.args) == colorprofile.ASCII; got != tt.want {
			t.Errorf("%s: colorless = %v, want %v", tt.name, got, tt.want)
		}
	}
}
