# WP-0021: files family

**Area:** metrics
**Implements:** ADR-0076, ADR-0078, ADR-0040, ADR-0053, ADR-0062, ADR-0031, ADR-0032
**Requires:** WP-0018, WP-0062, WP-0063

## Goal
The `files` family computes every metric of `docs/metrics.md` section 5 from
commit records and the replay stage's tree, reads section 1 values from the
record, carries the person-and-year cells the interpretation axes need, and
provides the directory activity `ownership` will share through `core`.

## In scope
1. **Read section 1 values from the record** (carried forward from WP-0012):
   `FileChange.Excluded`, `ActiveDate`, and the year through
   `core.CommitYear`. Nothing in the family calls `filter.IncludedFiles`,
   `core.Input.Date` or `filter.CommitDate`, or reads `core.Input.PathFilter`;
   a tracked path's exclusion at the analysed commit is read from
   `ReplayState.Excluded` (WP-0062).
2. **The population is the analysed commits, bulk commits included.** Section
   1 counts bulk commits toward commit counts, and every metric of this family
   but none of the line metrics is a count of commits or paths. Today the
   family counts non-bulk commits only; this changes `most_modified` and
   `oldest_untouched` on the fixture `noise`.
3. **Directory activity is shared derived data** (ADR-0040 clause 4). Add to
   `core` a function computing, from the analysed commits, the number of
   commits touching a non-excluded path under each directory at depth 1 and 2;
   the aggregate stage computes it once and places it on `core.Input`, as it
   does the identity table. A directory at depth 1 is a path's first segment,
   for a path of two segments or more; at depth 2, its first two, for a path of
   three or more. `ownership` reads the same value (WP-0023).
4. **Compute every metric of section 5**, adding those the family lacks.
   Write these definitions into section 5:
   - `extension_distribution` holds two lists, `by_commits` and `by_files`,
     each of at most 20 objects with `extension` and a count: the analysed
     commits touching a non-excluded path with that extension, each commit
     counted once per extension; and the non-excluded files at the analysed
     commit with that extension. An extension is the lowercased text after the
     last dot of a path's base name, where that dot is not its first
     character, and the empty string otherwise.
   - `directory_activity` lists the directories of clause 3 with at least 10
     touching commits, as objects with `path`, `depth` and `commits`, by
     commits descending and then path, at most 50.
   - `oldest_untouched` is chosen among the non-excluded files at the analysed
     commit that an analysed commit changed, by the earliest date of the
     latest analysed commit changing each, ties by path. It carries `path`,
     `date`, and `last_changed_by`, the cell identity of that commit's author.
   - `head_file_count` counts every path in the analysed commit's tree, files
     and submodules, excluded or not.
   - `deleted_file_count` counts the distinct non-excluded paths an analysed
     commit changed that are absent at the analysed commit.
   - `rename_count` counts the changes of analysed commits that git detected as
     renames to a non-excluded path.
   - A change **adds a file** when it has no previous version, no previous path,
     a new version, and a non-excluded path. `new_file_commits` counts the
     analysed commits with at least one such change.
   Every list cut at its limit degrades the family with `cardinality_limit`.
5. **Add the metrics the interpretation axes name** (`new_file_share`,
   `new_files_created`, `reach` and `directory_count` in
   `internal/pipeline/interpret/taxonomy/axes.md`) to section 5:
   - `new_file_count`: the changes of analysed commits that add a file;
   - `directories_touched`: the depth-1 directories holding a non-excluded path
     an analysed commit changed;
   - `directory_count`: the depth-1 directories at the analysed commit holding
     a non-excluded path.
6. **Cells** (ADR-0078 clause 3), one per identity and year with an analysed
   commit, folded through the identity table: the cell identity and `year`;
   `new_file_commits`; `new_file_count`; and `directories`, the names of the
   depth-1 directories the cell's commits touched, sorted. Summing adds the two
   counts and takes the union of the names. The scoped metrics are
   `new_file_commits`, `new_file_count` and `directories_touched`; every other
   metric of section 5 is a repository metric.
7. **One derivation** (ADR-0078 clause 5): an exported pure function over the
   sum of the cells a scope includes, filling the family's projection slot. The
   repository values of the three scoped metrics are that function over every
   cell.
8. **Mark the scopes in section 5**: the `Scope` column, `person, year` on the
   three scoped rows and `repository` on the rest; the row `cells`, marked
   `repository`; the `Cell field` table.
9. Add the metric and cell types to `internal/core/metrics.go` and
   `docs/report-schema.json`.
10. **Increment the family version to 3.0**: existing values change with the
    population.
11. Regenerate every golden file in one commit whose body names each cause.
12. Every test this package adds is named with the prefix `TestFiles`.

## Out of scope
- Any other family; `ownership`'s use of directory activity is WP-0023's.
- Following renames through history.
- Year scopes for the repository metrics, such as the most modified files of a
  year. None is required yet; a later package adds cells for one with a minor
  version increment.

## Files
**May create or modify:** `internal/metrics/files/**`,
`internal/core/metrics.go`, `internal/core/input.go`, a new file under
`internal/core/` for directory activity and its test,
`internal/core/stats.go`, `internal/core/stats_test.go`,
`internal/pipeline/aggregate/**`, `internal/checks/**` **only where a test
names or calls the files family or directory activity**, `docs/metrics.md`
**section 5 only**, `docs/report-schema.json`, `testdata/golden/**`.
**Must not touch:** every other package under `internal/metrics/`,
`internal/pipeline/collect/**`, `internal/pipeline/replay/**`, `cmd/**`,
`docs/decisions/**`, and every other section of `docs/metrics.md`.

## Steps
1. Read the record's values; run the golden comparison: nothing may differ.
   A difference means the record and the recomputation disagree; stop and
   report it.
2. Change the population, and note which golden values move.
3. Add directory activity to `core` and the aggregate stage.
4. Add the missing metrics, the cells and the derivation; fill the projection
   slot.
5. Edit section 5, the core types and the schema.
6. Increment the version and regenerate the golden files in one commit.

## Definition of done
- On the fixture `basic`, every metric section 5 defines is present.
- Directory activity is computed once per build, in `core`, and the family
  reads it from `core.Input`.
- `TestMetricCatalogueCells` and `TestProjectionEqualsRecomputation` pass, the
  latter listing `files`.
- The family's version is 3.0.
- No file of the family refers to `IncludedFiles`, `PathFilter`, `Input.Date`
  or `CommitDate`.

## Verification
```
make gate-full
go test ./internal/metrics/files/... ./internal/core/...
go test ./internal/checks -run '^(TestMetricCatalogue|TestProjection|TestReportSchema|TestGolden)'
git grep -n 'IncludedFiles\|PathFilter\|Input\.Date\|CommitDate' -- internal/metrics/files
# no output
```
