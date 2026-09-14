# ADR-0038: Token-enforced styling with headless primitives

**Status:** Accepted

## Context
The existing local dashboard shell uses a component library carrying a strong
visual identity of its own. Visual distinctiveness is not decoration in this
product: every comparable tool presents a grid of line charts, and that is the
gap this project targets. Fighting a design system's identity through theming
produces a themed version of that system, not a distinct one.

## Decision
1. The component library carrying an opinionated visual identity MUST be
   removed. The existing shell is rewritten. This cost is accepted.
2. Styling MUST use utility classes co-located with markup, driven by a project
   design token set.
3. Colour, spacing, typography and radius values MUST come from tokens. Raw
   values in components MUST be rejected by a lint rule.
4. Accessible interactive primitives — menu, dialog, tabs, tooltip, popover —
   MUST come from a headless library. Keyboard navigation and focus management
   MUST NOT be hand-written.
5. The dashboard and the Wrapped presentation MUST share one token set and apply
   it differently. They are two applications of one system, not two systems.
6. The frontend bundle MUST stay within the budget defined in ADR-0053.

## Consequences
- The product has its own visual identity rather than a recognisable framework's.
- Style drift between agent sessions is mechanically detectable, because
  off-token values fail a lint rule.
- Existing shell components are rewritten.

## Out of scope
- Runtime CSS-in-JS.
- A published component library for external consumers.

## Assumption
Visual distinctiveness is worth more to this project than the speed a
ready-made component library provides.

## Acceptance criteria
- The opinionated component library is absent from dependencies.
- A lint rule rejects raw colour, spacing, typography and radius values.
- Both surfaces resolve their styling from the same token source.

## Dependencies
ADR-0036 must be implemented before this one.
