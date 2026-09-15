# ADR-0061: Link-time build metadata is the one exception to the no-package-variable rule

**Status:** Accepted
**Note:** This record narrows ADR-0042 clause 2. It does not supersede it.

## Context
Three accepted records meet at one point and cannot all be satisfied as written.
ADR-0049 clause 6 requires version information to be injected explicitly at build
time. The toolchain injects such values only into package-level string
variables. ADR-0042 clause 2 forbids package-level mutable state.

## Decision
1. ADR-0042 clause 2's prohibition is on state that is **written at runtime**.
   Values written once at link time and never written afterwards are outside it.
2. Exactly one file MAY declare package-level string variables for link-time
   injection. It MUST live in `internal/core/`, contain nothing but those
   variables and a function that returns them, and be the only file in the
   repository carrying a suppression for the package-variable lint rule.
3. That suppression MUST name this record, as ADR-0055 clause 4 requires.
4. Nothing in the repository MAY write to those variables at runtime. They are
   read once at composition and passed onward by injection like any other
   dependency (ADR-0042 clause 1).
5. No other value MAY use this mechanism. Configuration, feature switches, mode
   and limits are injected, not linked.
6. Because these values vary between builds of the same source, they MUST be
   confined to the report's generation metadata section (ADR-0021 clause 6) and
   MUST NOT enter any metric, any cache key, or any golden comparison.

## Consequences
- Version information is injectable without a general exception to the rule.
- The exception is one file, one lint suppression, and a record number in that
  suppression, so it is visible rather than precedent-setting.
- Clause 6 keeps determinism intact: build metadata cannot make two runs of the
  same commit produce different comparable output.

## Out of scope
- Link-time injection of anything other than build metadata.
- Reading build metadata from the environment at runtime as an alternative.

## Assumption
The toolchain continues to offer no mechanism for injecting build values into
anything other than package-level string variables.

## Acceptance criteria
- Exactly one file in the repository suppresses the package-variable lint rule,
  and its suppression names ADR-0061.
- No code path assigns to those variables outside link time.
- Build metadata appears only in the report's designated metadata section, and
  removing it from a comparison makes two builds of one commit identical.

## Dependencies
ADR-0042 and ADR-0049 must be implemented before this one.
