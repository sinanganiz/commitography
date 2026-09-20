// Process-group termination (ADR-0044 clause 4). Cancelling an invocation
// must leave nothing running, and os/exec's own cancellation kills the direct
// child only: a descendant git started — a pager, a credential helper, an
// external diff driver a repository configured — would be orphaned.
//
// Platform semantics differ, so ADR-0044's assumption applies: the two
// implementations live behind the one interface below, in group_unix.go and
// group_windows.go.

package git

import "os/exec"

// group owns the process group of one invocation.
type group interface {
	// prepare configures the command before it starts, so the child lands in
	// a group of its own rather than in this process's.
	prepare(cmd *exec.Cmd)

	// adopt is called once the child is running, for the platforms that can
	// only place a live process into its group.
	adopt(cmd *exec.Cmd)

	// terminate kills the whole group. It is called on cancellation and on
	// Close, and must be safe to call when the process has already exited.
	terminate(cmd *exec.Cmd) error
}
