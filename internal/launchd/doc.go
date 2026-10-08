// Package launchd parses launchd job plists, which only exist on macOS. This
// file has no build constraint so the package builds on every platform, as
// tools that test each package directory by name (such as Nix's
// buildGoModule) expect.
package launchd
