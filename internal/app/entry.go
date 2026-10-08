//go:build linux || darwin || freebsd || windows

package app

import appversion "github.com/pranshuparmar/witr/internal/version"

// Main runs the CLI with the build-time version metadata injected via ldflags.
// Both entry points — the module root (main.go) and cmd/witr — call this, so
// their behavior cannot drift apart.
func Main() {
	SetVersion(appversion.Version, appversion.Commit, appversion.BuildDate)
	Execute()
}
