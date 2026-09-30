# WP-0023: ownership family

**Area:** metrics
**Implements:** ADR-0076, ADR-0078, ADR-0020, ADR-0018, ADR-0009, ADR-0053, ADR-0062, ADR-0031, ADR-0032
**Requires:** WP-0018, WP-0021, WP-0062, WP-0063

## Goal
The `ownership` family computes every metric of `docs/metrics.md` section 7
from replay's ownership map over all tracked text lines, and carries the
person-and-year cells the interpretation axes need, keyed by the year each line
was last written.

## In scope
1. **Inputs.** The family declares replay state alone (ADR-0076 clause 6) and
   reads, beside it, two values the aggregate stage computes once for every
   family: the identity table (WP-0063) and directory activity (WP-0021). Both
   are shared derived data in `core` (ADR-0040 clause 4, ADR-0076 clause 4),
   not the output of a family. Write this into section 7 in one sentence.
2. **When there is no map.** Where replay state carries no ownership map,
   because the collected history does not reach every ancestor of the analysed
   commit, the family is skipped. No reason code states that condition today.
   Add `history_incomplete` to section 13's family status codes, defined as
   "the collected history does not reach every ancestor of the analysed
   commit, so replay could not derive line ownership", add it to the reason
   enumeration in `internal/core/reason.go`, and produce it here, in the same
   change (ADR-0062 clause 6). `worktype` and `ai-archaeology` reuse it.
3. **Compute every metric of section 7.** Write these definitions into
   section 7:
   - An identity is the owning identity of a line, whether or not it has an
     analysed commit. Identities the identity table does not represent
     individually count in the aggregate bucket wherever a metric lists
     identities; `bus_factor` and `knowledge_concentration` count every
     identity separately, the bucket never as one.
   - `lines_by_identity` lists `id` or `aggregate` with `lines`, ordered by
     `id`, the aggregate last.
   - The directories in scope are those of `files.directory_activity`'s
     definition with at least 10 touching commits, read from directory
     activity. `bus_factor` holds `repository` and a list of objects with
     `path` and `bus_factor`. `knowledge_concentration` is a list of objects
     with `path` and `share`, and names no owner: `directory_ownership` carries
     the owners, and the command-line flag the sentence "The owner is named only
     … when per-contributor output is enabled" refers to is being removed
     (WP-0017). Remove that sentence.
   - `directory_ownership` lists, for each directory in scope, the 10
     identities with the most lines there, ordered by `id`, with `lines`.
   - A line's age is the map's `Day` minus the line's day. `median_line_age_days`
     is the median of the ages, as `core.Median` computes a median.
   - `code_age_distribution` is a list of objects with `year` and `lines`, one
     for every year from the earliest to the latest line's year, zeros
     included.
   - The repository's first calendar year is the year of the earliest
     `first_commit_date` in the identity table.
   - `orphaned_line_ratio` counts a line whose owner has no analysed commit
     within the 365 days before the map's `Day`, or none at all.
4. **Add the metrics the interpretation axes name** (`concentration`,
   `persistence` and `sole_owner_of_directories` in
   `internal/pipeline/interpret/taxonomy/axes.md`) to section 7:
   - `lines_by_directory`: surviving lines in each depth-1 directory and each
     directory in scope, as objects with `path` and `lines`;
   - `written_line_count`: the lines the scope's commits wrote, from replay's
     per-commit `Written` (WP-0062), attributed to the writing commit's
     identity and year.
5. **Cells** (ADR-0078 clause 3), keyed by owning identity, folded through the
   identity table, and by **the year the line was last written** (ADR-0078
   clause 8): the cell identity and `year`; `lines`, the surviving lines;
   `directories`, the surviving lines per directory of `lines_by_directory`;
   and `written`, the lines the identity's commits of that year wrote. Summing
   adds every count, per directory for `directories`. Write into section 7 that
   the year of an ownership cell is the year of last write, not of an event.
6. **One derivation** (ADR-0078 clause 5), an exported pure function over the
   sum of the cells a scope includes, filling the family's projection slot. The
   scoped metrics are `line_count`, `code_age_distribution`,
   `lines_by_directory` and `written_line_count`; their repository values are
   that function over every cell. Every other metric of section 7 is a
   repository metric. Extend `core.Input.RestrictToYear` so that the replay
   state it returns holds only the lines last written in the year, and only
   the per-commit `Written` of that year's commits.
7. **Method.** The family's declaration carries section 7's method statement
   on forward replay and blame (ADR-0032 clause 8).
8. Every list cut at its section 12 limit degrades the family with
   `cardinality_limit`. The reason `limit_reached_size` on any map file
   degrades it, confidence `partial`: the lines of that file are absent or
   attributed without the version replay could not read.
9. **Ratios carry 6 significant digits** through `core.RoundSignificant`; add
   it with a unit test if it is not there.
10. **Mark the scopes in section 7**: the `Scope` column, `person, year` on the
    four scoped rows and `repository` on the rest; the row `cells`, marked
    `repository`; the `Cell field` table.
11. Add the metric and cell types to `internal/core/metrics.go` and
    `docs/report-schema.json`; the family leaves `not_implemented`, and its
    version becomes 1.0.
12. Regenerate every golden file in one commit whose body states that the
    family is computed for the first time.
13. Every test this package adds is named with the prefix `TestOwnership`. One
    test holds `bus_factor` and `code_age_distribution` on the fixture
    `renames-and-copied-block` to values derived by hand from the fixture's
    construction, stated in the test.

## Out of scope
- `git blame`, whose divergence from replay ADR-0019 clause 6 already measures.
- The interpretation axes built from these figures (WP-0028).
- Any other family.

## Files
**May create or modify:** `internal/metrics/ownership/**`,
`internal/core/metrics.go`, `internal/core/input.go`, `internal/core/reason.go`,
`internal/core/stats.go`, `internal/core/stats_test.go`,
`internal/pipeline/aggregate/registry.go`, `internal/checks/**` **only where a
test names or calls the ownership family or the new reason code**, `docs/metrics.md`
**sections 7 and 13 only**, `docs/report-schema.json`, `testdata/golden/**`.
**Must not touch:** every other package under `internal/metrics/`,
`internal/pipeline/collect/**`, `internal/pipeline/replay/**`, `cmd/**`,
`docs/decisions/**`, and every other section of `docs/metrics.md`.

## Steps
1. Add `history_incomplete` with its producer and its catalogue line.
2. Compute the metrics of clause 3 from the map; add those of clause 4.
3. Build the cells and the derivation; extend `RestrictToYear`; fill the
   projection slot.
4. Edit sections 7 and 13, the core types and the schema.
5. Regenerate the golden files in one commit.

## Definition of done
- On the fixture `basic`, the family is `ok` and every metric section 7 defines
  is present.
- `history_incomplete` appears in section 13, in the reason enumeration, and in
  the family's code, and `TestReasonCodeCatalogue` passes.
- `TestMetricCatalogueCells` and `TestProjectionEqualsRecomputation` pass, the
  latter listing `ownership`.
- The family's version is 1.0 and its status is computed.
- Section 7 no longer mentions per-contributor output.

## Verification
```
make gate-full
go test ./internal/metrics/ownership/...
go test ./internal/checks -run '^(TestMetricCatalogue|TestProjection|TestReportSchema|TestGolden|TestReasonCode|TestOwnership)'
git grep -n 'per-contributor' -- docs/metrics.md
# no output
```
