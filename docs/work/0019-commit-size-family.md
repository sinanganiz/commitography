# WP-0019: commit-size family

**Area:** metrics
**Implements:** ADR-0076, ADR-0078, ADR-0062, ADR-0031, ADR-0032
**Requires:** WP-0018, WP-0063

## Goal
The `commit_size` family computes every metric of `docs/metrics.md` section 3
from person-and-year cells, by one derivation that serves the repository, any
selection and any year, and reads effective lines and excluded paths from the
commit record.

## In scope
1. **Read section 1 values from the record** (carried forward from WP-0012):
   a commit's effective lines from `EffectiveLines`, whether it is bulk from
   `IsBulk`, a path's exclusion from its `FileChange.Excluded`, its date from
   `ActiveDate`, and its year from `core.CommitYear`. Nothing in the family
   calls `filter.IncludedFiles`, `core.Input.Date` or `filter.CommitDate`, or
   reads `core.Input.PathFilter`.
2. **The population** is the analysed, non-bulk commits that are not merges.
   Write this into section 3, with the reason: a merge's changes are its
   parents' (ADR-0073), its record holds no files, and counting it as a
   commit of zero lines would bias every size figure. The code already leaves
   merges out of the mean and median; this makes every metric agree.
3. **Cells** (ADR-0078 clause 3), one for each identity and year with an
   analysed commit, folded through the input's identity table:
   - the cell identity and `year`;
   - `lines`: the population's commits counted by effective lines, as a list of
     objects with `lines` and `commits`, ordered by `lines`;
   - `files`: the population's commits counted by distinct non-excluded paths
     touched, as objects with `files` and `commits`, ordered by `files`;
   - `added` and `removed`: the population's lines added and removed on
     non-excluded paths;
   - `largest`: the population's largest commit, with hash, date, lines and
     files, or absent;
   - `bulk_commit_count`, and `bulk_commits`: the cell's bulk commits, at most
     10, with hash, date and lines.

   Summing two cells adds the histograms and the two line counts, keeps the
   larger `largest`, adds the bulk counts, and keeps the first 10 of both bulk
   lists. Largest and bulk ordering is by lines descending, then earlier date,
   then smaller hash (section 1, Ties).
4. **One derivation** (ADR-0078 clause 5): an exported pure function over the
   sum of the cells a scope includes, filling the family's projection slot. The
   repository metrics are that function over every cell.
5. **Compute every metric of section 3**, adding those the family lacks:
   `p90_lines`, `mean_files`, `median_files`, `size_buckets`,
   `small_commit_ratio`, `single_file_ratio`, `bulk_commit_count`,
   `bulk_commits` and `addition_ratio`. Add `p90_files`, which the
   interpretation axes name (`breadth`, `internal/pipeline/interpret/taxonomy/axes.md`),
   to section 3. Write these definitions into section 3:
   - a percentile is the nearest-rank percentile: the smallest value at or
     below which at least that share of the population lies; the median is the
     mean of the two middle values of an even population, as `core.Median`
     computes it today;
   - `size_buckets` are the ranges 0–9, 10–49, 50–199, 200–999 and 1000+, as a
     list of objects with `from`, `to` (absent on the last) and `commits`. The
     first range starts at 0, so that a commit changing only excluded paths or
     only renaming is counted, and the buckets sum to the population.
6. **Ratios carry 6 significant digits** through `core.RoundSignificant`; add
   it to `internal/core/stats.go` with a unit test if it is not there.
   `mean_lines`, `median_lines`, `mean_files` and `median_files` keep one
   decimal place, as today.
7. **Mark the scopes in section 3**: the `Scope` column, `person, year` on
   every metric row; the row `cells`, marked `repository`; and the `Cell field`
   table.
8. Add the metric and cell types to `internal/core/metrics.go` and
   `docs/report-schema.json`.
9. **Increment the family version** (ADR-0031 clause 2): its minor component,
   to 2.1, if no value the family already reports changed in any golden file;
   its major component, to 3.0, if any did.
10. Regenerate every golden file in one commit whose body names each cause.
11. Every test this package adds is named with the prefix `TestCommitSize`.

## Out of scope
- Any other family.
- The definition of effective lines or bulk commits, which section 1 owns.
- The interpretation axes (WP-0028).

## Files
**May create or modify:** `internal/metrics/commitsize/**`,
`internal/core/metrics.go`, `internal/core/stats.go`,
`internal/core/stats_test.go`, `internal/pipeline/aggregate/registry.go`,
`internal/checks/**` **only where a test names or calls the commit-size family**,
`docs/metrics.md` **section 3 only**, `docs/report-schema.json`,
`testdata/golden/**`.
**Must not touch:** every other package under `internal/metrics/`,
`internal/pipeline/collect/**`, `internal/pipeline/replay/**`, `cmd/**`,
`docs/decisions/**`, and every other section of `docs/metrics.md`.

## Steps
1. Switch to the record's values and run the golden comparison: nothing may
   differ. A difference means the record and the recomputation disagree; stop
   and report it.
2. Apply the population of clause 2 and note which golden values move.
3. Build the cells and the derivation; add the missing metrics.
4. Edit section 3, the core types and the schema; fill the projection slot.
5. Increment the version and regenerate the golden files in one commit.

## Definition of done
- On the fixture `noise`, which holds a bulk commit, every metric section 3
  defines is present, and `bulk_commit_count` is at least 1.
- On the fixture `basic`, `size_buckets` sum to the population's commit count.
- `TestMetricCatalogueCells` and `TestProjectionEqualsRecomputation` pass, the
  latter listing `commit_size`.
- The family's version is 2.1 where no existing golden value moved, and 3.0
  otherwise, and the golden commit body says which applied.
- No file of the family refers to `IncludedFiles`, `PathFilter`, `Input.Date`
  or `CommitDate`.

## Verification
```
make gate-full
go test ./internal/metrics/commitsize/...
go test ./internal/checks -run '^(TestMetricCatalogue|TestProjection|TestReportSchema|TestGolden)'
git grep -n 'IncludedFiles\|PathFilter\|Input\.Date\|CommitDate' -- internal/metrics/commitsize
# no output
```
