# ADR-0027: Two job classes, interactive priority, and one active job per repository

**Status:** Accepted

## Context
Server mode runs scheduled re-analyses in the background while a reader may
trigger an analysis interactively. Without prioritisation, a long background
run on a large repository blocks interactive work. Independently, two analyses
of the same repository running at once would race on the replay checkpoint.

## Decision
1. Jobs MUST be classified as **interactive** (reader-triggered) or
   **background** (recurrence-triggered).
2. Interactive jobs MUST take priority. A running background job MUST be
   suspended or cancelled and requeued when an interactive job is waiting.
3. Worker count MUST be configurable in the operational plane.
4. At most one active job per repository MUST be permitted, enforced by a lock
   keyed on repository identity as defined in ADR-0017 clause 2. This
   constraint is independent of worker count and MUST NOT be disableable.
5. The queue MAY be held in memory. The **job record** — repository, analysed
   commit, resolved analysis configuration digest, outcome — MUST be persisted
   and MUST survive restart.
6. Retry policies, backoff and dead-letter handling MUST NOT be implemented.
   Jobs are idempotent by ADR-0017: an interrupted job resumes from its
   checkpoint on the next run, so a lost job is a repeated job, not a lost
   result.
7. Every job MUST be cancellable by the reader, and cancellation MUST leave the
   checkpoint in a valid state — either at the last fully processed commit or
   discarded.

## Consequences
- A large nightly re-analysis cannot make the interface unresponsive.
- Checkpoint corruption through concurrent writes is structurally prevented.
- Restart loses queue position but never loses analysis progress or history.

## Out of scope
- Distributed workers, external queue infrastructure, multi-node coordination.

## Assumption
A deployment serves one operator and tens of repositories.

## Acceptance criteria
- An interactive job starts while a background job is running on another
  repository, without waiting for it to finish.
- A second analysis request for a repository that already has an active job is
  refused or queued, never run concurrently.
- Cancelling a job mid-run leaves a checkpoint that either resumes correctly or
  is discarded, verified by the incremental-equals-full invariant.

## Dependencies
ADR-0017 must be implemented before this one.
