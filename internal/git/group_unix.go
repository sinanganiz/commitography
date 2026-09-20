//go:build !windows

// The Unix half of ADR-0044 clause 4. The child is made the leader of a new
// process group, and termination signals the negated group identifier, which
// reaches every descendant that has not left the group.

package git

import (
	"os/exec"
	"syscall"
)

type processGroup struct{}

func newGroup() group { return processGroup{} }

// prepare puts the child in a process group of its own, whose identifier is
// the child's own process identifier.
func (processGroup) prepare(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// adopt has nothing to do: the group exists from the moment the child does.
func (processGroup) adopt(*exec.Cmd) {}

// terminate signals the whole group. A negative identifier is what makes the
// signal reach the group rather than the leader alone.
func (processGroup) terminate(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		// The group may already be gone, in which case the direct child is
		// the only thing left to try.
		return cmd.Process.Kill()
	}
	return nil
}
