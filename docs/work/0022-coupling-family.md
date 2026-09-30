# WP-0022: coupling family

**Area:** metrics
**Implements:** ADR-0076, ADR-0053, ADR-0048, ADR-0062, ADR-0031, ADR-0032
**Requires:** WP-0061

## Goal
The `coupling` family computes every metric of `docs/metrics.md` section 6 from
commit records alone, bounds its graph in the report, reads section 1 values
from the record, and makes every limit it applies a catalogue rule that
degrades the family visibly.

## In scope
1. **Read section 1 values from the record** (carried forward from WP-0012):
   a path's exclusion from `FileChange.Excluded`, bulk from `IsBulk`. The family
   stops reading `core.ScopedCommits`, which recomputes exclusion through the
   path filter; leave `core.ScopedCommits` in place for `hotspot` until
   WP-0026. The population is the analysed, non-bulk commits.
2. **Write the counting rules into section 6's rule table**, as the code
   applies them and the table leaves open:
   - `changes(A)` counts the population's commits touching A, including those
     touching more than 50 non-excluded paths, which contribute no pair;
   - a path is counted under the name the commit gives it; a rename is not
     followed.
3. **The pair bound is a catalogue rule.** The code holds an undocumented
   bound, `couplingMaxPairs`, and past it discards pairs seen once, which
   lowers the support of pairs that would have qualified, while reporting it
   only as a diagnostic. That is silent truncation (ADR-0048) under a limit the
   catalogue does not define (ADR-0062 clause 6). Make it honest:
   - count candidate pairs only between paths that the population changes at
     least 5 times, the minimum support, since no other pair can qualify;
   - if the candidate pairs still exceed 5 000 000, discard pairs seen once as
     today, and degrade the family with `limit_reached_memory`, confidence
     `low`;
   - add both to section 6's rule table.
4. **Compute every metric of section 6**, adding `graph` and
   `coupled_file_ratio`. Write these definitions into section 6:
   - `graph` takes the reported pairs in their order, support descending, then
     confidence descending, then paths, and adds each pair's edge while doing
     so keeps the nodes at 150 or fewer and the edges at 400 or fewer. A node
     is a path with its `changes`; an edge is a pair with its support. Stopping
     short of the reported pairs degrades the family with `cardinality_limit`.
   - `coupled_file_ratio` is the share of paths the population changes that
     appear in at least one qualifying pair. The current definition divides by
     the files at the analysed commit, which is replay state, and `coupling`
     declares commit records alone (ADR-0076 clause 6). The metric has not been
     reported, so its redefinition changes no value.
5. **Ratios carry 6 significant digits**, `confidence` included, through
   `core.RoundSignificant`; add it to `internal/core/stats.go` with a unit test
   if it is not there.
6. `coupling` carries no cells: section 6 gives none of its metrics a person or
   year scope. Its `Metric` table gains no `Scope` column.
7. Add the metric types to `internal/core/metrics.go` and
   `docs/report-schema.json`.
8. **Increment the family version to 2.0**: existing `confidence` values change
   with the rounding.
9. Regenerate every golden file in one commit whose body names each cause.
10. Every test this package adds is named with the prefix `TestCoupling`. One
    of them builds a synthetic history whose candidate pairs exceed a bound
    lowered for the test, and requires the family degraded with
    `limit_reached_memory`.

## Out of scope
- Following renames, which would need the replay stage's view of paths.
- Any person-scoped coupling figure, including the `owns_top_coupled_pair`
  badge axis, which WP-0028 decides.
- Changing the thresholds of section 6.

## Files
**May create or modify:** `internal/metrics/coupling/**`,
`internal/core/metrics.go`, `internal/core/stats.go`,
`internal/core/stats_test.go`, `internal/pipeline/aggregate/registry.go`,
`internal/checks/**` **only where a test names or calls the coupling family**,
`docs/metrics.md` **section 6 only**, `docs/report-schema.json`,
`testdata/golden/**`.
**Must not touch:** every other package under `internal/metrics/`,
`internal/core/scoped.go`, `internal/pipeline/collect/**`,
`internal/pipeline/replay/**`, `cmd/**`, `docs/decisions/**`, and every other
section of `docs/metrics.md`.

## Steps
1. Read the record's values; run the golden comparison: nothing may differ.
   A difference means the record and the recomputation disagree; stop and
   report it.
2. Apply the candidate filter and the degrading bound.
3. Add `graph` and `coupled_file_ratio`.
4. Edit section 6, the core types and the schema.
5. Increment the version and regenerate the golden files in one commit.

## Definition of done
- On the fixture `coupling`, every metric section 6 defines is present.
- No constant in the family bounds anything section 6 does not state.
- The family is degraded with `limit_reached_memory` when the pair bound is
  reached, by the test of clause 10.
- The family's version is 2.0.
- No file of the family refers to `ScopedCommits`, `IncludedFiles` or
  `PathFilter`.

## Verification
```
make gate-full
go test ./internal/metrics/coupling/...
go test ./internal/checks -run '^(TestMetricCatalogue|TestReportSchema|TestGolden|TestReasonCode)'
git grep -n 'ScopedCommits\|IncludedFiles\|PathFilter' -- internal/metrics/coupling
# no output
```
