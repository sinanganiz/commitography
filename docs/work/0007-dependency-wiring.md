# WP-0007: Dependency wiring and ambient state removal

**Area:** foundation
**Implements:** ADR-0042, ADR-0061, ADR-0021
**Requires:** WP-0005

## Goal
No global or package-level mutable state exists apart from the single link-time
metadata file, nothing reads the process clock directly, `context.Context`
carries no services, every dependency is passed through a constructor and wired
in one place per entry point, and two runs with a fixed clock produce identical
reports.

## In scope
1. Establish one composition location per entry point: the command, and the
   server. Every dependency is constructed and passed there.
2. Convert components to constructor injection. No dependency injection
   framework, no reflection-based wiring, no service locator.
3. Remove every global and package-level mutable singleton. The one permitted
   exception is the link-time build metadata file (ADR-0061 clause 2), whose
   variables are **unexported** and reachable only through an accessor, so that
   nothing can write them at runtime.
4. Introduce injected clock, randomness and filesystem access in `internal/core`
   or a subpackage beneath it (ADR-0066 clause 2). Every consumer takes them as
   a dependency.
5. Remove every direct process clock call. Enable the lint rule.
6. Remove services and business data from `context.Context`. Cancellation,
   deadlines and tracing identifiers are all it may carry.
7. Enable the lint rules held in the tracking list for globals, singletons,
   context contents and clock calls. Each was recorded against this package.
8. Make tests fix clock and randomness so they run in parallel with no shared
   state.
9. Enable the determinism checker's same-input half: two runs of one fixture
   produce byte-identical reports outside the generation metadata section.

## Out of scope
- Any behavioural change. Values a component reads do not change; only how it
  receives them does.
- Abstracting the filesystem beyond what determinism and testability require. A
  general filesystem abstraction layer is not wanted.
- The parallelism half of the determinism checker, which arrives with
  parallelism (WP-0012).
- Changing what any test asserts.

## Files
**May create or modify:** `internal/**`, `cmd/**`, `internal/checks/**`, linter
configuration.
**Must not touch:** `docs/decisions/**`, `docs/metrics.md`, `testdata/**`,
golden files, `internal/pipeline/interpret/taxonomy/**`, `web/**`.

## Steps
1. Introduce the clock, randomness and filesystem interfaces and their real
   implementations.
2. Convert one package at a time to constructor injection, innermost first,
   running the fast gate after each.
3. Build the composition locations last, once nothing constructs its own
   dependencies.
4. Remove the globals, then enable each lint rule separately so an unexpected
   mass failure is attributable.
5. Make the build metadata variables unexported behind an accessor.
6. Enable the same-input determinism checker and observe it failing against a
   deliberately clock-dependent value, then revert.

## Definition of done
- The lint rules for globals, package-level mutable singletons, services in
  context, and direct clock calls are enabled and the build passes.
- Exactly one lint suppression exists for the package-variable rule, and it
  names ADR-0061.
- No code path can assign to a build metadata variable at runtime.
- Every dependency a component holds appears at its construction site.
- Two runs of one fixture produce byte-identical reports outside the generation
  metadata section.
- Tests run in parallel with no shared state.

## Verification
```
make gate-fast && make gate-full
git grep -n 'time.Now()' -- cmd internal      # no output
git grep -nE '^var [A-Za-z]' -- internal cmd  # only the metadata file
go test -race -count=1 ./...
go test ./internal/checks -run Determinism
```
