# WP-0014: Work-type classification inputs

**Area:** pipeline
**Implements:** ADR-0074, ADR-0020, ADR-0073, ADR-0019, ADR-0062
**Requires:** WP-0013

## Goal
For every line-level change in an analysed commit, replay records the kind of
change, the editing identity, the previous owner and the replaced line's age, as
per-pair age histograms and per-editor addition counts, applying no window and
invoking no blame.

## Why this package was rewritten

The first version fixed each line's class during replay, which needed the
recency window inside replay and would have forced a replay for every window
change. ADR-0074 moves the window to aggregation and defines what a changed line
is. This package now produces the inputs; the `worktype` family turns them into
classes (WP-0024).

## In scope
1. Implement ADR-0074 clauses 1 to 7 in the replay walk:
   - three event kinds: replacement, deletion, addition;
   - positional pairing of the first min(R, A) removed and added lines in each
     changed block;
   - a replacement or deletion carries the removed line's previous owner and
     age; an addition carries neither;
   - each event counts once;
   - age is the editing commit's day minus the replaced line's authoring day, by
     the configured date source, at day resolution.
2. **Record inputs only** (ADR-0074 clause 8): for each pair of editing identity
   and previous owner, **one age histogram for replacements and one for
   deletions**, whose sum is the histogram clause 8 describes; for each editing
   identity, the count of additions. **Apply no window.** Replay reads no
   analysis parameter beyond those it already reads to decide which lines exist
   and who owns them — path exclusion, date source and identity resolution — and
   **never the recency window**.
3. Only **analysed commits** produce events (ADR-0073 clause 6).
3a. **Record the year deviation.** The year restriction is applied in
   aggregation, not through the analysed flag, so with a year set, replay's
   events include other years while the identities section does not. Replay
   cannot correct this without reading the year, which ADR-0074 forbids. Record
   the gap next to the code and in the linter's tracking list, naming
   **WP-0017**, which removes the year from the analysis altogether.
4. A file whose previous or current version exceeded the size cap contributes
   **no events**. Replay currently gives such lines to the commit author as
   though they were new; this package stops that for classification. **Carry
   `limit_reached_size` in replay state**; the family cannot change in this
   package, and WP-0024 turns the carried reason into the family's `degraded`
   status.
5. **No blame invocation** anywhere in this path.
6. Add a **purpose-built fixture** in which the kind, editor, previous owner and
   age of every changed line are known by construction. It must contain at
   least: a replacement of one's own recent line; a replacement of another
   identity's recent line; a replacement of an old line; a pure deletion; a pure
   addition; **rewrites between one person's two addresses inside the window**,
   so that a naive summation over identities gives the wrong answer; a line
   whose age equals the default window and one a day younger; and a merge with a
   conflict resolution. Record it in the fixture manifest.
7. State the definition concretely in `docs/metrics.md` section 8: the three
   event kinds, positional pairing, the age rule, the boundary, merges, and the
   oversized-file rule (ADR-0062 clause 2).
8. Every test this package adds is named with the prefix `TestWorkType`, so that
   its verification command matches nothing else.

## Settled details

1. **Merges, when counted as analysed.** Positional pairing runs over the whole
   changed block against the first parent. An addition or replacement is
   recorded only where the added line belongs to the merge itself, meaning it
   matches no parent (ADR-0073 clause 5). **A deletion is recorded only for a
   line present in every parent's version of the file and absent from the
   result**; any other difference was made by a parent's own commit and is
   already recorded there. If any parent's version of a file exceeded the size
   cap, that file produces no events.
2. **Binary.** A change to or from a binary version produces no events,
   matching how effective lines are counted.
3. **Version.** Editing section 8 does not increment the `worktype` family's
   version: the family computes nothing yet. ADR-0031 clause 2 applies from
   WP-0024, which is the first package to compute it.

## Out of scope
- **Applying the window, producing class counts, the report breakdown, the
  bound at 200, repository shares and the projection checker.** All of that is
  the `worktype` family, WP-0024.
- Sharing the top-200 identity selection with the identities section. The
  breakdown that needs it is WP-0024's.
- Similarity-based pairing within a block (ADR-0074, out of scope).
- Changing what replay records for ownership. Only classification inputs are
  added here.

## Files
**May create or modify:** `internal/pipeline/replay/**`, `internal/core/**`,
`internal/checks/**`, `testdata/**` fixtures and the fixture manifest,
`testdata/golden/<the new fixture>.json` **as a new file only**,
`.golangci.yml` **for the year deviation entry only**, and `docs/metrics.md`
**section 8 only**.
**Must not touch:** `internal/metrics/**`, `internal/pipeline/run.go`,
`internal/pipeline/aggregate/**`, `internal/pipeline/collect/**`, `cmd/**`,
`docs/decisions/**`, any other section of `docs/metrics.md`, **any existing
golden file**.

## Steps
1. Write the fixture first, with the expected event for every changed line
   written beside it.
2. Implement the event kinds and positional pairing against it.
3. Record age histograms and addition counts in replay state.
4. Add the oversized-file exclusion.
5. Update section 8.

## Definition of done
- For every changed line in the fixture, the recorded kind, editor, previous
  owner and age equal the expected values.
- Rewriting one's own recent line produces a replacement event and no addition
  event.
- The line whose age equals the window is recorded with exactly that age, and
  the line a day younger with one less.
- **Changing `recency_window_days` leaves replay state byte-identical.**
- An oversized file contributes no events and replay state carries
  `limit_reached_size` for it.
- In the fixture's merge, a line present in every parent and removed by the
  merge is recorded as a deletion by the merge author; a line removed on only
  one side is not.
- The year deviation is recorded and names WP-0017.
- No blame invocation occurs in the path.
- `docs/metrics.md` section 8 states the definition.
- **Every existing golden file is byte-identical**, and the only golden file
  added is the new fixture's. Classification inputs are replay state, so no
  existing report changes until WP-0024.

## Verification
```
make gate-full
go test ./internal/checks ./internal/pipeline/replay -run '^TestWorkType'
git grep -n 'blame' -- internal/pipeline/replay     # no output
git diff --stat --diff-filter=a <base> -- testdata/golden   # empty: no existing golden changed
git diff --stat --diff-filter=A <base> -- testdata/golden   # exactly one added file
```
