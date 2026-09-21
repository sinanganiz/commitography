# WP-0012: Collect stage

**Area:** pipeline
**Implements:** ADR-0020, ADR-0007, ADR-0052, ADR-0062, ADR-0017, ADR-0071, ADR-0031
**Requires:** WP-0009, WP-0010, WP-0011

## Goal
The collect stage reads history in a single pass through the git package,
produces normalized commit records carrying every per-commit definition in
`docs/metrics.md` section 1, writes them as an independently cacheable
artifact, splits reading across concurrent readers without changing its output,
and computes no metric.

## In scope
1. **The record carries every per-commit definition in `docs/metrics.md`
   section 1**: analysed-commit membership, excluded paths, effective lines,
   the bulk commit flag, author local time and the active date. Switching each
   family to read these fields rather than recompute them is **not** this
   package's work; each family's rebuild in WP-0018 to WP-0027 does it.
2. The stage computes **no metric** (ADR-0020 clause 2). A checker asserts that
   collect writes nothing into any family namespace.
3. One diff per commit. A git invocation per commit is the pathology, and the
   subprocess count checker from WP-0011 covers it.
4. Above a commit-count threshold, split the commit list across concurrent
   readers and reassemble in order (ADR-0052 clause 1). The degree is set
   through the pipeline options, so that both the command and the server can
   set it, and defaults to a value derived from available cores, never a fixed
   number. An operator-facing flag for it is WP-0017's concern.
5. **Open the parallelism half of the determinism checker**: degrees 1, 2 and N
   produce identical commit records and identical reports (ADR-0052 clause 6).
6. Collect output is **independently cacheable** (ADR-0020 clause 2): writing
   it and reading it back round-trips exactly, and a consumer that needs only
   commit records runs from it without reading the repository. Proving that the
   **whole** aggregate stage runs with the repository removed is WP-0061's,
   because the aggregation stage still lists the tree and reads the working
   tree until WP-0013 closes that deviation.
7. Date bounds come from the resolved instant WP-0010 embeds, not from the
   string the operator typed. The same embedded configuration selects the same
   commits at any hour.
8. **The collect artifact is internal working data** and carries raw addresses
   (ADR-0033 clause 1). Author exclusion is decided per resolved identity, so
   **the resolver runs inside collect to decide membership**, while the artifact
   keeps the raw values. The artifact therefore depends on the analysis
   configuration, not only on the repository: record in the collect package
   that any cache of it must be keyed on the configuration digest as well
   (WP-0033). Add a checker asserting that no output path, route or exported
   artifact reads it.
9. Records are NUL-delimited end to end. A file name or subject containing a
   newline must not split, merge or drop a record; assert it against the
   hostile-names fixture.
10. **Turn rename detection on.** A file moved without content change is zero
    effective lines, not its full length removed and added; the current
    behaviour inflates every metric that counts lines or touches. Add one
    sentence to `docs/metrics.md` section 1 stating how a rename counts toward
    effective lines. This moves at least the commit-size and files families on
    the renames fixture: **increment the version of every family whose golden
    output moves**, and state in the commit body that rename detection changed
    effective lines (ADR-0019 clause 2, ADR-0031 clause 2).
10a. **Parse rename entries by position.** In NUL-delimited output a rename is
    an empty path field followed by two path records. Take those two records as
    paths because of where they sit, never because of what they look like: a
    hostile path can be crafted to resemble a commit header (ADR-0045).
10b. **Apply ADR-0071 in the git package**: rename detection and its limit, the
    diff algorithm, path quoting and the global attributes file are pinned on
    every invocation, centrally. Without this, an operator's personal git
    configuration changes the numbers. Add the hostile global configuration
    test from ADR-0071 clause 5 and observe it failing before the pins exist.
10c. **Read `.gitattributes` from the analysed commit through git, never from
    the working tree.** Attributes are repository content as of the analysed
    commit; a working-tree copy can carry uncommitted changes, and ADR-0020
    clause 3 reserves working-tree access for replay. Binary detection follows
    git's rule, which a repository attribute may override; add that to the
    section 1 sentence on binary detection, and state it in the method field.

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
`internal/core/filter/**`, `internal/pipeline/run.go`,
`internal/pipeline/analysis.go` **for the parallelism option only**,
`internal/git/**` **for the pinned configuration set only**,
`internal/checks/**`, `testdata/**` golden files, `docs/metrics.md` **section 1
only**, and the version constant of any family under `internal/metrics/`
**whose golden output moves, and nothing else in that package**.
**Must not touch:** any other code under `internal/metrics/**`,
`internal/pipeline/replay/**`, `internal/pipeline/aggregate/**`,
`internal/pipeline/render/**`, `internal/pipeline/interpret/**`, `cmd/**`,
`docs/decisions/**`, any section of `docs/metrics.md` other than section 1.

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
- A consumer of commit records alone runs from the artifact with the
  repository unreadable.
- Rename detection is on; a pure move counts zero effective lines; section 1
  says so; every family whose golden moved has a higher version.
- An analysis under the hostile global configuration of ADR-0071 clause 5
  produces a byte-identical report.
- A path crafted to resemble a commit header, used as a rename source, parses
  as a path.
- Collect reads no file from the working tree.
- The git process count does not grow with commit count.
- The hostile-names fixture yields the expected record count, with names intact.
- No route, output path or exported artifact reads the collect artifact.
- A golden change, if any, states in the commit body which section 1 definition
  moved into the record.

## Verification
```
make gate-full
go test ./internal/checks -run 'Determinism|Parallel|CollectArtifact|Hostile|PinnedConfig' \
  -skip '^TestFixtureDeterminism$'
go test ./internal/checks -run SubprocessCount
```
