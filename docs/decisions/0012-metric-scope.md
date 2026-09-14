# ADR-0012: Metric scope includes static analysis as a non-priority capability

**Status:** Accepted

## Context
The metric catalogue was reopened from its previous repository-only form. Four
families beyond the original catalogue were considered: work-type
classification, AI authorship archaeology, churn-by-complexity hotspots, and
true static analysis.

## Decision
1. The following metric families are in scope and MUST be implemented:
   temporal, commit size, commit messages, file activity, change coupling,
   ownership, work type, AI archaeology, hotspot, static analysis.
2. Static analysis is explicitly marked **non-priority**. It MUST NOT be
   implemented before every other family in clause 1 satisfies its acceptance
   criteria. No date is attached to this; it is an ordering constraint under
   ADR-0004 clause 5.
3. Static analysis, when implemented, MUST be a metric family under ADR-0024
   like any other, declaring `worktree` as its input. It MUST NOT be allowed to
   restructure the aggregation stage.
4. Metrics requiring a hosting provider API are out of scope by ADR-0007 and
   MUST NOT be added to this catalogue.

## Consequences
- Work-type classification is the substance of the person lens; without it the
  lens reduces to commit counts, which is the commodity the product is trying
  not to be.
- AI archaeology is derivable from commit trailers and authorship patterns and
  therefore costs little beyond its own rules.
- Hotspot analysis requires a language-agnostic complexity proxy, not a parser.

## Out of scope
- Language-specific parsing, linting, rule engines and quality gates are not
  part of the hotspot family.
- Any metric that ranks or scores individuals (ADR-0009).

## Assumption
A language-agnostic complexity proxy combined with churn is informative enough
to be worth shipping without per-language analysis.

## Acceptance criteria
- Every family in clause 1 exists in the catalogue defined by ADR-0024 with a
  declared input.
- No family reads a hosting provider API.

## Dependencies
ADR-0024 must be implemented before this one.
