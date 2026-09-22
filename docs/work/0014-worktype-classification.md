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
   and previous owner, a histogram of replaced and deleted line ages; for each
   editing identity, the count of additions. **Apply no window.** Replay reads
   no analysis parameter.
3. Only **analysed commits** produce events (ADR-0073 clause 6). A merge counted
   as analysed contributes only the lines it owns. Because the year deviation
   recorded against WP-0017 acts through the analysed flag, events and the
   identities section stay consistent with each other until WP-0017 removes it.
4. A file whose previous or current version exceeded the size cap contributes
   **no events**, and the `worktype` family is marked `degraded` with
   `limit_reached_size`. Replay currently gives such lines to the commit author
   as though they were new; this package stops that for classification.
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
`internal/checks/**`, `testdata/**` fixtures and the fixture manifest, and
`docs/metrics.md` **section 8 only**.
**Must not touch:** `internal/metrics/**`, `internal/pipeline/run.go`,
`internal/pipeline/aggregate/**`, `internal/pipeline/collect/**`, `cmd/**`,
`docs/decisions/**`, any other section of `docs/metrics.md`, golden files.

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
- An oversized file contributes no events and marks the family `degraded`.
- No blame invocation occurs in the path.
- `docs/metrics.md` section 8 states the definition.
- **Every golden file is byte-identical**: classification inputs are replay
  state, and the report does not change until WP-0024.

## Verification
```
make gate-full
go test ./internal/checks ./internal/pipeline/replay -run '^TestWorkType'
git grep -n 'blame' -- internal/pipeline/replay     # no output
git diff --stat <base> -- testdata/golden            # empty
```
