# ADR-0075: The files family declares replay state as well as commit records

**Status:** Accepted
**Note:** This record corrects one row of ADR-0024 clause 5. It supersedes it in
no other part.

## Context
ADR-0024 clause 5 declares the `files` family's inputs as commit records alone.
Several of its metrics describe the tree at the analysed commit rather than the
changes leading to it: the file count at that commit, the tracked text file
count, and the extension distribution by file count. Commit records describe
changes, not a tree.

Since ADR-0073 clause 8, the tracked text files at the analysed commit are
derived by replay from that commit's tree. The declaration was therefore wrong
before the aggregate stage began resolving declared inputs, and would have
silently stripped those metrics the moment it did.

## Decision
1. The `files` family declares **`commit-records` and `replay-state`**.
2. Every other row of ADR-0024 clause 5 stands unchanged.
3. Whether `worktree` remains a distinct input kind, now that replay reads
   content from git objects, is still open (ADR-0073, out of scope) and is
   decided when the hotspot family is rebuilt.
4. **Resolving declared inputs must not change any family's status.** If it
   does, an input declaration is wrong and the change stops there rather than
   being absorbed into a golden update.

## Consequences
- The files family keeps the metrics `docs/metrics.md` section 5 defines.
- Clause 4 turns the input table from a description into something the aggregate
  stage verifies: a wrong declaration shows up as a status change, not as a
  quietly emptier report.

## Out of scope
- The `worktree` input kind.
- Adding an input kind. The set in ADR-0024 clause 1 stays at four.

## Assumption
Replay state carries the analysed commit's tracked text file list in a form the
files family can consume. WP-0013 clause 8 produces it.

## Acceptance criteria
- The files family declares both inputs.
- Resolving declared inputs leaves every family's status as it was.

## Dependencies
ADR-0024 and ADR-0073 must be implemented before this one.
