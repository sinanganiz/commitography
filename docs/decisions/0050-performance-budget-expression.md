# ADR-0050: Duration budgets are ratios, memory budgets are absolute

**Status:** Accepted

## Context
ADR-0019 requires a performance budget enforced in continuous integration.
Shared runners vary in speed, so absolute duration thresholds either produce
false alarms or get loosened until they catch nothing. The regressions worth
catching — a subprocess per commit, an accidental quadratic — are algorithmic and
show up as ratios regardless of machine speed.

## Decision
1. Duration budgets MUST be expressed as a ratio against a calibration workload
   measured in the same run. Absolute duration thresholds MUST NOT be used as
   gates.
2. Memory budgets MUST be absolute, because memory does not vary with machine
   speed.
3. The following MUST be measured and recorded on every enforcing run:
   - full analysis duration, as a ratio;
   - incremental analysis duration, as a ratio;
   - replay peak memory, absolute and per tracked line;
   - checkpoint size, per tracked line;
   - git subprocess count;
   - report size;
   - frontend bundle size;
   - first response latency for an interactive request while a background job
     is running.
4. The last measurement in clause 3 MUST exist because it is the only
   verification of the interactive priority rule in ADR-0027 clause 2.
5. Budget values MUST live in a version-controlled file, governed by ADR-0054.

## Consequences
- Algorithmic regressions are caught independently of runner speed.
- The priority rule is verified rather than assumed.
- Measurements are comparable across runs without storing history.

## Out of scope
- Historical trend tracking, cross-branch regression comparison.
- Real-user measurement, which would require telemetry and is forbidden by
  ADR-0013.

## Assumption
A calibration workload exists whose duration tracks machine speed without being
affected by the changes under test.

## Acceptance criteria
- Every measurement in clause 3 is produced by the enforcing run.
- Introducing a git invocation per commit fails the budget on any runner.

## Dependencies
ADR-0019 and ADR-0027 must be implemented before this one.
