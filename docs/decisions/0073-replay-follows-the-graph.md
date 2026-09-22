# ADR-0073: Replay follows the commit graph

**Status:** Accepted

## Context
ADR-0020 describes replay as a chronological walk maintaining one running
ownership state. That holds only for linear history. On branched history, a
commit's parent state differs from whatever the running state happens to be, so
applying its diff to the running state places lines wrongly: the result is not a
slower correct answer but a fast wrong one.

Most repositories that merge with merge commits are branched. Line ownership
feeds bus factor, knowledge concentration, work type and the persistence axis of
the archetypes, so this decides whether those figures are right. `git blame`
follows the graph and is the reference ADR-0019 clause 6 measures against.

## Decision
1. Replay processes commits in **topological order**, every parent before its
   children.
2. **Each commit's ownership state is derived from its parents' states**, never
   from a single running state.
3. States share unchanged files. A commit creates new ownership only for the
   files it changes; every other file is shared with its parent's state.
4. A commit's state is retained only while it has an unprocessed child, and is
   released when its last child has been processed.
5. **A merge** starts from its first parent's state and applies its difference
   from the first parent. A line that difference introduces which already
   exists, unchanged, in another parent's version of the same file **inherits
   that parent's owner**. Only a line that matches no parent is owned by the
   merge commit's author. This is how conflict resolution and evil merges come
   to be owned by the person who merged.
6. Replay traverses merge commits for ownership **whatever the merge-counting
   setting**. Work-type classification and every other metric population apply
   only to analysed commits, so a merge contributes to them only when merges are
   counted.
7. The line difference is computed **in-process by one fixed algorithm**,
   independent of git's diff configuration. Changing the algorithm is a change
   of meaning for every family that consumes replay state and increments each of
   their versions (ADR-0031 clause 2).
8. Replay reads file contents from git objects, never from the working tree.
   Tracked text files at the analysed commit are derived from that commit's
   tree. Replay therefore needs no working tree, and a bare repository is
   analysable.
9. Replay remains sequential (ADR-0052 clause 2). Topological order is a single
   ordered walk.

## Consequences
- Work written on a side branch is owned by the person who wrote it, not by the
  person who merged it.
- Memory scales with the number of simultaneously open branches, not with
  history length, because released states are freed and unchanged files are
  shared.
- Ownership becomes independent of the operator's git diff configuration.
- Analysis of a mirror or bare clone works, which the server mode's remote
  repositories are likely to be.

## Out of scope
- Whether `worktree` remains a distinct input kind under ADR-0024, now that
  replay reads content from git objects. That affects the hotspot family and is
  decided when that family is rebuilt.
- Incremental resume across branches, which ADR-0017 and the checkpoint package
  address.
- Following renames across a merge in ways blame's copy detection does. The
  divergence is measured, not eliminated.

## Assumption
The number of branches open at once stays small relative to history length in
typical repositories. A repository with many long-lived parallel branches costs
more memory, bounded by the ceiling of ADR-0048, which reports `degraded` rather
than truncating.

## Acceptance criteria
- On a fixture with a merged side branch, lines written on the side branch are
  owned by their side-branch author.
- A line matching no parent is owned by the merge commit's author.
- Divergence from blame on a branched fixture stays below a recorded threshold.
- Replay completes on a bare clone.
- Peak retained states are measured and recorded alongside per-line memory.

## Dependencies
ADR-0020, ADR-0051 and ADR-0072 must be implemented before this one.
