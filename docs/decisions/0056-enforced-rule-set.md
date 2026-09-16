# ADR-0056: The enforced rule set

**Status:** Superseded
**Superseded by:** ADR-0063

## Context
ADR-0055 establishes how rules are enforced. This record establishes which.

## Decision
1. The following MUST be enforced by linter or formatter configuration:

   | Rule | Record |
   |---|---|
   | Formatting and vet clean | — |
   | `internal/metrics/*` may not import one another | ADR-0040 |
   | Package dependency direction `core` ← `metrics` ← `pipeline` ← `server`/`storage` | ADR-0040 |
   | Process execution imported only by the git package | ADR-0047 |
   | No direct process clock call; injected clock required | ADR-0042 |
   | No bare `go` statement | ADR-0044 |
   | No services carried in `context.Context` | ADR-0042 |
   | No globals or package-level mutable singletons | ADR-0042 |
   | No raw colour, spacing, typography or radius values in the frontend | ADR-0038 |
   | No imperative DOM manipulation in visualisation code | ADR-0037 |
   | Direct dependencies match the allow list | ADR-0049 |
   | Frontend bundle size budget | ADR-0053 |

2. The following MUST be enforced by repository checkers:

   | Checker | What it verifies | Record |
   |---|---|---|
   | Record integrity | Template sections present; index complete; no accepted record contains a date; cross-references resolve | ADR-0001, ADR-0004 |
   | Family contract | Each family implements the interface, declares inputs, owns a namespace, carries a version, has golden coverage | ADR-0024 |
   | Namespace violation | No family writes outside its namespace | ADR-0024 |
   | Determinism | Same input twice, and differing parallelism degrees, produce identical reports | ADR-0021, ADR-0052 |
   | CLI/server equivalence | Both paths produce identical reports for identical input | ADR-0021 |
   | Incremental equivalence | Incremental result equals full result | ADR-0017 |
   | Identity projection | Merge projection equals pre-merged recomputation | ADR-0018 |
   | Leak scan | Report, exported image, API responses **and logs** contain no raw email, absolute path or hostname | ADR-0033, ADR-0041 |
   | Mode capability matrix | Every capability marked unavailable in `public` is unreachable on every route | ADR-0029 |
   | Archetype reachability | Every archetype reachable by a fixture; every archetype has a default description | ADR-0030, ADR-0039 |
   | Model-free equivalence | With no model configured, the report is identical outside the generated-text namespace | ADR-0039 |
   | Subprocess count | Git process count does not grow proportionally to commit count | ADR-0019 |
   | Goroutine leak | No residual goroutines after the suite | ADR-0044 |
   | Bundle integrity | Frontend rebuilt from source matches the committed bundle | ADR-0049 |
   | Cross-compilation | All target platforms build from one machine with no C toolchain | ADR-0022, ADR-0035 |
   | Fixture determinism | Two generations produce identical commit hashes | ADR-0019 |
   | Decision reference | Constraint-bearing files carry resolvable record references | ADR-0059 |

3. Adding a rule to either table MUST include its enforcement in the same
   change (ADR-0055 clause 6).

## Consequences
- The leak scan covers logs, which is where the privacy boundary is most likely
  to be broken quietly.
- The capability matrix is verified rather than reasoned about, which matters
  because a single open route in public mode defeats the whole privacy position.

## Out of scope
- Coverage thresholds, mutation testing, style rules not listed here.

## Assumption
Every rule above is mechanically checkable. A rule that turns out not to be
belongs in the conventions document (ADR-0058) instead.

## Acceptance criteria
- Every row in both tables exists as configuration or as a checker.
- Each checker's failure message names its record.

## Dependencies
ADR-0055 must be implemented before this one.
