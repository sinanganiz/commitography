# WP-0012: Collect stage

**Area:** pipeline
**Implements:** ADR-0020, ADR-0007, ADR-0052, ADR-0062, ADR-0017
**Requires:** WP-0009, WP-0010, WP-0011

## Goal
The collect stage reads history in a single pass through the git package,
produces normalized commit records carrying every per-commit definition in
`docs/metrics.md` section 1, writes them as an independently cacheable
artifact, splits reading across concurrent readers without changing its output,
and computes no metric.

## In scope
1. Every definition in `docs/metrics.md` section 1 that is a property of a
   commit is computed **here and only here**: analysed-commit membership,
   excluded paths, effective lines, the bulk commit flag, author local time and
   the active date. A family receives them; no family recomputes them.
2. The stage computes **no metric** (ADR-0020 clause 2). A checker asserts that
   collect writes nothing into any family namespace.
3. One diff per commit. A git invocation per commit is the pathology, and the
   subprocess count checker from WP-0011 covers it.
4. Above a commit-count threshold, split the commit list across concurrent
   readers and reassemble in order (ADR-0052 clause 1). The degree is
   configurable and defaults to a value derived from available cores, never a
   fixed number.
5. **Open the parallelism half of the determinism checker**: degrees 1, 2 and N
   produce identical commit records and identical reports (ADR-0052 clause 6).
6. Collect output is **independently cacheable** (ADR-0020 clause 2): writing
   it and reading it back round-trips exactly, and every later stage can run
   from it without reading the repository.
7. Date bounds come from the resolved instant WP-0010 embeds, not from the
   string the operator typed. The same embedded configuration selects the same
   commits at any hour.
8. **The collect artifact is internal working data.** It carries raw addresses,
   because identity resolution runs after it (ADR-0033 clause 1). Add a checker
   asserting that no output path, route or exported artifact reads it, so it
   cannot become an export by accident.
9. Records are NUL-delimited end to end. A file name or subject containing a
   newline must not split, merge or drop a record; assert it against the
   hostile-names fixture.
10. Binary detection follows section 1: a NUL byte within the first 8 000
    bytes. Rename detection is on, so the file family has a source for its
    rename count.

## Out of scope
- Reading the working tree, listing tracked files, or any blame. All three
  belong to replay (WP-0013), and the recorded deviation that has aggregate
  doing them is WP-0013's to close.
- Computing any metric, including one that looks like a by-product.
- The checkpoint and any persistence of the artifact beyond writing and reading
  it (WP-0033, WP-0034).
- Changing a metric definition. If a record produced here disagrees with
  section 1, the document is correct (ADR-0062 clause 3).

## Files
**May create or modify:** `internal/pipeline/collect/**`, `internal/core/model/**`,
`internal/core/filter/**`, `internal/pipeline/run.go`, `internal/checks/**`,
`testdata/**` golden files.
**Must not touch:** `internal/metrics/**`, `internal/pipeline/replay/**`,
`internal/pipeline/aggregate/**`, `internal/pipeline/render/**`,
`docs/decisions/**`, `docs/metrics.md`, `internal/pipeline/interpret/**`.

## Steps
1. Reconcile the existing records against section 1 one definition at a time,
   running the golden comparison after each.
2. Move any per-commit derivation that later stages repeat into the record.
3. Add the artifact round-trip and the no-metric checker.
4. Add concurrent reading behind the threshold, then the parallel determinism
   checker; observe it failing against a deliberately order-dependent
   reassembly, then revert.
5. Add the hostile-record and artifact-not-exported checkers.

## Definition of done
- Collect writes into no family namespace, asserted by a checker.
- Reports produced at parallelism degrees 1, 2 and N are byte-identical.
- Writing the collect artifact and reading it back yields identical records.
- Every later stage runs from the artifact with the repository unreadable.
- The git process count does not grow with commit count.
- The hostile-names fixture yields the expected record count, with names intact.
- No route, output path or exported artifact reads the collect artifact.
- A golden change, if any, states in the commit body which section 1 definition
  moved into the record.

## Verification
```
make gate-full
go test ./internal/checks -run 'Determinism|Parallel|CollectArtifact|Hostile'
go test ./internal/checks -run SubprocessCount
```
