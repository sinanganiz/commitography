# ADR-0020: The pipeline has five stages

**Status:** Accepted

## Context
The previous three-stage pipeline assumed metrics were computed by aggregating
independent commit records. Work-type classification and full line-age analysis
require a chronological walk that accumulates state, which is neither pure
collection nor pure aggregation. Interpretation — archetype and badge
assignment — reads across metric families and therefore cannot live inside any
one of them without breaking ADR-0024.

## Decision
1. The pipeline MUST consist of exactly five stages, in order:
   `collect` → `replay` → `aggregate` → `interpret` → `render`.
2. `collect` reads git history in a single pass and produces normalized commit
   records. It MUST NOT compute metrics. Its output MUST be cacheable
   independently.
3. `replay` walks commits in chronological order and maintains, in memory, a
   per-file line ownership map holding for each line its owning identity and
   its authoring timestamp. It is the only stage that owns the checkpoint
   (ADR-0017) and the only stage with working tree access.
4. Work-type classification MUST be performed during replay by consulting the
   ownership map, not by invoking `git blame` per commit. The classification
   rules are:
   - line not present in the map → **new work**
   - previous owner is the same identity and the line is newer than the
     configured recency window → **rework**
   - previous owner is a different identity and the line is newer than the
     configured recency window → **help others**
   - the line is older than the configured recency window → **legacy refactor**
   The recency window MUST be configurable and MUST default to 30 days.
5. `aggregate` computes metric families from commit records and replay state.
   It MUST be stateless and MUST be re-runnable without touching git.
6. `interpret` reads the aggregated report and assigns archetypes and badges
   deterministically (ADR-0014). Optional language-model prose is produced here
   and nowhere else. `interpret` MUST be re-runnable over a stored report
   without re-running any earlier stage.
7. `render` produces presentation output only and MUST NOT compute any metric.

## Consequences
- Because replay produces complete line ownership for the head commit, code age
  is computed over all tracked text files rather than a sample, and a flag to
  skip blame is no longer required and MUST NOT exist.
- Changing a metric definition requires re-running `aggregate` only. Changing a
  taxonomy requires re-running `interpret` only.
- Replay's memory use is proportional to tracked lines and MUST be measured
  against a fixture under ADR-0019 clause 4.
- Replay-derived ownership is not identical to `git blame` because it does not
  reproduce copy and move detection. The divergence MUST be measured
  (ADR-0019 clause 6) and the report MUST describe the method used
  (ADR-0032).

## Out of scope
- An event-sourced architecture in which reports are derived by query.
- Merging collect and replay into a single non-cacheable pass.

## Assumption
Replay state plus commit records satisfy every metric family's input needs. A
family that must return to git invalidates this assumption.

## Acceptance criteria
- The five stages exist as separately invocable units.
- No `git blame` invocation occurs in the work-type classification path.
- Re-running `aggregate` from cached collect output and a checkpoint produces
  the same report without reading the repository.
- No flag exists to skip blame.

## Dependencies
None.
