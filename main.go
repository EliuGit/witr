//go:build linux || darwin || freebsd || windows

// The module root is an entry point of its own, so that
// `go install github.com/pranshuparmar/witr@latest` keeps working. It used to be
// a symlink to cmd/witr/main.go, but checkouts that cannot create symlinks — Git
// for Windows defaults to core.symlinks=false — materialize such an entry as a
// plain text file holding the link target, and then every package command at the
// module root fails with "expected 'package', found cmd":
//
//	go build ./...
//	go vet ./...
//	go test ./...
//
// Keep a real file so the repository builds from the root on every platform.
// Both entry points share app.Main, so their behavior cannot drift apart.
package main

import "github.com/pranshuparmar/witr/internal/app"

func main() {
	app.Main()
}
