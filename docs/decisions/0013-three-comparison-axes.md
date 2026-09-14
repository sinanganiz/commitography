# ADR-0013: Comparison uses three axes and no global corpus

**Status:** Accepted

## Context
Percentile statements against a global population would require the project to
collect and retain analysis results from many repositories. That conflicts with
the project's position of collecting nothing, and it would make a primary
self-hosted capability depend on a secondary public deployment. Without a
corpus, however, an archetype cannot be assigned by relative ranking: in a
three-person repository the latest committer would become the night owl, and in
a single-contributor repository there is nobody to compare against.

## Decision
1. There MUST be no global corpus, no baseline file derived from other users'
   repositories, and no telemetry of any kind.
2. Comparison MUST be expressed on exactly three axes, each with a distinct
   data source:
   - **Archetype** — assigned from **absolute, hand-calibrated thresholds**.
   - **Badges and superlatives** — computed from **relative standing within the
     same repository**.
   - **Trend** — computed from **the same subject's own earlier analyses**,
     available only where version history exists (ADR-0011).
3. The interface MUST NOT state or imply a population percentile, a world
   ranking, or a comparison against other users, because no such data exists.
4. Absolute thresholds MUST live in a human-readable, versioned data file
   rather than in code.
5. Where a badge requires in-repository comparison and the repository has only
   one contributor, the badge MUST be reported as `skipped` with a reason
   (ADR-0032), not silently omitted and not awarded by default.

## Consequences
- Archetype assignment works in a single-contributor repository; badges may
  not, and say so.
- Thresholds are a calibration artifact that can be tuned without touching
  code, and their values are auditable.
- Trend is the axis that gives repeat value to registered repositories.

## Out of scope
- Percentiles, global benchmarks, industry comparisons and cross-user
  statistics are forbidden.

## Assumption
Hand-calibrated thresholds produce a usable distribution of archetypes across
typical repositories. This is unverified until measured (see ADR-0030).

## Acceptance criteria
- No network call transmits analysis results anywhere.
- No shipped data file contains statistics derived from third-party
  repositories.
- Archetype assignment produces a result for a repository with exactly one
  contributor.

## Dependencies
None. ADR-0030 depends on this one.
