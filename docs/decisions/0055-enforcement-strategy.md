# ADR-0055: Rules are enforced by tooling, and checker messages name the record

**Status:** Accepted

## Context
Implementation is produced continuously and reviewed sparsely. A rule that is
written but not enforced is, on average, absent. Many project rules cannot be
expressed in linter configuration — that a family declares its inputs, that no
record contains a date, that the committed bundle matches its source — but each
can be expressed as a short test.

## Decision
1. Rules expressible in linter or formatter configuration MUST be expressed
   there.
2. Rules not so expressible MUST be implemented as **checkers in the repository**
   that run as tests and fail the build on violation.
3. **Every checker failure message MUST name the record it enforces**, for
   example `ADR-0040: internal/metrics/coupling may not import
   internal/metrics/ownership`. A message that states only the mechanical fact
   is insufficient, because the reader must be able to reach the reasoning
   rather than look for a way around the rule.
4. Disabling, loosening or adding an exception to a checker MUST require a
   change to the governing record. An inline suppression MUST carry the record
   number and a justification.
5. Commit hooks MAY exist as a convenience. They MUST NOT be the only place a
   rule is enforced, because they can be skipped and may not be installed.
6. A checker MUST be added in the same change as the rule it enforces. A rule
   introduced without its checker accumulates violations that are later resolved
   by weakening the rule.

## Consequences
- Rules survive across sessions because they are mechanical rather than
  remembered.
- The decision set becomes reachable from failures, which is the main path by
  which it gets read at all.
- Weakening a rule is visible and requires justification.

## Out of scope
- Mandatory local hooks.
- Enforcement by review checklist.

## Assumption
Checkers are cheap enough to run within the gate budgets in ADR-0057.

## Acceptance criteria
- Every enforced rule listed in ADR-0056 has either a configuration rule or a
  checker.
- Every checker failure message contains a record number.
- No checker is disabled without a record change.

## Dependencies
ADR-0001 must be implemented before this one.
