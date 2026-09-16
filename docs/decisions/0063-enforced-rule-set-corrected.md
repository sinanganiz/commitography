# ADR-0063: The enforced rule set

**Status:** Accepted
**Supersedes:** ADR-0056

## Context
ADR-0056 assigned each rule to either linter configuration or a repository
checker. Implementation found three assignments wrong. Two rules cannot be
expressed in configuration: no bundled linter matches a statement form, and the
rule that would require a rule-description dependency is forbidden by ADR-0049
clause 2; and the dependency allow list compares manifests rather than imports,
which is not something a linter inspects. A third rule was missing entirely:
ADR-0049 requires reproducible builds, but nothing verified it, and the build
currently injects a clock reading, so two builds of one commit already differ.

This record restates the complete rule set with those corrections, rather than
amending ADR-0056 in place, so that one table remains authoritative.

## Decision
1. Enforced by linter or formatter configuration:

   | Rule | Record |
   |---|---|
   | Formatting and vet clean, over **tracked files only** | — |
   | `internal/metrics/*` may not import one another | ADR-0040 |
   | Package dependency direction `core` ← `metrics` ← `pipeline` ← `server`/`storage` | ADR-0040, ADR-0060 |
   | Nothing may import `internal/checks/` | ADR-0060 |
   | Process execution imported only by the git package | ADR-0047 |
   | No direct process clock call; injected clock required | ADR-0042 |
   | No services carried in `context.Context` | ADR-0042 |
   | No globals or package-level mutable singletons, except the one link-time metadata file | ADR-0042, ADR-0061 |
   | No raw colour, spacing, typography or radius values in the frontend | ADR-0038 |
   | No imperative DOM manipulation in visualisation code | ADR-0037 |
   | Frontend bundle size budget | ADR-0053 |

2. Enforced by repository checkers:

   | Checker | What it verifies | Record |
   |---|---|---|
   | Record integrity | Template sections present; index complete; no accepted record contains a date; cross-references resolve | ADR-0001, ADR-0004 |
   | **No bare `go` statement** | Every goroutine is started inside a wait or error group | ADR-0044 |
   | **Direct dependencies match the allow list** | Manifest comparison, Go and frontend | ADR-0049 |
   | **Reproducible build** | Two builds of one commit produce identical binaries | ADR-0049 |
   | Family contract | Each family implements the interface, declares inputs, owns a namespace, carries a version, has golden coverage | ADR-0024 |
   | Namespace violation | No family writes outside its namespace | ADR-0024 |
   | Metric catalogue | No metric, reason code or cardinality limit exists that `docs/metrics.md` does not define | ADR-0062 |
   | Determinism | Same input twice, and differing parallelism degrees, produce identical reports | ADR-0021, ADR-0052 |
   | CLI/server equivalence | Both paths produce identical reports for identical input | ADR-0021 |
   | Incremental equivalence | Incremental result equals full result | ADR-0017 |
   | Identity projection | Merge projection equals pre-merged recomputation | ADR-0018 |
   | Leak scan | Report, exported image, API responses and logs contain no raw email, absolute path or hostname | ADR-0033, ADR-0041 |
   | Mode capability matrix | Every capability marked unavailable in `public` is unreachable on every route | ADR-0029 |
   | Archetype reachability | Every archetype reachable by a fixture; every archetype has a default description | ADR-0030, ADR-0039 |
   | Model-free equivalence | With no model configured, the report is identical outside the generated-text namespace | ADR-0039 |
   | Subprocess count | Git process count does not grow proportionally to commit count | ADR-0019 |
   | Goroutine leak | No residual goroutines after the suite | ADR-0044 |
   | Bundle integrity | Frontend rebuilt from source matches the committed bundle | ADR-0049 |
   | Cross-compilation | All target platforms build from one machine with no C toolchain | ADR-0022, ADR-0035 |
   | Fixture determinism | Two generations produce identical commit hashes on every supported platform | ADR-0019 |
   | Decision reference | Constraint-bearing files carry resolvable record references | ADR-0059 |

3. **Build metadata MUST NOT be derived from the clock or from any value that
   varies between builds of one commit.** Where a build timestamp is wanted, it
   is the analysed commit's own timestamp. This is what makes the reproducible
   build checker satisfiable.
4. Every check runs over tracked files only, never over the working tree.
   Generated fixtures are untracked and are deliberately not valid source.
5. Adding a rule to either table MUST include its enforcement in the same
   change (ADR-0055 clause 6).
6. A rule whose subject does not yet exist is recorded in a tracking list in the
   linter configuration, naming the package that will introduce it, and is
   enforced by that package.

## Consequences
- The two rules that cannot be configured are checkers, which is what ADR-0055
  clauses 1 and 2 already required; only their placement was wrong.
- Reproducibility becomes verifiable rather than asserted, and the clock-derived
  build value that currently breaks it is prohibited by name.
- The leak scan, the capability matrix and the metric catalogue checks remain
  the three whose absence would be most costly, and all three are present.

## Out of scope
- Coverage thresholds, mutation testing, style rules not listed here.
- Enforcing a frontend rule before the design token system it depends on exists;
  clause 6 covers that case.

## Assumption
Every rule above is mechanically checkable. A rule that turns out not to be
belongs in the conventions document (ADR-0058) instead, and moving it there
requires a superseding record.

## Acceptance criteria
- Every row in both tables exists as configuration or as a checker, or appears
  in the clause 6 tracking list with a package number.
- Each checker's failure message names its record.
- Two builds of one commit produce identical binaries.

## Dependencies
ADR-0055 must be implemented before this one.
