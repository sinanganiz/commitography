# ADR-0057: Two gates and a release path, with duration budgets

**Status:** Accepted

## Context
Because implementation runs continuously, iteration latency matters more than
total pipeline time: if feedback is slow, parallel sessions produce work that
invalidates itself. Release duration blocks nobody.

## Decision
1. There MUST be two gates and a release path:

   | Gate | Runs | Budget |
   |---|---|---|
   | **Fast** | On every change | 5 minutes |
   | **Full** | Before merge | 15 minutes |
   | **Release** | On release | No budget |

2. Gate contents MUST be:
   - **Fast**: build, format, vet, all configuration rules, unit tests,
     invariants, golden comparison on small fixtures, record integrity, family
     contract, namespace violation, goroutine leak, leak scan.
   - **Full**: everything in fast, plus golden comparison on large fixtures,
     determinism, incremental equivalence, identity projection, mode capability
     matrix, model-free equivalence, performance budgets, subprocess count,
     cross-compilation, bundle integrity, vulnerability scanning.
   - **Release**: everything in full, plus platform artifacts, software bill of
     materials and signing.
3. **Exceeding a gate budget MUST NOT be resolved by removing a check.**
   Acceptable responses are: reduce the fixture, move the check from fast to
   full, or make the check faster.
4. A nightly or scheduled full run MUST NOT be introduced as a substitute for a
   gate. A check whose result nobody is waiting for is not enforced.
5. Every checker in ADR-0056 MUST be assigned to exactly one gate.

## Consequences
- Feedback on a change arrives quickly enough to keep parallel work coherent.
- Expensive verification still blocks merges rather than running unobserved.

## Out of scope
- Scheduled background pipelines.
- Per-change performance profiling.

## Assumption
The fast gate's contents fit within its budget on the available runners. If not,
checks move to full rather than being deleted.

## Acceptance criteria
- Both gates exist with the stated contents and budgets.
- Every checker in ADR-0056 appears in exactly one gate.
- No scheduled pipeline is the sole location of any check.

## Dependencies
ADR-0056 must be implemented before this one.
