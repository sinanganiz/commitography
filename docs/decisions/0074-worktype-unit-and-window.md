# ADR-0074: What work-type classification counts, and where its window applies

**Status:** Accepted
**Note:** This record refines ADR-0020 clause 4. It supersedes it in no part.

## Context
ADR-0020 clause 4 classifies a changed line by its previous owner and age, but
does not define the unit. A difference presents a rewrite as a removed line and
an added line, and an added line is by construction never in the ownership map,
so read literally every addition is new work and the other three classes can
only ever describe deletions.

The readings diverge sharply. Rewriting one's own recent code is entirely rework
under one and half rework, half new work under the other.

Separately, fixing the class of each line during replay ties the result to the
recency window, so changing the window requires replaying history. ADR-0020's
own consequences promise the opposite: that changing a metric definition
requires re-running aggregation only.

## Decision
1. The unit is a **line-level change event** in an analysed commit's
   difference. There are three kinds:
   - **replacement** — an added line paired with a removed line;
   - **deletion** — a removed line with no paired added line;
   - **addition** — an added line with no paired removed line.
2. Within a changed block of R removed and A added lines, the first min(R, A)
   removed lines pair with the first min(R, A) added lines **by position**. The
   block comes from the fixed in-process algorithm of ADR-0073 clause 7, so the
   pairing is deterministic.
3. A replacement or a deletion is classified by the **removed line's** previous
   owner and age. An addition is **new work**.
4. **Each event counts once.** Rewriting one's own recent line is one rework
   event, never one rework event and one new work event.
5. A line's age is the editing commit's day minus the replaced line's authoring
   day, both by the configured date source, at day resolution. A line is
   **recent when its age is less than the window**; equal or greater is legacy.
6. A merge counted as analysed contributes only the lines it owns, which are the
   lines matching no parent (ADR-0073 clause 5).
7. A file whose previous or current version exceeded the size cap of ADR-0072
   clause 5 contributes **no events**, and the family is `degraded` with reason
   `limit_reached_size`. Its lines are not treated as additions.
8. **Replay records the inputs and applies no window.** For each pair of editing
   identity and previous owner it records a histogram of the ages of replaced and
   deleted lines, and for each editing identity the count of additions.
9. **The window is applied when the family is aggregated.** Changing the window
   therefore re-runs aggregation only, and neither replay state nor its
   checkpoint depends on the window.

## Consequences
- Rewriting one's own recent code reads as rework, which is what it is.
- Changing the window is cheap and does not invalidate replay state or its
  checkpoint.
- The report still carries class counts per cell, as ADR-0018 clause 1
  requires, and those counts are sufficient for identity projection: once a line
  is recent, help and rework differ only in whether editor and owner are the
  same, so merging two identities moves their recent cross-cells from help to
  rework and leaves legacy untouched.
- Replay needs no analysis parameter, so replay state is a function of history
  and identity resolution alone.

## Out of scope
- Similarity-based pairing, or detecting a moved line within a block.
- Storing line ages in the report. They remain replay state.

## Assumption
Positional pairing within a block approximates which line replaced which closely
enough for the figures to be meaningful. The difference from similarity-based
pairing is not measured; if it proves to matter, a later record introduces it.

## Acceptance criteria
- On a purpose-built fixture, every changed line's kind, editor, previous owner
  and age equal the values known by construction.
- Rewriting one's own recent line produces a replacement event and no addition
  event.
- Changing the window leaves replay state byte-identical and changes only the
  aggregated class counts.
- An oversized file contributes no events and marks the family `degraded`.

## Dependencies
ADR-0018, ADR-0020, ADR-0072 and ADR-0073 must be implemented before this one.
