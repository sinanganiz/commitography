# ADR-0009: Individual metrics are self-first and mode-dependent

**Status:** Accepted

## Context
Per-person figures are required for the person lens and for Wrapped. The same
figures, presented as a ranked comparison between named colleagues, turn the
product into a surveillance tool, which carries legal exposure and destroys
credibility with the audience the product is built for. The distinguishing
question is not whether a number about a person exists, but who reads it about
whom.

## Decision
1. In self-hosted operation, per-person figures MUST be available to the
   operator. The product MUST NOT technically restrict this, because the data
   never leaves the operator's machine and the restriction would be removable
   in a fork regardless.
2. In public mode, per-person content MUST be visible only to the visitor who
   selected that identity, MUST exist only for the duration of their session,
   and MUST NOT be addressable by URL or indexable (see ADR-0033).
3. No default view MAY present contributors as a list ordered by output volume.
   Where contributors are listed, the default ordering MUST be first commit
   date ascending.
4. Leaderboard, score, rating, rank and grade presentations of contributors
   MUST NOT be built.
5. Every person-scoped figure displayed MUST state its definition, as required
   by ADR-0032 for all metrics.

## Consequences
- The product is capable of per-person reporting and does not pretend
  otherwise.
- The interface does not encourage comparison between named individuals, which
  is a design position that can be verified by inspection.

## Out of scope
- Consent workflows, per-person opt-out and access control lists are not built
  in self-hosted operation.
- Manager-facing reporting features are not built.

## Assumption
A self-hosted operator is responsible for how they use data about their own
team; the project does not carry that responsibility on their behalf.

## Acceptance criteria
- No view sorts contributors by commit count, lines changed, or any output
  volume by default.
- In public mode, no route exists that returns person-scoped content for a
  named identity.

## Dependencies
ADR-0029 and ADR-0033 must be implemented before this one.
