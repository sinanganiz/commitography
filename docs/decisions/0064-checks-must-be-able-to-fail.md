# ADR-0064: A check that cannot fail is not a check

**Status:** Accepted

## Context
The fast gate was found to be running roughly forty fixture-dependent tests that
all skipped, because the gate never generated the fixtures and the tests treat a
missing fixture as a reason to skip rather than to fail. The gate reported
success. Nothing in the repository was wrong in a way any output revealed.

This is the failure mode the entire verification regime exists to prevent, and
it had occurred inside the regime itself. A check whose preconditions are absent
does not report a problem; it reports nothing, and nothing reads as success.

## Decision
1. A gate MUST establish every precondition its checks require before running
   them. A check whose precondition is absent MUST NOT run silently.
2. **A missing precondition MUST fail, never skip.** A test or checker that
   cannot find the data it needs reports a failure naming what is missing.
   Treating absent input as a reason to skip is forbidden.
3. A skip is permitted only for a condition that is genuine and permanent in
   that environment, such as a platform that cannot express the case under test.
   Every such skip MUST state its reason, and the reason MUST NOT be the absence
   of something the gate was supposed to provide.
4. Every gate MUST report what it executed: the number of checks run, passed,
   failed and skipped. A gate that reports only success is not auditable.
5. Where a checker is deliberately narrowed because its subject is not yet
   complete, the narrowing MUST be recorded next to the rule and MUST name the
   package that widens it (ADR-0063 clause 6). A narrowing that is not recorded
   is indistinguishable from a checker that does nothing.
6. When a check is added, its failure MUST be demonstrated before it is
   accepted: introduce the violation, observe the failure, revert. A check never
   observed failing has not been shown to work.

## Consequences
- A green gate means the checks ran, not that they were absent.
- Fixture generation becomes part of gate setup rather than a local convenience.
- Clause 6 costs one extra step per check and is the only thing that
  distinguishes a working checker from a well-named no-op.

## Out of scope
- Coverage thresholds. This record is about checks executing at all, not about
  how much they cover.
- Forbidding skips outright; clause 3 keeps the legitimate case.

## Assumption
Every precondition a gate needs can be established by the gate. A precondition
that cannot be — a credential, a network service — indicates a check that does
not belong in a gate.

## Acceptance criteria
- Removing a fixture causes the tests that need it to fail, not skip.
- Each gate prints counts of checks run, passed, failed and skipped.
- No check in either gate skips for a reason the gate could have prevented.
- Every checker in ADR-0063 has been observed failing at least once.

## Dependencies
ADR-0019, ADR-0057 and ADR-0063 must be implemented before this one.
