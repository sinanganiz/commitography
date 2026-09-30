# WP-0063: Scopes, cells and the identity bound in core

**Area:** core
**Implements:** ADR-0078, ADR-0018, ADR-0062, ADR-0064
**Requires:** WP-0061

## Goal
`core` holds the scope, the cell identity and the one identity bound; the
identities section folds through that bound; `docs/metrics.md` defines scopes,
cells and how a metric is marked scoped; and two checkers exist that every
family gaining cells joins without changing them: one holding cell fields to
the catalogue, one holding projection equal to recomputation.

## In scope
1. **Move the identity bound into `core`** (ADR-0078 clause 9). Add a type,
   `core.IdentityTable`, computed once from the analysed commits: for every
   identity its `id`, analysed commit count, and first and last analysed local
   dates; and the individually represented set, selected by the rule the
   identities section uses today (most analysed commits, ties by earlier
   `first_commit_date`, then by `id`, up to `core.LimitIdentities`). It
   answers, for an `id`, whether the identity is individual or folds into the
   aggregate bucket.
2. The aggregate stage computes the table **once per build**, before any family
   runs, and places it on `core.Input` as a field, so every family reads the
   same table. `buildIdentities` folds through it and selects nothing itself.
   Take the dates from each commit record's `ActiveDate`, which the collect
   stage decided (WP-0012), not by recomputing them.
3. **Nothing outside `core` selects the bound.** Add the checker
   `TestScopeBoundIsSelectedOnce`: no Go file under `internal/` outside
   `internal/core` references `core.LimitIdentities`, test files of
   `internal/checks` aside. Observe it failing against the tree before the
   move, where the aggregate stage references it.
4. **Add the scope types to `core`** (ADR-0078 clause 1):
   - `CellIdentity`: an `id`, or the aggregate bucket. Its JSON form is
     `{"id": "<id>"}` or `{"aggregate": true}`, the fields named as the
     identities section names them. A cell keyed by one identity embeds it, so
     its cells read `{"id": …, "year": …}`; a cell keyed by two names each.
   - `Scope`: a selection and a year. The zero value is every identity and
     every year. A constructor validates a scope against an `IdentityTable`
     and refuses an `id` that is not individually represented, a repeated
     `id`, and any attempt to name the aggregate bucket.
   - `Scope` answers whether it includes a `CellIdentity` and whether it
     includes a year. A scope of every identity includes the aggregate bucket;
     a selection never does (ADR-0078 clause 9).
   - `CommitYear(model.Commit) int`: the year of the record's `ActiveDate`
     (ADR-0078 clause 8).
5. **Give the registry a projection slot.** A registry entry gains an optional
   function that takes the families of a report and a scope and returns the
   family's metrics for that scope. It is nil for every family in this
   package. The projection checker (clause 8) reads the registry, so a family
   joins it by filling its slot and nothing else.
6. **Define scopes and cells in `docs/metrics.md` section 1**, under a new
   subsection `### Scopes and cells`, as ADR-0078 states them: scope, cell,
   cell identity, the year of a cell, additivity, one derivation, the
   aggregate bucket, and that the identity bound is the one of section 14. State
   the two conventions every family section follows once it has a scoped
   metric:
   - its `Metric` table gains a third column, `Scope`, holding `repository`,
     `person`, `year` or `person, year` for every row; a table without the
     column is repository-only;
   - it has a table headed `Cell field`, defining every field of one cell,
     key fields included, and its `Metric` table lists `cells`.
7. **Add the catalogue checker for cells**, `TestMetricCatalogueCells`, in both
   directions, for every family:
   - a family with a `Cell field` table lists `cells` in its `Metric` table,
     and its Go metric type has a `cells` field whose element type's JSON names
     are exactly the table's fields, an embedded struct's fields counting under
     the names they are promoted to;
   - a family whose `Metric` table marks any row `person` or `year` has a
     `Cell field` table, and a family with a `Cell field` table marks at least
     one row so;
   - a `Scope` column, where present, holds one of the four values on every
     row.

   With no family carrying cells, it passes. Add
   `TestMetricCatalogueCellsRejectsEachViolation`, which feeds it a synthetic
   catalogue and type for each of the violations above and requires each to be
   refused (ADR-0064).
8. **Add the projection checker**, `TestProjectionEqualsRecomputation`
   (ADR-0078 clause 7, ADR-0019 clause 3), driven by the registry's projection
   slots. For every family whose slot is filled, over the fixtures `basic`,
   `mailmap`, `merges` and `multi-year-gap`:
   - **repository**: the family's projection for the zero scope equals the
     metrics the report holds, `cells` aside;
   - **identities**: for every pair of individually represented identities,
     up to the first 10 pairs in `id` order, the projection for the pair equals
     the projection for the one identity they become in a run whose
     configuration merges their addresses (`config.Identity`);
   - **years**: for every year with an analysed commit, the projection for
     that year equals the family's registered section built over an input
     restricted to that year, `cells` aside. Add
     `core.Input.RestrictToYear(year int) core.Input`, which marks every commit
     of another year as not analysed and keeps the input's identity table, so
     the bound is the whole analysis's. It does not yet touch replay state;
     each replay-based family package extends it for the replay inputs it
     reads, in the same change as the family.

   The checker also requires the set of families with a filled slot to equal
   the set whose catalogue section has a `Cell field` table, so a family cannot
   carry cells without joining it.
9. Add `TestProjectionRejectsANonAdditiveCell`: a scratch family defined in the
   test file, whose projection derives a median from per-cell medians, run
   through the checker's comparison functions on the fixture `basic`, must be
   refused. It is not registered and changes no report.
10. Every test this package adds is named with the prefix `TestScope`,
    `TestMetricCatalogueCells` or `TestProjection`.

## Out of scope
- Giving any family cells, a `Scope` column or a projection. WP-0018 to WP-0026
  do that, each for its own family.
- Changing any report. No golden file changes in this package.
- Replay state and its year key (WP-0062).
- Removing the year from the analysis (WP-0017). `core.Input.Analyzed` keeps
  applying it; the table is computed over the commits it returns, as the
  identities section is today.
- The interpret stage's use of scopes (WP-0016).

## Files
**May create or modify:** `internal/core/**`, `internal/pipeline/aggregate/**`,
`internal/checks/**`, and `docs/metrics.md` **section 1 only**.
**Must not touch:** `internal/metrics/**`, `internal/pipeline/collect/**`,
`internal/pipeline/replay/**`, `internal/pipeline/interpret/**`, `cmd/**`,
`docs/decisions/**`, `docs/report-schema.json`, `testdata/**`.

## Steps
1. Write `TestScopeBoundIsSelectedOnce` and observe it failing on the current
   tree, which selects the bound in the aggregate stage.
2. Add `IdentityTable`; compute it in the aggregate stage; fold the identities
   section through it. Run the golden comparison: nothing changes.
3. Add `CellIdentity`, `Scope`, `CommitYear` and `RestrictToYear`, with unit
   tests in `internal/core`.
4. Add the registry's projection slot.
5. Write section 1's `Scopes and cells` subsection.
6. Add the cell catalogue checker and its rejection test.
7. Add the projection checker and its rejection test.

## Definition of done
- `core.IdentityTable` exists, the aggregate stage computes it once per build,
  and `TestScopeBoundIsSelectedOnce` passes after having been observed failing.
- `core.CellIdentity`, `core.Scope`, `core.CommitYear` and
  `core.Input.RestrictToYear` exist with unit tests.
- A scope naming an identity outside the bound, a repeated identity or the
  aggregate bucket is refused by the scope constructor's tests.
- `docs/metrics.md` section 1 defines scopes, cells, the `Scope` column and the
  `Cell field` table.
- `TestMetricCatalogueCells` and `TestProjectionEqualsRecomputation` pass, and
  their rejection tests pass by refusing every synthetic violation.
- No file under `testdata/` changed.

## Verification
```
make gate-full
go test ./internal/checks -run '^(TestScope|TestMetricCatalogueCells|TestProjection)'
go test ./internal/core/...
git log --oneline --grep='^WP-0063' -- testdata
# no output: no commit of this package touched testdata
```
