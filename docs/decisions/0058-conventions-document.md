# ADR-0058: Non-enforceable patterns live in a guidance document

**Status:** Accepted

## Context
Some patterns produce consistency but cannot be mechanically checked: naming,
test organisation, comment style, how to structure a new visualisation. They
should not be lost, and they should not swell the decision set, which stays
useful only while it stays readable.

## Decision
1. Patterns that cannot be enforced mechanically MUST be recorded in
   `docs/conventions.md` with their reasoning.
2. That document MUST state its status in its first lines: it is **guidance**,
   not binding, and a justified deviation is acceptable.
3. `AGENTS.md` MUST state the distinction explicitly: `docs/decisions/` is
   binding and a violation is a defect; `docs/conventions.md` is guidance.
4. When a convention becomes mechanically enforceable, it MUST move to a checker
   under ADR-0055 and be removed from the conventions document. A rule MUST NOT
   exist in both places.
5. If a convention begins to bind an interface, an artifact format or a product
   guarantee, it is no longer a convention and MUST be written as a record.

## Consequences
- Guidance is available without inflating the binding set.
- The distinction between binding and advisory is stated where it is read, which
  matters most in the direction that would otherwise soften a record.
- Conventions migrate toward enforcement over time rather than accumulating.

## Out of scope
- Style rules that duplicate the formatter.
- Conventions about product behaviour, which are records.

## Assumption
The distinction is respected in practice because it is stated in the file an
implementer reads first.

## Acceptance criteria
- `docs/conventions.md` exists and declares its status in its opening lines.
- `AGENTS.md` states the binding versus guidance distinction.
- No rule appears in both the conventions document and the enforced set.

## Dependencies
ADR-0001 and ADR-0055 must be implemented before this one.
