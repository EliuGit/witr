package output

import (
	"os"

	"github.com/charmbracelet/colorprofile"
)

// The CLI palette.
//
// The bright codes (90-97) are an extension to ANSI — "aixterm" colors, not
// part of the base 16. A terminal that advertises only 16 colors ignores them
// and renders the output with no color at all, which is what a plain
// TERM=xterm does. Pick the palette from the detected color profile instead of
// assuming the extension is understood: 256 colors or better keeps the bright
// codes chosen for contrast on dark themes, anything less falls back to the
// standard codes.
var (
	ColorReset     = ansiString("\033[0m")
	ColorRed       ansiString
	ColorGreen     ansiString
	ColorBlue      ansiString
	ColorCyan      ansiString
	ColorMagenta   ansiString
	ColorDim       ansiString
	ColorDimYellow ansiString
)

func init() {
	ColorRed, ColorGreen, ColorBlue, ColorCyan, ColorMagenta, ColorDim, ColorDimYellow =
		palette(colorprofile.Env(os.Environ()))
}

// palette returns the foreground codes to use for a terminal with the given
// color profile. Only terminals with 256 colors or better are assumed to
// understand the bright codes; the rest get the standard 30-37 foregrounds,
// with "dim" expressed as faint rather than bright black.
//
// Env-based detection is deliberate: it reads $TERM, $COLORTERM, $NO_COLOR and
// $CLICOLOR* only, with no terminfo file lookup, so it works identically on
// every platform the tool ships for.
func palette(p colorprofile.Profile) (
	red, green, blue, cyan, magenta, dim, dimYellow ansiString,
) {
	if p >= colorprofile.ANSI256 {
		return "\033[91m", "\033[92m", "\033[94m", "\033[96m", "\033[95m", "\033[90m", "\033[93m"
	}
	return "\033[31m", "\033[32m", "\033[34m", "\033[36m", "\033[35m", "\033[2m", "\033[2;33m"
}
