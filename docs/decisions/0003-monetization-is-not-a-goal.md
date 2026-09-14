# ADR-0003: Monetization is not a goal

**Status:** Accepted

## Context
The buyers who pay for engineering analytics are managers purchasing visibility
into other people's work. Individual developers do not pay for analytics about
themselves. Pursuing revenue would therefore require building a manager-facing
product and a sales motion, which conflicts with the project's stated goal of
reputation and with its bottom-up, developer-facing design.

## Decision
1. The product MUST NOT contain paid tiers, usage quotas tied to payment,
   licence keys, trial expiry, or any feature gated behind purchase.
2. The product MUST NOT contain billing, subscription or entitlement code.
3. No feature MAY be designed primarily to drive conversion to a paid offering.
4. Features that would only make sense in a commercial product MUST NOT be
   built merely because they would be commercially useful later.

## Consequences
- Onboarding can be frictionless because there is no conversion funnel to
  protect.
- Success is measured by adoption and reach, not revenue.
- The absence of revenue is a deliberate position, not an unresolved gap, and
  should not be re-opened by implementers.

## Out of scope
- Donations and sponsorship links are not features and are not covered here.
- A future commercial product built on the same engine is not forbidden, but it
  is covered by ADR-0002 clause 3 and requires its own repository.

## Assumption
The project's value to its author is reputation and demonstrated capability.

## Acceptance criteria
- No billing, entitlement, licence-key or quota-enforcement code exists in the
  repository.

## Dependencies
None.
