# ADR-0054: Budgets are gates, and loosening one requires a record

**Status:** Accepted

## Context
Without a stated response, the most likely reaction to a broken budget is to
raise the budget. In a commit that also contains the change which broke it, that
edit looks like ordinary configuration maintenance.

## Decision
1. Exceeding a performance budget MUST fail the build. A warning that does not
   block is equivalent to no budget.
2. Budget values MUST live in a version-controlled file.
3. **Loosening a budget value MUST require a decision record** stating the
   measured justification. Tightening a budget does not.
4. Exceeding a budget MUST NOT be resolved by deleting, skipping or disabling
   the check. Acceptable responses are: fix the regression, reduce the fixture,
   or move the check from the fast gate to the full gate (ADR-0057).
5. Profiling MUST be available as a manual target. Automatic profile collection
   and storage in continuous integration MUST NOT be implemented.

## Consequences
- A performance regression cannot be absorbed by a quiet threshold edit.
- The rule matches the golden-file rule in ADR-0019 and the enforcement rule in
  ADR-0055, so there is one pattern rather than three.

## Out of scope
- Automated profiling infrastructure and stored profile history.

## Assumption
Budget loosening is rare enough that requiring a record is not obstructive. If
it becomes routine, the budgets were wrong and should be re-derived once.

## Acceptance criteria
- Budget values are in a version-controlled file.
- A build exceeding a budget fails.
- Profiling is available as a manual target and is not run automatically.

## Dependencies
ADR-0050 must be implemented before this one.
