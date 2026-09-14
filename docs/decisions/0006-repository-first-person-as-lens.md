# ADR-0006: The repository is the unit of analysis; the person is a lens

**Status:** Accepted

## Context
The engine's distinguishing capability is depth of git history analysis —
change coupling, ownership concentration, bus factor, line age. These are
meaningful at repository scope. Tools that make the individual the unit of
analysis are limited to what a hosting provider's contribution API exposes and
are numerous and interchangeable. At the same time, the object people share is
personal, so a purely repository-scoped product has no personal hook.

## Decision
1. The unit of analysis MUST be a single repository. Every stored report
   describes one repository at one commit under one analysis configuration.
2. Person-scoped figures MUST be derived from within a repository analysis.
   They MUST NOT be produced by aggregating a hosting provider's contribution
   data.
3. The person view MUST be expressed as a lens over the repository view, not as
   a separate top-level section (see ADR-0028).
4. Cross-repository person aggregation is permitted only through ADR-0025 and
   MUST NOT change the unit of analysis.

## Consequences
- The report schema is repository-shaped. Person figures are a breakdown inside
  it, never a parallel top-level document.
- Metrics that only exist at repository scope remain the product's primary
  content even when a person lens is active.

## Out of scope
- A standalone developer profile page that is not anchored to a repository is
  not built.

## Assumption
What interests a reader is their role inside a project, not their global
totals.

## Acceptance criteria
- `report.json` has a single repository as its subject.
- No metric is computed from a hosting provider's contribution or activity API.

## Dependencies
None.
