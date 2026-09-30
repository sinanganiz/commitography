# WP-0024: worktype family output

**Area:** metrics
**Implements:** ADR-0076, ADR-0018, ADR-0074, ADR-0078, ADR-0019, ADR-0062, ADR-0031, ADR-0032
**Requires:** WP-0018, WP-0062, WP-0063

## Goal
The `worktype` family applies the recency window to replay's recorded events
when it is aggregated, reports the repository's shares, and carries the
editor-by-owner breakdown ADR-0018 requires as cells by year, from which any
selection's and any year's shares are an exact projection.

## In scope
1. **Apply the window here** (ADR-0074 clauses 8 and 9, carried forward from
   WP-0014). A replacement or deletion of age less than
   `recency_window_days` is recent; equal or greater is legacy. Replay state is
   read, never changed.
2. **Cells** (ADR-0078 clause 3, ADR-0018 clause 1), folded through the
   identity table on both identities; an editor or owner the table does not
   represent individually folds into the aggregate bucket:
   - a **removal cell** for each editor, owner and editing year with a
     replacement or deletion: `editor`, `owner`, `year`, and the counts
     `recent_own`, `recent_other` and `legacy`. `recent_own` counts recent
     events whose editor **is** the removed line's owner, decided before
     folding; `recent_other` counts the rest. Two identities folded into the
     bucket keep their events apart this way, so the repository's rework is
     never inflated by folding;
   - an **addition cell** for each editor and editing year with an addition:
     `editor`, `year` and `new_work`, with no `owner`.

   Summing adds the counts of cells with equal keys. This replaces the metric
   `breakdown`: remove its row from section 8, and write that the cells are the
   breakdown ADR-0018 requires.
3. **One derivation** (ADR-0078 clause 5), an exported pure function over a
   scope, filling the family's projection slot. For a scope of selection S:
   `new_work` sums `new_work` of addition cells whose editor is in S;
   `legacy_refactor` sums `legacy` of removal cells whose editor is in S;
   `rework` sums `recent_own` of those cells, and `recent_other` of those whose
   owner is in S too; `help_others` sums `recent_other` of those whose owner is
   not. For the scope of every identity, `rework` is every `recent_own` and
   `help_others` every `recent_other`. The year filters cells by `year`.
   Write this into section 8, replacing its paragraph on person-scope shares.
4. **Report** `repository_shares`, `classified_line_count` and
   `recency_window_days` as section 8 defines them, from the derivation over
   every cell. Shares carry 6 significant digits through
   `core.RoundSignificant`; add it with a unit test if it is not there.
5. **Status.**
   - Replay state's `Worktype.Degraded` of `limit_reached_size` degrades the
     family with that reason, confidence `partial` (carried forward from
     WP-0014).
   - Where replay state holds no work-type inputs, because the history does not
     reach every ancestor, the family is skipped with `history_incomplete`
     (WP-0023).
6. **Extend `core.Input.RestrictToYear`** so that the replay state it returns
   holds only the work-type pairs and additions of that year.
7. **Invariants** (ADR-0019 clause 3):
   - `TestWorktypeSharesSumToOne`: on every fixture, the four repository shares
     sum to 1 within 0.000005;
   - the projection checker (WP-0063) covers the merge invariant of ADR-0018
     clause 2 through the family's projection slot; on the fixture `basic`,
     whose one person commits under two addresses, it must compare at least one
     pair.
8. **Mark the scopes in section 8**: the `Scope` column, `person, year` on
   `repository_shares` and `classified_line_count` and `repository` on
   `recency_window_days`; the row `cells`, marked `repository`; the `Cell field`
   table, defining both kinds of cell.
9. Add the metric and cell types to `internal/core/metrics.go` and
   `docs/report-schema.json`; the family leaves `not_implemented`, and its
   version becomes 1.0.
10. Regenerate every golden file in one commit whose body states that the
    family is computed for the first time.
11. Every test this package adds is named with the prefix `TestWorktype`. One
    holds the fixture `worktype-events` to class counts derived by hand from its
    construction at the default window, and at a window of 1 day.

## Out of scope
- The year deviation recorded against WP-0017, which this package inherits
  unchanged.
- Any change to replay, which records and applies nothing (ADR-0074).
- The interpretation axes (WP-0028) and the person lens (WP-0051).

## Files
**May create or modify:** `internal/metrics/worktype/**`,
`internal/core/metrics.go`, `internal/core/input.go`, `internal/core/stats.go`,
`internal/core/stats_test.go`, `internal/pipeline/aggregate/registry.go`,
`internal/checks/**` **only where a test names or calls the worktype family**,
`docs/metrics.md` **section 8 only**, `docs/report-schema.json`,
`testdata/golden/**`.
**Must not touch:** every other package under `internal/metrics/`,
`internal/pipeline/collect/**`, `internal/pipeline/replay/**`, `cmd/**`,
`docs/decisions/**`, and every other section of `docs/metrics.md`.

## Steps
1. Build the cells from replay's inputs with the window applied.
2. Write the derivation and report the repository values through it.
3. Apply the two statuses.
4. Extend `RestrictToYear`; fill the projection slot; add the invariant tests.
5. Edit section 8, the core types and the schema.
6. Regenerate the golden files in one commit.

## Definition of done
- On the fixture `worktype-events`, the class counts equal the hand-derived
  values at both windows.
- On every fixture, the repository shares sum to 1 within 0.000005.
- `TestProjectionEqualsRecomputation` lists `worktype`, and on `basic` compares
  at least one pair of identities.
- Section 8 has no `breakdown` row and defines both kinds of cell.
- The family's version is 1.0 and its status is computed.

## Verification
```
make gate-full
go test ./internal/metrics/worktype/...
go test ./internal/checks -run '^(TestMetricCatalogue|TestProjection|TestReportSchema|TestGolden|TestWorktype|TestWorkType)'
git grep -n '`breakdown`' -- docs/metrics.md
# no output
```
