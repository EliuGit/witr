package proc

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/pranshuparmar/witr/pkg/model"
)

func ResolveAncestry(pid int) ([]model.Process, error) {
	return resolveAncestry(pid, ReadProcess, processGone)
}

// resolveAncestry walks from pid up through its parents. gone reports whether
// a PID that couldn't be read no longer exists at all.
func resolveAncestry(pid int, readProcess func(int) (model.Process, error), gone func(int) bool) ([]model.Process, error) {
	var chain []model.Process
	var readErr error
	seen := make(map[int]bool)

	current := pid

	for current > 0 {
		if seen[current] {
			break // loop protection
		}
		seen[current] = true

		p, err := readProcess(current)
		if err != nil {
			if len(chain) == 0 {
				readErr = err
			}
			// The walk ends here either way, but only a parent that no longer
			// exists means the one that started the child has exited. One that
			// exists but can't be read (hidden /proc, another jail) does not.
			if len(chain) > 0 && gone(current) {
				markParentExited(&chain[len(chain)-1])
			}
			break
		}

		if len(chain) > 0 {
			child := &chain[len(chain)-1]
			// A real parent must have started no later than its child. If the
			// process currently occupying the PPID is newer, the original
			// parent exited and its PID was recycled while (or before) we
			// walked the chain. Stop here instead of stitching an unrelated
			// process onto the ancestry. Some platforms can leave start times
			// unavailable, so only enforce the invariant when both are known.
			if startedAfter(p, *child) {
				markParentExited(child)
				break
			}
			if adopted(*child, p) {
				markParentExited(child)
			}
		}

		chain = append(chain, p)

		if p.PPID == 0 || p.PID == 1 {
			break
		}
		current = p.PPID
	}

	if len(chain) == 0 {
		if pid > 0 && !gone(pid) {
			return nil, fmt.Errorf("process %d exists but can't be read (insufficient permissions): %w", pid, readErr)
		}
		return nil, fmt.Errorf("process %d does not exist", pid)
	}

	// Reverse the chain to get root
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	return chain, nil
}

// startedAfter reports whether a started after b, when both start times are
// known.
func startedAfter(a, b model.Process) bool {
	return !a.StartedAt.IsZero() && !b.StartedAt.IsZero() && a.StartedAt.After(b.StartedAt)
}

// adopted reports whether child was re-parented to parent (init or a
// subreaper) after the process that started it exited. A forked process
// inherits its parent's session, so a parent from another session means the
// original parent is gone, unless the child started a session of its own (a
// session leader, such as a service or a deliberately detached daemon).
func adopted(child, parent model.Process) bool {
	return child.Session > 0 && child.Session != child.PID &&
		parent.Session > 0 && parent.Session != child.Session
}

// windowsSessionProcesses are started through launchers that always exit
// (smss.exe session instances, userinit.exe), so a missing parent is normal.
var windowsSessionProcesses = map[string]bool{
	"wininit.exe":  true,
	"csrss.exe":    true,
	"winlogon.exe": true,
	"explorer.exe": true,
}

// markParentExited records that the process that started p has exited,
// except where the OS launches p that way by design.
func markParentExited(p *model.Process) {
	if runtime.GOOS == "windows" && windowsSessionProcesses[strings.ToLower(p.Command)] {
		return
	}
	p.ParentExited = true
}
