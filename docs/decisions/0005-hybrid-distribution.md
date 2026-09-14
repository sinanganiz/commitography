# ADR-0005: Hybrid distribution — self-hosted primary, public instance secondary

**Status:** Accepted

## Context
Two distribution shapes were considered. A hosted platform where users connect
their code host accounts would require storing long-lived repository access
credentials for every user; this is the failure mode that has already produced
real breaches in this product category, and it is not a liability this project
should carry. A self-hosted-only tool avoids that entirely but has no surface
that can be reached by someone who has not installed anything.

## Decision
1. There MUST be exactly one codebase and one container image, operated in one
   of two modes defined in ADR-0029.
2. The self-hosted deployment is the primary product. Every capability defined
   in the ADR set MUST be available in self-hosted operation.
3. A public instance operated by the project is a secondary deployment of the
   same image, restricted to the capability set in ADR-0029.
4. The public instance MUST NOT analyse repositories that require
   authentication.
5. The public instance MUST NOT pre-compute analyses for repositories that no
   visitor has requested. Analysis is performed on demand only.
6. Because analysis is on demand, the public instance MUST present the same
   asynchronous job experience as self-hosted operation: queued state, stage
   progress, and cancellation.

## Consequences
- No user repository credential is ever stored by the project (see ADR-0016).
- The first visitor for a given repository and commit pays the full analysis
  cost. A persistent report cache is therefore mandatory, not optional
  (ADR-0017).
- Because public-instance URLs do not exist until someone triggers an analysis,
  the shareable artifact cannot be a URL (ADR-0015).

## Out of scope
- Multi-tenant hosting with per-user isolation is not built.
- Pre-computation, crawling, or bulk import of public repositories is
  forbidden.

## Assumption
On-demand analysis plus caching keeps public-instance cost bounded on a single
machine.

## Acceptance criteria
- A single container image runs in both modes, selected by one switch.
- No code path pre-computes or enumerates repositories that were not requested.

## Dependencies
None. ADR-0029 depends on this one.
