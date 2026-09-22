# WP-0013: Replay stage and ownership map

**Area:** pipeline
**Implements:** ADR-0020, ADR-0051, ADR-0072, ADR-0073, ADR-0033, ADR-0019, ADR-0052
**Requires:** WP-0012

## Goal
Replay walks the commit graph in topological order, derives each commit's line
ownership from its parents' states, reads file contents from git objects through
a streaming length-framed reader, computes line differences in-process, reads
nothing from the working tree, and closes the recorded deviation that has the
aggregation stage listing files and reading the working tree.

## Why this package was rewritten

The first version assumed a chronological walk over one running state and
assumed line positions were available. Neither held: the collect records carry
per-file counts, not line positions, and a single running state gives wrong
line positions on branched history. ADR-0072 and ADR-0073 decide both.

## In scope
1. **Add a streaming length-framed reader to the git package** (ADR-0072). It
   reads object contents by identifier, caps each record at the single-file-size
   limit of ADR-0048, and marks an oversized file `degraded` without failing the
   run. **A header that does not parse aborts the read as an internal error; the
   reader never resynchronises.**
2. **Add the old and new blob identifiers of every changed file to the collect
   records**, in collect's existing single pass. Replay then needs no second walk
   over history, only object reads. For merges, the records carry the blob
   identifier of each parent's version, which clause 5 needs.
3. Walk commits in **topological order**, deriving each commit's state from its
   parents (ADR-0073 clauses 1 and 2). States share unchanged files; a commit
   allocates new ownership only for the files it changes. Release a state when
   its last child has been processed.
4. Store ownership compactly (ADR-0051): the owner as an **integer index into an
   identity table**, the authoring time at **day resolution**, lines in
   contiguous per-file slices.
5. **Merges** follow ADR-0073 clause 5: start from the first parent's state,
   apply the difference from the first parent, and let a line that already
   exists unchanged in another parent's version inherit that parent's owner.
   Only a line matching no parent is owned by the merge author.
6. Replay traverses merges **whatever the merge-counting setting** (ADR-0073
   clause 6).
7. Compute line differences **in-process with one fixed algorithm**, implemented
   in this repository with no new dependency, and independent of git's diff
   configuration (ADR-0073 clause 7).
8. **Read nothing from the working tree.** Tracked text files at the analysed
   commit come from that commit's tree through git, with binary detection by
   git's rule and attributes from the analysed commit, consistent with WP-0012
   clause 10c.
9. **Close the recorded deviation.** Remove the tracked-file listing and the
   working-tree binary check from the aggregation stage, and remove the entry
   from the linter's tracking list **in the same change**. There is no blame to
   move: WP-0008 already removed the sampled blame code age. Correct the
   tracking entry's wording as it is removed.
10. **Replay is sequential**; the walk starts no goroutine (ADR-0052 clause 2).
11. Measure **per-line memory** and **peak retained states** against a fixture,
    and record both under ADR-0050 clause 3.
12. Measure **divergence from `git blame`** on the renames-and-copies fixture
    and on a branched fixture with a merged side branch, and assert each below a
    threshold recorded in the test (ADR-0019 clause 6). Set the method on the
    ownership family, which stays `skipped` until WP-0023, the same way the
    pipeline root already sets a method on a skipped family.
13. The serialised map carries **no raw address**, as a consequence of the
    identity index. Add a checker.
14. Every test this package adds is named with the prefix `TestReplay`, so that
    its verification command matches nothing else.

## Out of scope
- **Code age as a metric**, which belongs to the ownership family (WP-0023).
  This package proves the map covers every tracked text line; the family turns
  it into figures.
- Work-type classification (WP-0014).
- Checkpoint persistence and incremental resume (WP-0034).
- Spilling cold per-file state to disk (ADR-0051 clause 5), unless the memory
  measurement in clause 11 shows the ceiling is reached on a supported fixture.
- Whether `worktree` remains a distinct input kind (ADR-0073, out of scope).
- Raising the minimum supported git version.

## Files
**May create or modify:** `internal/pipeline/replay/**`, `internal/git/**` **for
the length-framed reader only**, `internal/pipeline/collect/**` **for adding blob
identifiers to the records only**, `internal/pipeline/aggregate/**` **for
removing the moved code only**, `internal/pipeline/run.go`, `internal/core/**`,
`internal/checks/**`, `.golangci.yml` **for the tracking list entry only**,
`testdata/**` fixtures, the fixture manifest, and golden files.
**Must not touch:** `internal/metrics/**`, `internal/pipeline/interpret/**`,
`internal/pipeline/render/**`, `cmd/**`, `docs/decisions/**`, `docs/metrics.md`.

## Steps
1. Build the length-framed reader with its forged-header test, and observe that
   test failing against a reader that resynchronises.
2. Add blob identifiers to the collect records; confirm the report is unchanged,
   since the records are internal.
3. Implement the in-process line difference against small hand-written cases.
4. Build the topological walk on a linear fixture first, then the branched
   fixture.
5. Add the merge rule, and a fixture where a side-branch author's lines and a
   conflict resolution each have a known expected owner.
6. Remove the moved code from aggregation and the tracking entry.
7. Add the memory, retained-state, blame-divergence and no-raw-address
   measurements.

## Definition of done
- On a fixture with a merged side branch, every line written on the side branch
  is owned by its side-branch author.
- A line matching no parent is owned by the merge commit's author.
- Replay reads no file from the working tree, and completes on a bare clone.
- A forged length header aborts the read with an internal error.
- A blob over the cap marks its file `degraded` and the analysis completes.
- The aggregation stage neither lists tracked files nor reads the working tree,
  and the tracking entry is gone.
- Per-line memory and peak retained states are measured and within recorded
  budgets.
- Divergence from blame is below its recorded threshold on both fixtures.
- The serialised map contains no raw address.
- The walk starts no goroutine.
- The map covers every tracked text line at the analysed commit.
- The report is unchanged apart from the method statement on the ownership
  family, and that change is stated in the commit body.

## Verification
```
make gate-full
go test ./internal/checks ./internal/pipeline/replay ./internal/git -run '^TestReplay'
git grep -n 'ls-tree\|looksBinary' -- internal/pipeline/aggregate   # no output
grep -n 'WP-0013' .golangci.yml                                     # no output
```
