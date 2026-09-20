# WP-0013: Replay stage and ownership map

**Area:** pipeline
**Implements:** ADR-0020, ADR-0051, ADR-0033, ADR-0019, ADR-0052
**Requires:** WP-0012

## Goal
The replay stage walks analysed commits in chronological order, maintains a
compact per-file line ownership map, is the only stage that reads the working
tree, and closes the recorded deviation that has the aggregation stage listing
files and running blame.

## In scope
1. Walk analysed commits in chronological order, maintaining a per-file map
   from line to owning identity and authoring time.
2. Store it compactly (ADR-0051 clause 1): the owner as an **integer index into
   an identity table**, the authoring time at **day resolution**, lines in
   contiguous per-file slices. Day resolution is deliberate: the only consumer
   is the 30-day window in ADR-0020 clause 4, and finer resolution costs space
   and changes no metric.
3. Replay is **the only stage with working tree access** (ADR-0020 clause 3).
4. **Close the recorded deviation.** The aggregation stage currently lists
   tracked files and runs blame through git, which ADR-0020 clause 3 reserves
   for this stage. Move both here, and remove the entry from the linter's
   tracking list **in the same change**.
5. **No blame invocation remains in the ownership path.** Code age is computed
   over all tracked text lines from the map, never from a sample.
6. **Replay is sequential.** Parallelising the chronological walk is forbidden
   (ADR-0052 clause 2), because ownership state is order-dependent and a
   parallel walk produces wrong classification rather than a slower correct
   one. Add a checker asserting the walk starts no goroutine.
7. Measure **per-line memory** against a fixture and record it under ADR-0050
   clause 3. This is the value that sets the product's scale ceiling.
8. Measure **divergence from `git blame`** on the renames-and-copies fixture
   and assert it below a threshold recorded in the test (ADR-0019 clause 6).
   Replay tracks ownership through diffs and does not reproduce blame's copy
   and move detection. State the method in the owning family's method field
   (ADR-0032 clause 8).
9. The serialised map carries **no raw address**, which follows from the
   identity index rather than from a filter. Add a checker.

## Out of scope
- Work-type classification, which runs inside this walk but is WP-0014.
- Checkpoint persistence and incremental resume (WP-0034). Replay produces the
  map in memory and hands it on.
- Spilling cold per-file maps to disk (ADR-0051 clause 5). That is a fallback
  for when the memory ceiling is approached, not the default path, and it is
  not built until the measurement in clause 7 says it is needed.
- Computing any metric. The ownership family consumes this state and is
  WP-0023.

## Files
**May create or modify:** `internal/pipeline/replay/**`,
`internal/pipeline/aggregate/**` **for removing the moved code only**,
`internal/pipeline/run.go`, `internal/core/**`, `internal/checks/**`,
`.golangci.yml` **for the tracking list entry only**, `testdata/**` golden
files.
**Must not touch:** `internal/metrics/**`, `internal/pipeline/collect/**`,
`internal/pipeline/render/**`, `docs/decisions/**`, `docs/metrics.md`.

## Steps
1. Build the map and the walk against the fixtures, with no consumer yet.
2. Move the tracked-file listing into replay; run the golden comparison.
3. Move the blame-derived code age into replay, computed from the map; the
   golden will change, and the commit body states that code age is now complete
   rather than sampled.
4. Remove the tracking list entry.
5. Add the sequential-walk, no-raw-address and memory measurements.
6. Add the blame divergence measurement and record its threshold.

## Definition of done
- Replay produces complete line ownership at the analysed commit.
- No blame invocation remains in the ownership path.
- The aggregation stage neither lists tracked files nor runs blame, and the
  tracking list entry is gone.
- Per-line memory is measured and within a recorded budget.
- Divergence from blame is measured and below a recorded threshold.
- The serialised map contains no raw address.
- The chronological walk starts no goroutine.
- Code age covers all tracked text lines, and the report states the method.

## Verification
```
make gate-full
go test ./internal/checks -run 'Replay|Ownership|BlameDivergence|Sequential'
git grep -n 'blame' -- internal/pipeline/aggregate   # no output
grep -n 'WP-0013' .golangci.yml                       # no output
```
