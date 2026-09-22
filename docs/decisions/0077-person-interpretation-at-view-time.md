# ADR-0077: Person interpretation is computed at view time

**Status:** Accepted
**Note:** This record extends ADR-0014 and ADR-0023. It supersedes neither.

## Context
ADR-0014 clause 6 requires archetypes and badges for both subjects of
ADR-0023: the repository, and a person within it. A reader may declare several
identities to be one person (ADR-0010 clause 3), and that person's figures are a
projection of the stored breakdown (ADR-0018). An archetype computed ahead of
time for each identity cannot serve a merged selection.

In public mode, person-scoped content must never persist (ADR-0033 clause 6),
yet the report is cached. A person's archetype held inside the report would put
a named individual's label into a persisted public artifact.

## Decision
1. **The report carries only the repository's interpretation.** It holds no
   person-subject archetype, badge or prose.
2. **A person's interpretation is computed on request**, by running the
   interpret stage over the stored report and the selected identity set, using
   the projected figures of ADR-0018. ADR-0020 clause 6 already makes interpret
   re-runnable over a stored report.
3. The result is a pure function of the report, the selection and the taxonomy
   version, and is tested as one.
4. **In public mode the result is never persisted**, has no URL of its own and
   is not indexable (ADR-0033 clause 6).
5. In server mode it MAY be cached, keyed on the report's cache key and the
   selection, and never inside the report.
6. Language-model prose for a person is produced the same way, and only where
   ADR-0039 permits it; never in public mode.
7. The command-line output (ADR-0034) therefore carries no person
   interpretation. The web application always runs against the server
   (ADR-0036), which computes it.

## Consequences
- A merged selection receives a correct archetype.
- No named person's archetype sits in a persisted public artifact.
- The report is identical across modes (ADR-0021), because it never contains
  person interpretation.

## Out of scope
- The API route and response shape, which the server packages define.
- Interpretation for a selection spanning several repositories (ADR-0023
  clause 5).

## Assumption
Interpretation is fast enough to compute on each view. It is an ordered
first-match evaluation over precomputed axes, so it is.

## Acceptance criteria
- No report contains a person-subject interpretation.
- The interpretation for a two-identity selection equals the interpretation
  computed from a report in which those identities were merged before analysis.
- In public mode, no person interpretation is written to any store.

## Dependencies
ADR-0014, ADR-0018, ADR-0020 and ADR-0033 must be implemented before this one.
