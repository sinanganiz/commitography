# ADR-0037: Layout mathematics from libraries, DOM ownership by React alone

**Status:** Accepted

## Context
The signature visualisations do not map onto standard chart types, so a general
charting library cannot express them. Writing everything by hand, however, means
reimplementing solved mathematics — axis tick selection, logarithmic and time
scales, curve interpolation, force-directed and hierarchical layout — usually
worse. The real hazard is two systems writing to the same DOM.

## Decision
1. General-purpose charting libraries that render their own output MUST NOT be
   used.
2. Libraries MAY be used for computation only: scales, tick selection, curve
   generation, force simulation, hierarchical layout, geometry.
3. **No visualisation library may touch the DOM.** Libraries produce numbers;
   React renders SVG. Imperative DOM manipulation inside visualisation code is
   forbidden and MUST be rejected by a lint rule.
4. If a required layout algorithm is unavailable, that algorithm MAY be written
   by hand. This does not relax clause 3.
5. Visualisations MUST respect the cardinality limits defined in ADR-0053.

## Consequences
- Lens transitions and animation remain predictable because one system owns
  rendering.
- Layout mathematics is correct without being reimplemented.
- Adding a visualisation is a React component plus a layout computation, never a
  library instance bound to a node.

## Out of scope
- Canvas or WebGL rendering.
- Charting libraries that own their container element.

## Assumption
The needed layout algorithms exist as computation-only modules.

## Acceptance criteria
- No dependency that renders charts autonomously is present.
- A lint rule rejects direct DOM manipulation in visualisation code.

## Dependencies
ADR-0036 must be implemented before this one.
