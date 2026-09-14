# ADR-0011: Server mode with a persistent repository registry

**Status:** Accepted

## Context
A single-shot analysis tool cannot answer how a repository changed over time,
and an in-memory job list loses everything on restart, which is not acceptable
for the primary self-hosted product.

## Decision
1. Server mode MUST allow the operator to register repositories. A registration
   MUST survive process restart.
2. A registered repository MUST be re-analysable on demand and on a recurring
   trigger defined by the operator. The recurrence definition is operator
   configuration, not a project schedule, and ADR-0004 does not apply to it.
3. Analyses of a registered repository MUST be retained as distinct versions so
   that two analyses of the same repository at different commits can be
   compared.
4. Comparison across versions MUST be expressed as a lens over the repository
   view, not as a separate report type (see ADR-0028).
5. Registry, recurrence and version history MUST be unavailable in public mode
   (see ADR-0029).

## Consequences
- The product requires persistent storage (ADR-0017) and therefore an embedded
  storage engine (ADR-0022).
- Recurring re-analysis implies remote repositories, which is governed by
  ADR-0016.
- Time-series comparison becomes a metric surface that no single snapshot can
  produce, which is why the report remains a snapshot and history lives behind
  an API (ADR-0021).

## Out of scope
- Multi-user access control over registered repositories is not built.
- Notifications, alerting and webhooks on analysis completion are not built.

## Assumption
A deployment serves one operator and tens of repositories, not hundreds of
concurrent users.

## Acceptance criteria
- A registered repository, its recurrence setting and its analysis versions are
  all present after a process restart.
- Two versions of the same repository can be compared in the interface.
- In public mode no route exposes registration, recurrence or version history.

## Dependencies
ADR-0017 and ADR-0022 must be implemented before this one.
