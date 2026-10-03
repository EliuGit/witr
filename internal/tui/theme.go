package tui

import (
	"os"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Terminal color support, detected with the same rule as the CLI palette
// (internal/output/colors.go). Terminals below 256 colors can ignore 256-color
// and 24-bit codes entirely, leaving the TUI with no color and no visible
// selection, so there the theme falls back to the 8 standard colors and the
// selection to reverse video.
var (
	colorProfile = colorprofile.Env(os.Environ())
	basicColors  = colorProfile < colorprofile.ANSI256
)

// pick returns full, or basic on terminals below 256 colors.
func pick(full, basic lipgloss.TerminalColor) lipgloss.TerminalColor {
	if basicColors {
		return basic
	}
	return full
}

// lipglossProfile maps the detected profile onto lipgloss. Anything below 256
// colors renders as ANSI, which the basic palette fits in; ASCII stays ASCII
// so NO_COLOR is honored.
func lipglossProfile(p colorprofile.Profile) termenv.Profile {
	switch p {
	case colorprofile.TrueColor:
		return termenv.TrueColor
	case colorprofile.ANSI256:
		return termenv.ANSI256
	case colorprofile.ASCII:
		return termenv.Ascii
	}
	return termenv.ANSI
}

// Theme palette.
//
// Foreground, text, and border colors are AdaptiveColor so the TUI stays
// readable on both light and dark terminals: lipgloss picks Light or Dark at
// render time from the detected background. The Dark values match the original
// scheme, so dark terminals look unchanged; the Light values are darker
// equivalents chosen for contrast on a light background.
//
// Colors that paint their own background (title bar, selected row, tabs) read
// the same on any terminal, so they stay fixed.
//
// The basic fallbacks use the standard colors 0-7 only, never the bright
// 8-15, and leave dim colors at the terminal default.
var (
	// Accent — table/pane headers, prompts, active borders.
	colorAccent = pick(lipgloss.AdaptiveColor{Light: "#4338ca", Dark: "#5f5fd7"}, lipgloss.Color("4"))
	// Inactive borders.
	colorBorderDim = pick(lipgloss.AdaptiveColor{Light: "#c2c2c2", Dark: "#585858"}, lipgloss.NoColor{})
	// Inactive panel header text.
	colorHeaderDim = pick(lipgloss.AdaptiveColor{Light: "#5b616e", Dark: "#bcbcbc"}, lipgloss.NoColor{})
	// Secondary / muted text (footer, placeholders, empty states).
	colorMuted = pick(lipgloss.AdaptiveColor{Light: "#6e6e6e", Dark: "#767676"}, lipgloss.NoColor{})
	// Error text.
	colorError = pick(lipgloss.AdaptiveColor{Light: "#c0271d", Dark: "#ff5f5f"}, lipgloss.Color("1"))
	// Action-menu text.
	colorAmber = pick(lipgloss.AdaptiveColor{Light: "#9a6700", Dark: "#ffdf87"}, lipgloss.Color("3"))
	// Confirmation prompt text.
	colorConfirm = pick(lipgloss.AdaptiveColor{Light: "#b45309", Dark: "#ffaf5f"}, lipgloss.Color("3"))
	// Ancestry-tree connectors.
	colorTreeConn = pick(lipgloss.AdaptiveColor{Light: "#9333ea", Dark: "#d787ff"}, lipgloss.Color("5"))
	// Ancestry-tree target node.
	colorTreeTarget = pick(lipgloss.AdaptiveColor{Light: "#15803d", Dark: "#00d700"}, lipgloss.Color("2"))
	// Section labels in the detail / tree panes.
	colorSectionLabel = pick(lipgloss.AdaptiveColor{Light: "#7c3aed", Dark: "#af87ff"}, lipgloss.Color("5"))

	// Fixed colors — painted over their own background, so they need no
	// adaptation to the terminal background. The basic selection colors are
	// unset: the selected styles switch to reverse video instead.
	colorBrandFg   = pick(lipgloss.Color("#FAFAFA"), lipgloss.Color("7"))
	colorBrandBg   = pick(lipgloss.Color("#7D56F4"), lipgloss.Color("5"))
	colorOnAccent  = pick(lipgloss.Color("#ffffff"), lipgloss.Color("0"))
	colorGreenBg   = pick(lipgloss.Color("#22aa22"), lipgloss.Color("2"))
	colorIdleTabBg = pick(lipgloss.Color("#767676"), lipgloss.Color("7"))
	colorSelectFg  = pick(lipgloss.Color("#ffffaf"), lipgloss.NoColor{})
	colorSelectBg  = pick(lipgloss.Color("#5f00d7"), lipgloss.NoColor{})
)
