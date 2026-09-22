# ADR-0024: Metric families declare their inputs and never read each other

**Status:** Superseded
**Superseded by:** ADR-0076

## Context
Ten metric families are in scope (ADR-0012), including one that will be added
long after the others. Without a boundary, aggregation becomes a single body of
code in which families reach into each other's results, and adding a family
means rewriting the stage. It would also leave no template for an agent
implementing a new metric.

## Decision
1. Every metric family MUST be implemented behind a common interface and MUST
   declare:
   - the inputs it requires, drawn from exactly this set: `commit-records`,
     `replay-state`, `worktree`, `external-service`;
   - the output namespace it owns in the report;
   - its family version (ADR-0031).
2. The pipeline MUST resolve declared inputs and MUST skip any family whose
   inputs are unavailable, recording the skip under ADR-0032.
3. A family MUST NOT read another family's output. If two families need the
   same derived data, that data MUST be produced by an earlier stage or by a
   shared derived-data layer, never through a family-to-family dependency.
4. A family MUST NOT write outside its declared namespace.
5. The catalogue MUST contain exactly these families, with these inputs:

   | Family | Inputs |
   |---|---|
   | `temporal` | commit-records |
   | `commit-size` | commit-records |
   | `messages` | commit-records |
   | `files` | commit-records |
   | `coupling` | commit-records |
   | `ownership` | replay-state |
   | `worktype` | replay-state |
   | `ai-archaeology` | commit-records, replay-state |
   | `hotspot` | worktree, commit-records |
   | `static-analysis` | worktree |

6. Adding a family MUST require: an interface implementation, a declared input
   set, a namespace, a family version, and golden fixture coverage
   (ADR-0019). Nothing else in the pipeline may need to change.
7. The optional language-model prose capability MUST be modelled as a consumer
   declaring `external-service` and MUST default to unavailable.

## Consequences
- Capability differences between modes (ADR-0029) and missing inputs are
  handled by one mechanism rather than by conditionals scattered through
  aggregation.
- Adding a metric is a mechanical, templated task, which is what makes it safe
  to delegate.
- Static analysis can be added later without restructuring anything.

## Out of scope
- External plugin loading through subprocesses, dynamic libraries or WASM, and
  any stable plugin ABI.

## Assumption
The four input kinds cover every family's needs. A fifth can be added cheaply
if required.

## Acceptance criteria
- Every family in clause 5 implements the interface and declares its inputs.
- A static check fails if a family reads a namespace it does not own.
- Removing `worktree` availability causes `hotspot` and `static-analysis` to be
  reported as skipped, and no other family to change.

## Dependencies
ADR-0020 and ADR-0031 must be implemented before this one. ADR-0032 depends
on this one.
