# WP-0061: Aggregate stage

**Area:** pipeline
**Implements:** ADR-0020, ADR-0024, ADR-0052, ADR-0032
**Requires:** WP-0013, WP-0015

## Goal
The aggregate stage resolves each registered family's declared inputs, runs the
families that can run and skips the rest with a reason, runs them concurrently,
is stateless, and produces an identical report from cached collect output and
replay state with the repository unreadable.

## In scope
1. Resolve the inputs each registered family declares, and run it. A family
   whose inputs are unavailable is **skipped with the matching reason and never
   run** (ADR-0024 clause 2).
2. Run families **concurrently** (ADR-0052 clause 3). This is safe precisely
   because a family cannot read another family. The degree is configurable and
   defaults to a value derived from available cores.
3. The degree **never changes the output**. Extend the parallel determinism
   checker to cover family execution.
4. The stage is **stateless and re-runnable without touching git** (ADR-0020
   clause 5). Add the checker that proves it: run aggregate from cached collect
   output and replay state **with the repository directory removed**, and
   require a byte-identical report. Observe it failing against a family that
   reaches for the repository, then revert.
5. Write each family's output only into its namespace, with its status,
   version and reason.
6. Shared derived data needed by more than one family is produced by `core` or
   an earlier stage, never reached through a family (ADR-0024 clause 4).
7. Removing working tree availability skips `hotspot` and `static-analysis`
   with `worktree_unavailable` and changes no other family.

## Out of scope
- The interpret stage (WP-0016).
- Persisting collect output or the replay checkpoint (WP-0033, WP-0034). This
  package consumes them in memory or from whatever the pipeline root supplies.
- The correctness of any individual family against its catalogue definition
  (WP-0018 to WP-0027).
- Changing any metric value. Running a family through the stage must produce
  what it produced before, so golden files do not move.

## Files
**May create or modify:** `internal/pipeline/aggregate/**`,
`internal/pipeline/run.go`, `internal/core/**`, `internal/checks/**`.
**Must not touch:** `internal/metrics/**`, `internal/pipeline/collect/**`,
`internal/pipeline/replay/**`, `testdata/**` golden files,
`docs/decisions/**`, `docs/metrics.md`.

## Steps
1. Build input resolution and the skip path, with families running serially.
2. Add the no-repository checker and make it pass.
3. Add concurrent execution, then extend the determinism checker to the degree.
4. Prove the worktree case: remove worktree availability and confirm exactly
   two families change status.

## Definition of done
- Aggregate runs with the repository directory removed, from cached inputs, and
  produces a byte-identical report.
- Reports at parallelism degrees 1, 2 and N are byte-identical.
- No family writes outside its namespace.
- Removing worktree availability skips exactly `hotspot` and
  `static-analysis`, each with `worktree_unavailable`.
- **Every golden file is byte-identical to its state before this package.**

## Verification
```
make gate-full
go test ./internal/checks -run 'Aggregate|NoRepository|Parallel|FamilySkip'
git diff --stat <base> -- testdata/    # empty
```
