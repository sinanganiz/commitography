# ADR-0078: Scoped figures are projections of cells the report carries

**Status:** Accepted
**Note:** This record extends ADR-0018, ADR-0019, ADR-0074 and ADR-0077. It
supersedes none.

## Context
Several records need figures for part of a repository, computed from the stored
report alone:

- `docs/metrics.md` gives the temporal metrics a person scope and the work-type
  shares a projection, and the interpretation axes need person figures from
  seven families.
- ADR-0077 makes a person's interpretation a pure function of the report and
  the selection, so a person's figures must be derivable from the report.
- ADR-0018 clause 3 forbids merging identities from triggering a
  recomputation.
- ADR-0008 clause 2 generates Wrapped from the same report as the dashboard,
  and the year is to leave the analysis, so a year's figures must be derivable
  from the report too.

No record says how the report carries what these need. ADR-0018 solved it for
work type alone, with a breakdown that sums: merging two identities adds their
cells, and the result equals an analysis in which they were one identity. This
record makes that the rule for every figure narrower than the repository.

## Decision
1. A **scope** is an identity selection and a year. The selection is either
   every identity, or a set of individually represented identities
   (ADR-0018 clause 4) named by their `id`. The year is either every year, or
   one calendar year. The repository's figures are the figures of the scope of
   every identity and every year.
2. A metric is **scoped** where `docs/metrics.md` gives it a person scope, a
   year scope, or both. The catalogue states this for every metric it defines;
   a metric it does not mark is a repository metric only.
3. A family with a scoped metric carries **cells** in its namespace, as the
   metric `cells`. A cell holds the family's inputs for one key. The key is an
   identity, or the aggregate bucket, and a year; a family whose figures relate
   two identities keys its cells by both, as ADR-0018 clause 1 does.
   `docs/metrics.md` defines each family's cell fields in a table headed
   `Cell field`.
4. **Cells are additive.** For every cell field, the sum of two cells is
   defined and exact: counts and histograms add; date lists merge, adding the
   counts of equal dates; a maximum keeps the greater under the tie rule of
   `docs/metrics.md` section 1; a list bounded at N keeps the first N of both
   lists under its ordering. A value whose sum is not exact is not a cell field.
5. **A scoped figure is one derivation over the sum of the cells in its
   scope.** The family package exports that derivation as one pure function of
   the family's section and a scope. The repository value of a scoped metric is
   that function over every cell, and the family computes it no other way.
6. **Nothing else computes a scoped figure.** The interpret stage, the server
   and every other consumer obtain a scoped figure by calling the owning
   family's function over a stored report. The call reads only the report, so
   it is not a recomputation under ADR-0018 clause 3, and it holds no session
   state.
7. **Projection equals recomputation** for every scoped metric on every
   fixture:
   - the figure for a selection of two or more identities equals the figure
     for the one identity they become in an analysis where the configuration
     merged them beforehand;
   - the figure for a year equals the family computed directly over that
     year's analysed commits, or that year's line events.

   This extends the invariant of ADR-0019 clause 3 from the work-type breakdown
   to every scoped metric.
8. **The year of a cell** is the local calendar year, by the configured date
   source, of the analysed commit a commit figure counts, or of the editing
   commit a line event belongs to. A figure about the state at the analysed
   commit, such as surviving lines, has no event year; where its cells carry a
   year, it is the year the line was last written, and the catalogue says so.
9. **The identity bound is computed once.** The individually represented
   identities are selected in `core`, and the identities section and every
   family's cells use that one selection, so an identity is individual in every
   part of the report or in none. The aggregate bucket's cells enter only a
   scope of every identity; no selection names the aggregate bucket.
10. **A metric whose inputs cannot be summed exactly has no scoped form.** Where
    the catalogue needs a scoped form of such a metric, it redefines the metric
    over inputs that can be summed, and the family version increments
    (ADR-0031 clause 2).
11. **Interpretation takes a scope.** ADR-0077 clause 3 reads: the result is a
    pure function of the report, the scope and the taxonomy version. The report
    still carries only the interpretation of the scope of every identity and
    every year (ADR-0077 clause 1).
12. **Replay records line events by year as well.** The inputs of ADR-0074
    clause 8 are recorded for each pair of editing identity and previous owner
    and for each year of the editing commit, so that the work-type cells carry
    a year. The year is a property of each event, not an analysis parameter;
    replay still applies none (ADR-0074 clause 9).
13. **Cells are repository content.** They are keyed by identity digest, as the
    work-type breakdown of ADR-0018 clause 1 is, and they persist with the
    report in every mode. A figure derived for a scope that names identities is
    person-scoped content, governed by ADR-0033 clause 6 and ADR-0077 clause 4.
    A figure for the scope of every identity is repository content, whatever
    its year.

## Consequences
- A merged selection and a single year receive exact figures from the stored
  report, with no stage re-run.
- Wrapped reads a year's figures from the report the dashboard reads
  (ADR-0008 clause 2), so the year no longer needs to be an analysis parameter.
- A family has one derivation for its repository and its scoped figures, so the
  two cannot disagree.
- The report grows. Cells are bounded by the individually represented identities
  plus one bucket, times the years of history; date lists are bounded by the
  number of analysed commits.
- Some definitions change so that they can be summed. A gap between commits,
  for example, is measured between active dates rather than between
  timestamps.
- A figure that needs every commit's timestamp, such as the most commits in one
  minute, has no scoped form.

## Out of scope
- A selection spanning several repositories (ADR-0023 clause 5, ADR-0025).
- The API route and response shape of a scoped figure, which the server
  packages define.
- How the frontend displays a scoped figure.
- Which metrics are scoped. The catalogue decides that, family by family.

## Assumption
The report with its cells stays small enough to be stored and served whole. Its
cells are bounded by the identity bound, the years of history and the number of
analysed commits. If a real repository shows otherwise, a later record changes
how the report is stored, not this projection rule.

## Acceptance criteria
- Every scoped metric in `docs/metrics.md` belongs to a family that carries
  cells, and every cell field is defined in a `Cell field` table.
- On every fixture, the repository value of every scoped metric equals its
  family's derivation over every cell.
- On every fixture, the figure for each two-identity selection equals the
  figure for the merged identity of an analysis with the two merged by
  configuration, and the figure for each year equals the direct computation
  over that year.
- The identities section and every family's cells individually represent the
  same identities.

## Dependencies
ADR-0018, ADR-0074, ADR-0076 and ADR-0077 must be implemented before this one.
