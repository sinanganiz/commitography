# ADR-0032: Every family reports a status; absence is never silent

**Status:** Accepted

## Context
Families can be skipped because an input is unavailable (ADR-0024), because the
mode forbids the capability (ADR-0029), or because the repository does not
support them. Other results are computed but unreliable — low conventional
commit ratios, unresolved identities, ownership divergence from blame. If a
skipped family is simply absent from the report, a consumer reads absence as
zero, which is the most damaging class of error in an analysis product.

## Decision
1. Every family in the catalogue MUST be present in every report, regardless of
   outcome.
2. Every family MUST carry a status with exactly one of three values:
   - `ok` — computed and reliable;
   - `skipped` — not computed, with a machine-readable reason code;
   - `degraded` — computed, with a machine-readable reason code and a
     confidence indicator.
3. A `skipped` family's data fields MUST be empty rather than null-filled or
   zero-filled, so that absence cannot be mistaken for a measured zero.
4. Reason codes MUST come from a finite enumerated set documented in the
   schema. Free-text reasons MUST NOT be the only form.
5. The interface MUST display a persistent indicator on every visualisation
   belonging to a `degraded` family, stating the reason.
6. The interface MUST NOT hide a `skipped` family. It MUST render it in an
   empty state with its reason, so the reader learns the capability exists.
7. Every displayed metric MUST have an available definition. A number MUST NOT
   appear without an explanation being reachable.
8. Where a metric is derived from a sample, an approximation, or a method that
   differs from a well-known reference implementation, the report MUST state
   the method. This applies in particular to replay-derived line ownership
   (ADR-0020).

## Consequences
- Consumers can distinguish "not measured" from "measured as zero".
- Readers discover capabilities that their configuration disabled instead of
  never learning they exist.
- The project's honesty-about-uncertainty position becomes a schema-level
  property rather than a single metric's footnote.

## Out of scope
- A separate diagnostics section detached from the families it describes.

## Assumption
Reason codes form a finite, enumerable set.

## Acceptance criteria
- A report produced without working tree access contains `hotspot` and
  `static-analysis` with status `skipped` and a reason code.
- No family is absent from any report.
- Every metric rendered in the interface has a reachable definition.

## Dependencies
ADR-0024 must be implemented before this one.
