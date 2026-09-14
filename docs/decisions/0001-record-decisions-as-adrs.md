# ADR-0001: Record architecture decisions as ADRs

**Status:** Accepted

## Context
This project is developed primarily by AI coding agents working in long,
independent sessions with limited human review capacity. Decisions that live
only in conversation are re-litigated, silently contradicted, or invented from
scratch by the next agent. The project also previously carried a scope document
whose rules had no recorded rationale, which made them feel arbitrary and easy
to discard.

## Decision
1. Every architectural or scope decision MUST be recorded as a numbered file
   under `docs/decisions/`, named `NNNN-kebab-case-title.md`.
2. One decision per file. A file MUST NOT contain two independent decisions.
3. An ADR whose status is `Accepted` MUST NOT be edited to change its meaning.
   To change a decision, write a new ADR, set the new ADR's `Supersedes` field,
   and set the old ADR's status to `Superseded` with its `Superseded by` field
   filled in. Editing an accepted ADR for typos or broken links is allowed.
4. Every ADR MUST use the section set in `0000-template.md`: Context, Decision,
   Consequences, Out of scope, Assumption, Acceptance criteria, Dependencies.
5. ADRs MUST NOT contain dates, durations, schedules, milestones, sprint or
   phase names, or any statement about when work will happen. The
   `Dependencies` section expresses ordering only, in the form
   "ADR-NNNN must be implemented before this one".
6. `docs/decisions/INDEX.md` MUST list every ADR as a single line with its
   number, title and status. The index MUST be updated in the same commit that
   adds or supersedes an ADR.
7. The repository root agent instruction file MUST point to
   `docs/decisions/INDEX.md` as required reading.
8. Requirements MUST be written as numbers or prohibitions, not as adjectives.
   Words such as "modern", "fast", "intuitive", "beautiful" and "user-friendly"
   MUST NOT appear as requirements in any ADR.

## Consequences
- The decision history is readable in order and never rewritten.
- An agent can determine whether a proposed change contradicts an existing
  decision by reading the index and at most a few files.
- Decisions carry their rationale, so they can be revisited deliberately rather
  than discarded because nobody remembers why they exist.

## Out of scope
- ADRs do not cover implementation detail that can change without changing an
  interface, an artifact format, or a product guarantee.
- ADRs do not track work items, progress, or assignment.

## Assumption
The decision set stays small enough that reading the index remains cheap. If
the index exceeds roughly one hundred entries, grouping by area becomes
necessary.

## Acceptance criteria
- `docs/decisions/INDEX.md` exists and lists every file in `docs/decisions/`
  except the template.
- No file in `docs/decisions/` with status `Accepted` contains a date or a
  duration.
- The root agent instruction file references the index.

## Dependencies
None.
