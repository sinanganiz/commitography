# ADR-0048: Enforced limits, and truncation is never silent

**Status:** Accepted

## Context
A repository may be hostile or merely enormous, and the replay stage consumes
memory proportional to tracked lines, which is a direct exhaustion vector. In
public mode an anonymous visitor chooses the repository.

## Decision
1. The following limits MUST be enforced: clone size, commit count, tracked file
   count, single file size, wall-clock duration, and peak memory.
2. Public mode MUST additionally enforce a per-visitor request rate limit and a
   global concurrency ceiling. These MUST be bound to the mode switch
   (ADR-0029), not to independent flags, and their defaults MUST be
   substantially stricter than `server` mode defaults.
3. **A limit reached MUST NOT produce a silent truncation.** The result MUST be
   marked `degraded` per ADR-0032, naming which limit was reached, and the
   interface MUST display it persistently. Producing a confident report from a
   partially analysed history is the same failure the shallow-clone refusal
   exists to prevent.
4. Clone size MUST be enforced by monitoring the target directory and
   terminating the subprocess when the threshold is exceeded, since git does not
   provide the limit itself.
5. Exceeding the memory ceiling MUST abort the analysis with a reported reason.
   The process MUST NOT be allowed to exhaust the host.
6. Limit values belong to the operational configuration plane (ADR-0026) and
   MUST NOT affect metric values. The resulting `degraded` status is a metric
   outcome and IS recorded in the report.

## Consequences
- A hostile or oversized repository degrades one analysis rather than the host.
- A reader is never shown a complete-looking report derived from partial data.

## Out of scope
- Per-user quota accounting; there are no user accounts to attribute quota to.

## Assumption
Limits can be set above the overwhelming majority of real repositories. A limit
that triggers routinely makes the product unusable.

## Acceptance criteria
- A fixture exceeding each limit produces a `degraded` report naming that limit.
- Public mode defaults are strictly tighter than `server` mode defaults and are
  not separately overridable.
- Exceeding the memory ceiling aborts with a reported reason rather than
  exhausting the host.

## Dependencies
ADR-0026, ADR-0029 and ADR-0032 must be implemented before this one.
