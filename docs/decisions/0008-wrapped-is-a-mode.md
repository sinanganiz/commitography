# ADR-0008: Wrapped is a mode of the product, not the product

**Status:** Accepted

## Context
A year-in-review presentation is seasonal: it is meaningful once a year and
gives no reason to open the product at other times. A product whose hero
artifact is seasonal has no retention. Conversely, omitting it removes the only
naturally shareable surface the product has.

## Decision
1. The repository dashboard is the primary surface. Wrapped MUST NOT be the
   only way to read an analysis.
2. Wrapped MUST be generated from the same report as the dashboard. It MUST NOT
   have its own analysis path, its own metric implementations, or its own
   engine.
3. Wrapped MUST be a separate full-screen presentation surface with its own
   visual language, not a tab inside the dashboard (see ADR-0028).
4. Wrapped MUST accept a year parameter and MUST refuse to render for a year
   with fewer than 10 analysed commits, reporting the reason.

## Consequences
- Metric work done for the dashboard is automatically available to Wrapped.
- The two surfaces can diverge visually without diverging numerically.

## Out of scope
- A separate brand, domain or codebase for Wrapped is not created.
- Video or animation export is not built (see ADR-0015).

## Assumption
Readers will open the dashboard outside the seasonal window; Wrapped alone
would not sustain use.

## Acceptance criteria
- Wrapped and the dashboard read the same report artifact and produce identical
  numbers for the same metric.
- Requesting Wrapped for a year below the commit threshold returns an explicit
  refusal, not an empty page.

## Dependencies
ADR-0021 must be implemented before this one.
