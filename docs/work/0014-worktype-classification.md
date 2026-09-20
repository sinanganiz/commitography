# WP-0014: Work-type classification

**Area:** pipeline
**Implements:** ADR-0020, ADR-0018, ADR-0019
**Requires:** WP-0013

## Goal
Every changed line in the analysed population is classified into one of four
work types during the replay walk, with no blame invocation, and the result is
stored as the editor-by-previous-owner breakdown that makes an identity merge a
projection rather than a recomputation.

## In scope
1. Classify during the replay walk by consulting the ownership map, using
   exactly the rules in ADR-0020 clause 4:
   - the line is not in the map → **new work**
   - the previous owner is the same identity and the line is newer than the
     window → **rework**
   - the previous owner is a different identity and the line is newer than the
     window → **help others**
   - the line is older than the window → **legacy refactor**
2. **No blame invocation anywhere in this path.** The map is the source.
3. The window is the `recency_window_days` value from WP-0010, defaulting to
   30 days, and is echoed in the report for reproducibility.
4. Store the result as the **two-dimensional breakdown** required by ADR-0018
   clause 1: editing identity by previous-owner identity, with class counts per
   cell.
5. Bound it as ADR-0018 clause 4 requires: the 200 identities with the most
   analysed commits individually, the remainder in one aggregate bucket, and
   the threshold recorded in the report.
6. Population: changed lines in analysed, non-bulk commits, on non-excluded
   paths.
7. Add the **projection invariant checker** (ADR-0063 table 2): projecting a
   merge of two identities equals full recomputation with those identities
   pre-merged, on a fixture where one author commits under two addresses.
   Observe it failing against a naive summation, then revert.
8. Person-scope shares are a projection over the breakdown and are never
   recomputed server-side (ADR-0018 clause 3).

## Out of scope
- The `worktype` family's report namespace and output shape, which is WP-0024.
  This package produces the classification in replay state.
- The reader's identity selection and multi-identity merge interface (WP-0051).
- Any other metric that later becomes identity-pair dependent. If one appears,
  it uses this representation (ADR-0018 clause 5), but it is not built here.

## Files
**May create or modify:** `internal/pipeline/replay/**`, `internal/core/**`,
`internal/checks/**`, `testdata/**` golden files and fixtures.
**Must not touch:** `internal/metrics/**`, `internal/pipeline/collect/**`,
`internal/pipeline/aggregate/**`, `docs/decisions/**`, `docs/metrics.md`.

## Steps
1. Add a fixture whose expected class for every changed line is known by
   construction: a line written and rewritten by its author inside the window,
   one rewritten outside it, one rewritten by another identity inside the
   window, and new lines.
2. Implement classification against that fixture.
3. Add the breakdown and its bound, with the threshold in the report.
4. Add the projection checker and observe it failing.
5. Add a fixture with one author under two addresses and prove projection
   equals pre-merged recomputation.

## Definition of done
- Every changed line in the purpose-built fixture receives its expected class.
- No blame invocation occurs in the classification path.
- The breakdown is bounded at 200 identities with the threshold recorded.
- Projecting a merge equals pre-merged recomputation, asserted by a checker
  that has been observed failing.
- The window defaults to 30 days, is configurable, and is echoed in the report.
- Class shares over a subject sum to 100 percent within rounding tolerance.

## Verification
```
make gate-full
go test ./internal/checks -run 'WorkType|IdentityProjection|Invariant'
git grep -n 'blame' -- internal/pipeline/replay    # only the divergence test
```
