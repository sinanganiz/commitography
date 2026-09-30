# WP-0018: temporal family

**Area:** metrics
**Implements:** ADR-0076, ADR-0078, ADR-0062, ADR-0031, ADR-0032
**Requires:** WP-0063

## Goal
The `temporal` family computes every metric of `docs/metrics.md` section 2 from
person-and-year cells, by one derivation that serves the repository, any
selection and any year, and reads every section 1 value from the commit record.

## In scope
1. **Read section 1 values from the record** (carried forward from WP-0012).
   A commit's hour, weekday and month come from its record's `LocalTime`, its
   date from `ActiveDate`, and its year from `core.CommitYear`. Nothing in the
   family calls `core.Input.Date` or `filter.CommitDate`. The population is
   `core.Input.Analyzed()`, which keeps the year deviation until WP-0017.
2. **Cells** (ADR-0078 clause 3), one for each identity and year with an
   analysed commit, the identity folded through the input's identity table
   (WP-0063):
   - the cell identity and `year`;
   - `hour_by_weekday`: 7 lists of 24 counts, weekday 0 being Monday;
   - `dates`: the cell's active dates, each with its count of analysed
     commits, ordered by date.

   Summing two cells adds `hour_by_weekday` bucket by bucket and merges `dates`,
   adding the counts of equal dates.
3. **One derivation** (ADR-0078 clause 5). Export a pure function from the
   family package that sums the cells a `core.Scope` includes and derives every
   metric from the sum, and fill the family's projection slot in the registry
   with it. The family's repository metrics are that function over every cell.
   A scope whose sum is empty yields every metric absent.
3a. **Compare only scoped metrics.** The projection checker of WP-0063
   compares whole metric sets, which holds for `temporal`, whose every metric
   is scoped, and fails for a family that also has repository metrics: merging
   two identities, or restricting to a year, legitimately changes those. Make
   it read each family's `Scope` column, as `TestMetricCatalogueCells` does,
   and compare the metrics marked `person` for a selection, those marked `year`
   for a year, and every scoped metric for the zero scope. Demonstrate the
   restriction with the scratch family of `TestProjectionRejectsANonAdditiveCell`,
   given one repository metric that changes under a merge. The families after
   this one require it for that reason.
4. **Compute every metric of section 2** from the sum, adding those the family
   lacks: `month_histogram` (12 buckets, 0 being January), `year_histogram`,
   `early_ratio`, `office_hours_ratio`, `regularity`, `active_days`,
   `commits_per_active_day`, `repository_age_days` and `span_ratio`. Write the
   following into section 2 as their definitions, since the current text leaves
   them open or cannot be derived from cells:
   - `year_histogram` is a list of objects with `year` and `count`, one for
     every year from the first to the last active date's year, zeros included.
   - `longest_silence_days` is the largest number of calendar days strictly
     between two consecutive active dates, so consecutive active dates give 0,
     and a single active date gives 0. The timestamp gap it replaces cannot be
     summed across cells (ADR-0078 clause 10).
   - `repository_age_days` is the number of days from the first active date to
     the last.
   - `busiest_day` breaks a tie on count by the earlier date.
5. **Ratios carry 6 significant digits** (section 1, Rounding). Use
   `core.RoundSignificant` from `internal/core/stats.go`; if it does not exist,
   add it there with a unit test, rounding half away from zero.
6. **Mark the scopes in section 2**: add the `Scope` column to its `Metric`
   table, `person, year` on every metric row; add the row `cells`, marked
   `repository`; add the `Cell field` table defining the fields of clause 2; and
   remove the sentence beginning "Person scope:", which the column replaces.
7. Add the metric and cell types to `internal/core/metrics.go` and to
   `docs/report-schema.json`, the cells' JSON as clause 2 names it.
8. **Increment the family version to 2.0**: existing values change (ratios'
   rounding, and `longest_silence_days` by definition).
9. Regenerate every golden file in one commit whose body names each cause:
   the new metrics, the cells, the rounding, the silence definition and the
   version.
10. Every test this package adds is named with the prefix `TestTemporal` or
    `TestProjection`.

## Out of scope
- Any other family, and any metric section 2 does not define.
- Removing the year deviation (WP-0017).
- The interpretation axes built from these figures (WP-0028).
- Person-scoped values in the report beyond the cells. The report carries the
  cells; scoped figures are computed on request (ADR-0078 clause 6).

## Files
**May create or modify:** `internal/metrics/temporal/**`,
`internal/core/metrics.go`, `internal/core/stats.go`,
`internal/core/stats_test.go`, `internal/pipeline/aggregate/registry.go`,
`internal/checks/**` **only where a test names or calls the temporal family,
and for the projection checker's comparison in clause 3a**,
`docs/metrics.md` **section 2 only**, `docs/report-schema.json`,
`testdata/golden/**`.
**Must not touch:** every other package under `internal/metrics/`,
`internal/pipeline/collect/**`, `internal/pipeline/replay/**`, `cmd/**`,
`docs/decisions/**`, and every other section of `docs/metrics.md`.

## Steps
1. Switch the family to the record's values and the identity table; run the
   golden comparison and confirm that nothing differs. A difference means the
   record and the recomputation disagree: stop and report it.
2. Build the cells and the derivation; derive the repository metrics through
   it.
3. Add the missing metrics.
4. Edit section 2, the core types and the schema.
5. Fill the projection slot; run `TestProjectionEqualsRecomputation` and
   `TestMetricCatalogueCells`.
6. Increment the version and regenerate the golden files in one commit.

## Definition of done
- On the fixture `basic`, every metric section 2 defines is present.
- `TestMetricCatalogueCells` passes with the temporal `Cell field` table.
- `TestProjectionEqualsRecomputation` lists `temporal` among the families it
  checked, and passes.
- The projection checker compares only the metrics a family's `Scope` column
  marks for the dimension compared, and refuses the scratch family's scoped
  error while accepting its repository metric's change.
- The family's version is 2.0, and its golden commit body names every cause
  clause 9 lists.
- No file of the family refers to `Input.Date` or `CommitDate`.
- Section 2 has no sentence beginning "Person scope:".

## Verification
```
make gate-full
go test ./internal/metrics/temporal/...
go test ./internal/checks -run '^(TestMetricCatalogue|TestProjection|TestReportSchema|TestGolden)'
git grep -n 'Input\.Date\|\.Date(c)\|CommitDate' -- internal/metrics/temporal
# no output
git grep -n 'Person scope:' -- docs/metrics.md
# no output
```
