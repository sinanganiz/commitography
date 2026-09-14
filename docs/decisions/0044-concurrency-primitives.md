# ADR-0044: Owned goroutines, context-bound subprocesses, persistent locks

**Status:** Accepted

## Context
ADR-0027 defines the queue policy but not its implementation. The failure modes
that matter here are quiet: leaked goroutines, subprocesses that survive
cancellation, and a repository lock that vanishes on crash while a checkpoint is
mid-write.

## Decision
1. Every goroutine MUST have an owner that waits for its completion. A bare `go`
   statement MUST be rejected by a lint rule; wait groups or error groups are
   required.
2. Every goroutine MUST be cancellable through a context derived from its
   owner's.
3. Every git subprocess MUST be started with a context and MUST be terminated
   when that context is cancelled. A cancelled analysis MUST NOT leave running
   subprocesses.
4. Subprocess termination MUST target the process group, not only the direct
   child, so that descendants are not orphaned.
5. The per-repository lock required by ADR-0027 clause 4 MUST be held in
   persistent storage, not only in memory. A stale lock whose owning process is
   no longer live MUST be reclaimed after a timeout.
6. Cancellation MUST leave the replay checkpoint in a valid state: either
   complete at the last fully processed commit, or discarded. A partially
   written checkpoint MUST NOT be readable as valid.
7. Goroutine leak detection MUST be part of the test suite.

## Consequences
- Cancellation is real rather than cosmetic.
- A crash cannot produce concurrent analyses of one repository after restart.
- Checkpoint corruption through interruption is structurally prevented.

## Out of scope
- Actor frameworks, distributed coordination, external queues.

## Assumption
Process-group termination is available on all supported platforms; where
semantics differ, the platform-specific implementation lives behind one
interface.

## Acceptance criteria
- Lint rejects bare `go` statements.
- Cancelling a running analysis leaves no git process alive.
- Killing the process mid-analysis and restarting produces either a valid resume
  or a full rebuild, verified by the incremental-equals-full invariant.
- Leak detection reports no residual goroutines after the suite.

## Dependencies
ADR-0027 must be implemented before this one.
