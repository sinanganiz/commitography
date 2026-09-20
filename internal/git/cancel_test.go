package git

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// TestCancellationLeavesNoProcessInTheGroup enforces ADR-0044 clause 4: a
// cancelled invocation leaves no descendant running, not merely no direct
// child.
//
// It is verified by asking the operating system whether each process is still
// there, rather than by the absence of an error, which is what the work
// package requires. The tree is two deep on purpose: the recording program is
// the direct child that os/exec's own cancellation would reach, and the git
// it starts is the descendant that would be orphaned without the group.
func TestCancellationLeavesNoProcessInTheGroup(t *testing.T) {
	t.Parallel()
	recorder := newRecorder(t)
	repo := repositoryRoot(t)

	// cat-file --batch reads object names from standard input and blocks
	// until it gets one, so the tree stays alive until it is terminated.
	stdin, hold, err := os.Pipe()
	if err != nil {
		t.Fatalf("opening the invocation's standard input: %v", err)
	}
	defer stdin.Close()
	defer hold.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	process, err := Start(ctx, recorder.spec(At(repo, "cat-file", "--batch").WithStdin(stdin)))
	if err != nil {
		t.Fatalf("starting the invocation: %v", err)
	}
	pids := waitForTheTree(t, recorder)

	cancel()
	process.Close()

	for _, pid := range pids {
		if waitForExit(pid) {
			continue
		}
		t.Errorf("ADR-0044 clause 4: process %d was still running %s after the invocation was "+
			"cancelled; termination must reach the group, not the direct child alone",
			pid, terminationGrace)
	}
}

// TestCancelledInvocationReportsTheContextError keeps cancellation from being
// reported as a git failure: the invocation did not fail, it was stopped.
func TestCancelledInvocationReportsTheContextError(t *testing.T) {
	t.Parallel()
	repo := repositoryRoot(t)
	stdin, hold, err := os.Pipe()
	if err != nil {
		t.Fatalf("opening the invocation's standard input: %v", err)
	}
	defer stdin.Close()
	defer hold.Close()

	ctx, cancel := context.WithCancel(context.Background())
	process, err := Start(ctx, At(repo, "cat-file", "--batch").WithStdin(stdin))
	if err != nil {
		t.Fatalf("starting the invocation: %v", err)
	}
	cancel()
	if err := process.Wait(); !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled invocation reported %v, want it to report the cancellation", err)
	}
	process.Close()
}

// TestTimeoutStopsAnInvocation covers the other half of "a context and a
// timeout" in ADR-0065 clause 2: an invocation that never finishes is stopped
// by its own bound, with no cancellation from the caller.
func TestTimeoutStopsAnInvocation(t *testing.T) {
	t.Parallel()
	repo := repositoryRoot(t)
	stdin, hold, err := os.Pipe()
	if err != nil {
		t.Fatalf("opening the invocation's standard input: %v", err)
	}
	defer stdin.Close()
	defer hold.Close()

	_, err = Output(context.Background(),
		At(repo, "cat-file", "--batch").WithStdin(stdin).WithTimeout(200*time.Millisecond))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("an invocation past its timeout reported %v, want the deadline", err)
	}
}

const (
	// pollInterval and maxPolls bound the two waits below by a count rather
	// than by a deadline, so no checker reads the clock (ADR-0042 clause 4).
	pollInterval = 10 * time.Millisecond
	maxPolls     = 1000

	// terminationGrace is what maxPolls amounts to, for the failure message.
	terminationGrace = time.Duration(maxPolls) * pollInterval
)

// waitForTheTree blocks until the recording program has started the real git,
// and returns both process identifiers.
func waitForTheTree(t *testing.T, r recorder) []int {
	t.Helper()
	for i := 0; i < maxPolls; i++ {
		// Two records: the recording program's own, and the git it started.
		if pids := r.processes(); len(pids) >= 2 {
			return pids
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("ADR-0064: the invocation never reached git, so the checker has nothing to cancel")
	return nil
}

// waitForExit reports whether a process is gone within the grace period.
func waitForExit(pid int) bool {
	for i := 0; i < maxPolls; i++ {
		if !processAlive(pid) {
			return true
		}
		time.Sleep(pollInterval)
	}
	return false
}
