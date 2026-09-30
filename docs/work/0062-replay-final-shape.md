# WP-0062: Replay state takes its final shape

**Area:** pipeline
**Implements:** ADR-0079, ADR-0078, ADR-0076, ADR-0074, ADR-0051, ADR-0054
**Requires:** WP-0014, WP-0061

## Goal
Replay state carries everything the replay-based families need, and nothing a
family decides: every line carries its authoring commit, each authoring
commit's written lines and removal ages are recorded, the work-type inputs
carry the editing commit's year, and every text file at the analysed commit
carries the inputs of the complexity proxy. No report changes.

## In scope
1. **The authoring commit of a line** (ADR-0079 clauses 1, 2, 5 and 6).
   `core.OwnedLine` gains `Commit uint32`, JSON `c`, beside `Owner` and `Day`,
   which stay as they are. `core.Ownership` gains `Commits []string`, JSON
   `commits`, the commit table. While walking, the walker numbers each commit
   the first time it writes a line, from the commit records it already holds,
   so no second copy of the history is made. The map it returns at the
   analysed commit holds only the commits its lines reference, numbered in the
   order the lines first reference them, files by path and lines in order, and
   every line's index points into that table. `Encode` and `DecodeOwnership`
   round-trip the new fields.
2. **What became of each commit's lines** (ADR-0079 clauses 3 and 4). Add
   `core.AuthorshipInputs` and the field `ReplayState.Authorship`, nil exactly
   where `Ownership` is nil:
   - `Commits`: one entry for each commit that wrote a line or whose line a
     counted commit removed, ordered by commit object name, holding the commit
     object name, `Written`, and the two age histograms `Replaced` and
     `Deleted`, in the form `core.AgeCount` already has;
   - `Written` counts every line whose derivation gave it its commit as owner,
     whether or not that commit is counted (ADR-0073 clause 6): bulk commits
     and commits of excluded authors write lines too;
   - a replacement or deletion is added to the removed line's authoring
     commit, with its age, **at exactly the points where the work-type inputs
     record it**, and nowhere else;
   - `Degraded` is set to `limit_reached_size` exactly when
     `ReplayState.Worktype.Degraded` is.
3. **The editing year** (ADR-0078 clause 12). `core.WorktypePair` and
   `core.WorktypeAdditions` gain `Year int`, JSON `year`: the calendar year of
   the editing commit's `ActiveDate`. Pairs are ordered by editor, owner and
   year; additions by editor and year. Update the `Recorded inputs` paragraph of
   `docs/metrics.md` section 8 to say the inputs are recorded for each year of
   the editing commit as well. No metric's meaning changes, so no family
   version changes: `worktype` is still skipped at version 0.0.
4. **The complexity proxy's inputs** (ADR-0076 clause 2). Add
   `ReplayState.Indentation`: for each file of the map at the analysed commit
   that has lines, ordered by path, its path and:
   - `NonBlank`: the number of non-blank lines, where a line, split as the map
     splits lines, is blank when every byte of it is a space, a tab, a carriage
     return or the line feed that ends it;
   - `Leading`: the non-blank lines counted by their leading whitespace, the
     longest prefix of spaces and tabs, as the pair (number of tabs, number of
     spaces), ordered by tabs and then spaces;
   - `Increments`: over each two consecutive non-blank lines, blank lines
     between them skipped, the positive differences in their number of leading
     spaces, counted by difference and ordered by it.

   It is computed after the walk from the content of each file's analysed
   version, read through the object reader the walk uses. A binary or degraded
   file has no lines and no entry. Replay applies no unit and no cap to these;
   WP-0026 defines the proxy over them.
5. **The memory budgets** (ADR-0079 clause 7). `TestReplayMemoryBudget`
   counts the index with every line, which the line's size already does, and
   measures the map's commit table as its own budget,
   `replay-commit-table-bytes`, on the large fixture: the bytes of every name
   in the table and one string header per entry. Record both values in
   `internal/checks/budgets.txt`, and extend its header to say what each
   counts. **If `replay-bytes-per-line` rises by more than 4.00, stop and
   report**; ADR-0079 permits no more.
6. **Checkers**, each observed failing (ADR-0064), each with a rejection test:
   - `TestReplayAuthoringCommits`: on the fixture `basic`, every line of
     `src/main.go` carries the commit that wrote it, by construction the
     fixture's commits oldest first, as `TestReplayLinearHistoryOwnership`
     already holds its owners; and on every fixture, every index points into
     the table and the table holds no commit no line references.
   - `TestReplayAuthorshipMatchesWorktype`: on every fixture, the per-commit
     histograms summed over every commit equal the work-type histograms summed
     over every editor, owner and year, kind by kind; and no commit's lines
     surviving in the map exceed its `Written`.
   - `TestReplayIndentation`: on every fixture, every file of the map with
     lines has exactly one `Indentation` entry, and its `NonBlank` and the sum
     of its `Leading` counts are equal. Beside it, a unit test in the replay
     package covers blank lines with carriage returns, tabs before spaces,
     spaces before tabs, a file with no indentation, and a file with only
     blank lines.
   - Extend `TestWorkTypeFixtureEvents` so that each expected event carries
     its year.
7. The three new replay state fields are documented at their declarations in
   `internal/core/replay.go`, each naming the record it serves, and the
   package comment in `internal/pipeline/replay/doc.go` describes them.

## Out of scope
- Any metric family. Nothing in `internal/metrics/` reads the new fields in this
  package; WP-0021 to WP-0026 do.
- Deciding which commits are assisted, applying the recency window, or
  defining the complexity proxy's unit and cap.
- Any report change. No golden file changes.
- Persisting replay state (WP-0034).
- Removing the year deviation recorded against WP-0017.

## Files
**May create or modify:** `internal/pipeline/replay/**`, `internal/core/replay.go`,
`internal/core/replay_test.go`, `internal/checks/**`, and `docs/metrics.md`
**for the section 8 `Recorded inputs` paragraph only**.
**Must not touch:** `internal/metrics/**`, `internal/pipeline/aggregate/**`,
`internal/pipeline/collect/**`, `internal/core/` other than the two files
above, `cmd/**`, `docs/decisions/**`, `docs/report-schema.json`, `testdata/**`.

## Steps
1. Add the authoring commit to lines and the commit table to the map; extend
   the round trip; add `TestReplayAuthoringCommits` and see it fail before the
   walk fills the index.
2. Record the per-commit authorship inputs beside the work-type events; add
   `TestReplayAuthorshipMatchesWorktype`.
3. Add the year to the work-type inputs; update the fixture expectations and
   section 8's paragraph.
4. Compute the indentation inputs after the walk; add the unit test and
   `TestReplayIndentation`.
5. Measure both memory budgets, record them, and commit the budget change on
   its own with the measurements and ADR-0079 in its body.

## Definition of done
- Every line of every fixture's map carries an authoring commit that indexes
  the map's commit table, and the table holds only referenced commits.
- On `basic`, every line of `src/main.go` carries the commit that wrote it.
- On every fixture, the per-commit removal histograms sum to the work-type
  histograms, kind by kind.
- Every work-type pair and addition carries a year.
- Every file of every fixture's map with lines has one indentation entry whose
  counts agree.
- `budgets.txt` holds `replay-bytes-per-line` no more than 4.00 above its value
  before this package, and a measured `replay-commit-table-bytes`.
- Every new checker was observed failing and has a rejection test.
- No file under `testdata/` changed.

## Verification
```
make gate-full
go test ./internal/checks -run '^(TestReplay|TestWorkType)'
go test ./internal/pipeline/replay/... ./internal/core/...
git log -p --grep='^WP-0062' -- internal/checks/budgets.txt
# replay-bytes-per-line: old and new values, new no more than old + 4.00
git log --oneline --grep='^WP-0062' -- testdata
# no output
```
