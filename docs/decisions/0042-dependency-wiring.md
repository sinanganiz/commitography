# ADR-0042: Explicit constructor injection and no ambient state

**Status:** Accepted

## Context
ADR-0021 requires byte-identical output for identical input, and ADR-0019
requires parallel tests that share no state. Both are defeated by globals,
package-level singletons, and direct access to the clock, randomness or the
filesystem.

## Decision
1. Dependencies MUST be passed through constructors and wired explicitly in one
   composition location. No dependency injection framework and no reflection
   based wiring.
2. Global variables and package-level mutable singletons MUST NOT exist and
   MUST be rejected by a lint rule.
3. `context.Context` MUST carry cancellation and deadlines only. Services and
   business data MUST NOT be placed in it; tracing identifiers are the sole
   exception.
4. Clock, randomness and filesystem access MUST be injected. Direct calls to the
   process clock MUST be rejected by a lint rule.
5. Any value that would make output vary between runs MUST enter through an
   injected dependency so that it can be fixed in tests.

## Consequences
- Every dependency a component holds is visible at its construction site, so a
  newly introduced one appears in the diff.
- Determinism is structurally achievable rather than repeatedly repaired.
- Tests run in parallel without shared state.

## Out of scope
- Container-based or reflective dependency injection.
- Service locators, including a context-carried service bag.

## Assumption
Manual wiring stays readable. If the composition location grows unwieldy, it is
split by area, not replaced by a framework.

## Acceptance criteria
- Lint rules reject globals, package-level mutable singletons and direct clock
  calls.
- Tests fixing clock and randomness produce identical reports across runs.

## Dependencies
ADR-0021 must be implemented before this one.
