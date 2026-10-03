package proc

import (
	"fmt"

	"github.com/pranshuparmar/witr/pkg/model"
)

func ResolveAncestry(pid int) ([]model.Process, error) {
	return resolveAncestry(pid, ReadProcess)
}

func resolveAncestry(pid int, readProcess func(int) (model.Process, error)) ([]model.Process, error) {
	var chain []model.Process
	seen := make(map[int]bool)

	current := pid

	for current > 0 {
		if seen[current] {
			break // loop protection
		}
		seen[current] = true

		p, err := readProcess(current)
		if err != nil {
			// The parent the child recorded no longer exists.
			if len(chain) > 0 {
				chain[len(chain)-1].ParentExited = true
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
				child.ParentExited = true
				break
			}
			if adopted(*child, p, readProcess) {
				child.ParentExited = true
			}
		}

		chain = append(chain, p)

		if p.PPID == 0 || p.PID == 1 {
			break
		}
		current = p.PPID
	}

	if len(chain) == 0 {
		return nil, fmt.Errorf("no process ancestry found")
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
// subreaper) after the process that started it exited. An orphan keeps its
// session, so the tell is a parent from another session while the child's
// session leader, normally the shell that launched it, is gone or its PID now
// names a newer process. A session leader's own parent is legitimately in
// another session, so leaders are never treated as adopted.
func adopted(child, parent model.Process, readProcess func(int) (model.Process, error)) bool {
	if child.Session <= 0 || child.Session == child.PID || parent.Session <= 0 || parent.Session == child.Session {
		return false
	}
	leader, err := readProcess(child.Session)
	return err != nil || startedAfter(leader, child)
}
