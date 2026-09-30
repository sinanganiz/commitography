# WP-0026: hotspot family

**Area:** metrics
**Implements:** ADR-0076, ADR-0053, ADR-0062, ADR-0031, ADR-0032
**Requires:** WP-0022, WP-0062

## Goal
The `hotspot` family computes every metric of `docs/metrics.md` section 10,
its complexity proxy defined precisely over the indentation inputs replay
records at the analysed commit, and reads every section 1 value from the commit
record.

## In scope
1. **Read section 1 values from the record** (carried forward from WP-0012):
   `FileChange.Excluded`, `IsBulk`, `LocalTime` and `ActiveDate`. The family
   stops reading `core.ScopedCommits`. With `coupling` no longer reading it
   either (WP-0022), remove `core.ScopedCommits` and `internal/core/scoped.go`.
2. **Define the complexity proxy** over replay's `Indentation` entries
   (WP-0062), replacing steps 1 to 3 of section 10:
   - the indent unit `u` is the most common value among the file's positive
     increments in leading spaces between consecutive non-blank lines, the
     smaller value on a tie, and 1 where there is none;
   - a non-blank line with `t` leading tabs and `s` leading spaces has depth
     `min(t + s / u, 20)`: a tab is one unit whatever `u` is;
   - `complexity_proxy` is the mean depth over the file's non-blank lines times
     the base-2 logarithm of their number, and 0 for a file with none.

   Replay recorded the inputs and applied no unit and no cap (WP-0062), so a
   change to this definition re-runs aggregation only.
3. **The files** are the tracked text files replay recorded indentation for.
   A file of the map with no lines, because its content at the analysed commit
   was over the single-file-size limit, has no entry, and degrades the family
   with `limit_reached_size`, confidence `partial`. Where replay
   holds no map, the scores are absent, `churn_files` is computed, and the
   family is degraded with `history_incomplete` (WP-0023), confidence
   `partial`.
4. **Compute every metric of section 10.** Write these definitions into it:
   - `change_frequency` counts the analysed, non-bulk commits with a change
     naming the file's path; a rename is not followed;
   - each of `change_frequency` and `complexity_proxy` is min-max normalised
     across the files of clause 3, to 0 for every file where the maximum
     equals the minimum; `hotspot_score` is the product of the two;
   - `hotspots` lists at most 100 files, by `hotspot_score` descending, then
     path, each with `path` and the three values.
   A list cut at its limit degrades the family with `cardinality_limit`.
5. **Values carry 6 significant digits** through `core.RoundSignificant`: the
   proxy, the normalised values and the score. Add the function with a unit
   test if it is not there.
6. The family's declaration states, as its method, that the proxy measures
   indentation and is not a measurement of quality (section 10, ADR-0032
   clause 8).
7. **Correct section 11**, whose sentence says the static-analysis family
   declares `worktree`: it declares replay state (ADR-0076 clause 6).
8. `hotspot` carries no cells; its `Metric` table gains no `Scope` column.
9. Add the metric types to `internal/core/metrics.go` and
   `docs/report-schema.json`.
10. **Increment the family version** (ADR-0031 clause 2): its minor component,
    to 1.1, if `churn_files` did not change in any golden file; its major
    component, to 2.0, if it did.
11. Regenerate every golden file in one commit whose body names each cause.
12. Every test this package adds is named with the prefix `TestHotspot`. A unit
    test holds the proxy to hand-computed values for a tab-indented file, a
    file indented by four spaces, a file mixing both, a file of one line, and a
    file whose depth reaches the cap.

## Out of scope
- Parsing any language, or any measure of code quality.
- Following renames through history.
- Person-scoped hotspot figures, including the `owns_top_hotspot` badge axis,
  which WP-0028 decides.

## Files
**May create or modify:** `internal/metrics/hotspot/**`,
`internal/core/metrics.go`, `internal/core/scoped.go` **to remove it**,
`internal/core/stats.go`, `internal/core/stats_test.go`,
`internal/pipeline/aggregate/registry.go`, `internal/checks/**` **only where a test
names or calls the hotspot family**, `docs/metrics.md` **sections 10 and 11
only**, `docs/report-schema.json`, `testdata/golden/**`.
**Must not touch:** every other package under `internal/metrics/`,
`internal/pipeline/collect/**`, `internal/pipeline/replay/**`, `cmd/**`,
`docs/decisions/**`, and every other section of `docs/metrics.md`.

## Steps
1. Read the record's values for `churn_files`; run the golden comparison:
   nothing may differ. A difference means the record and the recomputation
   disagree; stop and report it.
2. Remove `core.ScopedCommits`.
3. Implement the proxy with its unit test, then the remaining metrics.
4. Edit sections 10 and 11, the core types and the schema.
5. Increment the version and regenerate the golden files in one commit.

## Definition of done
- On the fixture `basic`, every metric section 10 defines is present.
- The proxy's unit test holds the five hand-computed cases.
- `internal/core/scoped.go` does not exist, and nothing refers to
  `ScopedCommits`.
- Section 11 does not mention the working tree.
- The family's version is 1.1 where `churn_files` did not move, and 2.0
  otherwise, and the golden commit body says which applied.

## Verification
```
make gate-full
go test ./internal/metrics/hotspot/...
go test ./internal/checks -run '^(TestMetricCatalogue|TestReportSchema|TestGolden|TestHotspot)'
git grep -n 'ScopedCommits' -- internal
# no output
git grep -n -i 'worktree\|working tree' -- docs/metrics.md
# no output
```
