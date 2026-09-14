# ADR-0052: Collect is parallel, replay is sequential, aggregate is parallel by family

**Status:** Accepted

## Context
Reading history is diff-bound and divisible. Replay accumulates line ownership
chronologically, so each commit's classification depends on the result of every
commit before it. Metric families are independent by ADR-0024 clause 3.

## Decision
1. `collect` MAY split history across concurrent readers and reassemble in
   order.
2. **`replay` MUST be sequential. Parallelising the chronological walk is
   forbidden**, because line ownership state is order-dependent and a parallel
   walk produces incorrect classification rather than a slower correct one.
3. `aggregate` MAY run metric families concurrently, which is safe precisely
   because families cannot read each other.
4. Streaming parallelism across pipeline stages MUST NOT be implemented, because
   it would defeat the independent cacheability of collect output required by
   ADR-0020 clause 2.
5. The degree of parallelism MUST be configurable and MUST default to a value
   derived from available cores, not to a fixed number.
6. **The degree of parallelism MUST NOT change the report.** A test MUST verify
   that different degrees produce identical output (ADR-0021).

## Consequences
- The parallelism that helps is used; the parallelism that would silently
  corrupt results is prohibited rather than discouraged.
- Adding a family adds parallel work without coordination.

## Out of scope
- Parallel replay under any scheme.
- Cross-stage streaming.

## Assumption
Collect remains the dominant cost in full analysis. If replay becomes dominant,
the answer is a cheaper representation, not parallelisation.

## Acceptance criteria
- Reports produced at parallelism degrees 1, 2 and N are byte-identical.
- No code path executes the chronological walk concurrently.

## Dependencies
ADR-0020, ADR-0021 and ADR-0024 must be implemented before this one.
