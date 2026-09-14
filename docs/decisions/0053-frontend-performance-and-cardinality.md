# ADR-0053: Bundle budget and report-side cardinality limits

**Status:** Accepted

## Context
The bundle is embedded in the binary and the application holds a report the
client loads. The real hazard is not bundle weight but attempting to render a
coupling graph with tens of thousands of nodes, and this cannot be fixed by
tuning the renderer.

## Decision
1. A frontend bundle size budget MUST be enforced as a gate.
2. Rendering budgets MUST be enforced against a designated large fixture report:
   first meaningful render and lens transition latency.
3. **Every visualisation MUST have a maximum element count applied in the
   report, not in the frontend.** No visualisation may attempt to render a data
   set of unbounded cardinality.
4. When a cardinality limit truncates a data set, the affected family MUST be
   marked `degraded` per ADR-0032 and the interface MUST show that the view is
   truncated. This generalises the existing top-N behaviour of the file activity
   metric.
5. The Wrapped presentation MUST be code-split from the dashboard and loaded on
   demand, because the two carry different visual languages and dependency
   profiles and most visitors open only one.
6. Cardinality limits belong to the analysis configuration plane (ADR-0026),
   since they change report contents, and MUST therefore enter the cache key.

## Consequences
- Report size and render cost are controlled from one place.
- A large repository produces a smaller report rather than an unusable page.
- Truncation is visible to the reader, consistent with ADR-0032.

## Out of scope
- Real-user monitoring, which requires telemetry (ADR-0013).
- Canvas or WebGL rendering as a way to raise cardinality limits.

## Assumption
Cardinality limits can be set high enough to preserve analytical value.

## Acceptance criteria
- A repository exceeding a cardinality limit produces a truncated, `degraded`
  section rather than an oversized report.
- Bundle and rendering budgets fail the build when exceeded.
- Wrapped assets are not loaded when only the dashboard is opened.

## Dependencies
ADR-0026, ADR-0032, ADR-0036 and ADR-0037 must be implemented before this one.
