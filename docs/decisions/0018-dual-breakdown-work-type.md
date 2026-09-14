# ADR-0018: Work-type data is stored as a dual breakdown by editor and prior owner

**Status:** Accepted

## Context
Two work-type classes depend on a pair of identities rather than on one:
rework is an author changing their own recent line, and help-others is an
author changing somebody else's recent line. When a reader later declares that
two contributor identities are the same person (ADR-0010), figures computed
under the assumption that they were different people are wrong, and summing
them does not repair the error.

## Decision
1. Metrics whose classification depends on the identity of the previous owner
   of a changed line MUST be stored in the report as a two-dimensional
   breakdown: editing identity by previous-owner identity.
2. Merging identities MUST be implemented as a projection over that breakdown.
   The result MUST be exactly equal to the result of a full recomputation with
   those identities pre-merged. This is a required invariant test (ADR-0019).
3. Identity merging MUST NOT trigger a server-side recomputation and MUST NOT
   require stored session state.
4. To bound the breakdown, at most the 200 identities with the most analysed
   commits MUST be represented individually; all remaining identities MUST be
   folded into a single aggregate bucket. The threshold MUST be recorded in the
   report.
5. Any metric family that later becomes dependent on an identity pair MUST use
   this same representation.

## Consequences
- Multi-identity selection produces correct figures rather than approximations.
- The breakdown costs cells proportional to the square of the represented
  identity count, bounded at 200 by clause 4.
- Merging identities inside the aggregate bucket cannot be resolved exactly;
  the report states the bucket threshold so this limitation is visible.

## Out of scope
- Pre-analysis identity resolution as the only merge mechanism; `.mailmap` and
  configuration overrides remain available but are not a substitute.

## Assumption
Work-type classification is currently the only identity-pair-dependent family.

## Acceptance criteria
- Projecting a merge of two identities equals full recomputation with those
  identities pre-merged, on a fixture containing an author committing under two
  addresses.
- A repository with more than 200 contributors produces a bounded breakdown and
  reports the threshold.

## Dependencies
ADR-0020 must be implemented before this one.
