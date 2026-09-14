# ADR-0014: Interpretation is one archetype plus a bounded set of badges

**Status:** Accepted

## Context
A shareable identity needs to be singular and comparable — the reason
personality types spread is that the set is fixed and small. A single label,
however, cannot carry the variety of incidental findings that make the output
enjoyable to read.

## Decision
1. Exactly one **archetype** MUST be assigned to a subject. Assignment MUST
   always succeed; a guaranteed fallback archetype MUST exist (ADR-0030).
2. Zero or more **badges** MAY additionally be awarded. At most 3 badges MUST
   be displayed for a subject in any single presentation.
3. Archetypes and badges MUST be assigned deterministically. A language model
   MUST NOT choose, invent, rename or override an archetype or a badge.
4. A language model MAY generate prose that describes an already-assigned
   archetype. That capability MUST default to disabled, MUST require the
   operator's own model endpoint or credentials, and MUST NOT be required for
   any archetype or badge to be assigned or displayed.
5. Generated prose about a named person MUST NOT be displayed in public mode.
6. Archetypes and badges MUST be assignable to both subjects defined in
   ADR-0023.

## Consequences
- The same input always produces the same archetype, so results are
  comparable between people and verifiable by test (ADR-0019).
- Interpretation costs nothing to run and works offline.
- Disabling the language model removes flavour text only, never structure.

## Out of scope
- Model-generated nicknames, insults or characterisations of named individuals.
- Personality frameworks built from orthogonal axis combinations (considered
  and rejected: the metrics do not decompose into clean independent axes).

## Assumption
A fixed, bounded archetype set is more shareable than per-user generated text,
because it can be compared and collected.

## Acceptance criteria
- Every subject receives exactly one archetype, including subjects with minimal
  activity.
- Archetype and badge assignment is byte-identical across runs on the same
  report.
- With the language model disabled, every archetype and badge still renders.

## Dependencies
ADR-0030 must be implemented before this one.
