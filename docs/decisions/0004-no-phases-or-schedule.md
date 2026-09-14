# ADR-0004: Scope is complete and unphased; no schedules are recorded

**Status:** Accepted

## Context
Implementation is performed continuously by AI coding agents rather than by a
human team with a capacity limit. Under those conditions, calendar planning
does not describe reality and creates documents that are wrong on arrival. The
real risk is not lateness but half-integrated, mutually inconsistent features
landing at the same time.

## Decision
1. Everything described in the accepted ADR set is in scope. There are no
   phases, no milestones, no releases-with-contents, and no deferred tiers.
2. No document in the repository MAY state when work will be done, how long it
   will take, or what belongs to which time period.
3. Ordering MUST be expressed only as implementation dependency: "A cannot be
   implemented before B". Dependency is a fact about the code, not a plan.
4. Every feature MUST have a definition of done expressed as acceptance
   criteria in its governing ADR. A feature without acceptance criteria MUST NOT
   be considered complete.
5. Priority MAY be expressed as a non-priority marker on a specific capability
   where an ADR says so explicitly (see ADR-0012 on static analysis). It MUST
   NOT be expressed as a date.

## Consequences
- Documents stay valid regardless of when they are read.
- Coordination between parallel agent sessions rests on the dependency graph
  and acceptance criteria rather than on a schedule.
- There is no concept of "later" that can be used to justify shipping a partial
  implementation of an accepted decision.

## Out of scope
- Version tags, changelogs and release notes are not schedules and are allowed.

## Assumption
Implementation capacity is effectively continuous and is not the limiting
factor. Specification clarity and verification are the limiting factors.

## Acceptance criteria
- No accepted ADR contains a date, duration, milestone or phase name.
- Every ADR describing a capability contains acceptance criteria.

## Dependencies
ADR-0001 must be implemented before this one.
