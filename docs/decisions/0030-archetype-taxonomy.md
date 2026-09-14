# ADR-0030: Archetypes are assigned by an ordered rule list over absolute thresholds

**Status:** Accepted

## Context
ADR-0013 removed any global corpus, so archetype assignment rests on absolute
thresholds. A weighted scoring model would make assignment unexplainable and
untunable by hand. The project already uses an ordered first-match rule list
for commit message classification, so reusing that mechanism avoids introducing
a second concept.

## Decision
1. Archetypes MUST be defined in a human-readable, version-controlled data file
   rather than in code.
2. Each archetype MUST be defined as a conjunction of threshold conditions over
   one or more metric axes.
3. Assignment MUST evaluate the list in order and take the first match. The
   list MUST end with an unconditional fallback archetype, so assignment always
   succeeds.
4. The reason for an assignment MUST be reportable as the specific conditions
   that matched, and the interface MUST be able to display it. An archetype
   MUST NOT be presented without an available explanation.
5. Archetype identifiers MUST be stable, language-independent slugs. Display
   names and descriptions MUST be a separate translatable layer. The default
   display language MUST be English.
6. The taxonomy MUST carry a version. The report MUST record the taxonomy
   version used. Changing a threshold or an identifier's meaning MUST increment
   it.
7. Badges MUST use the same definition file and the same first-match evaluation,
   evaluated independently of the archetype list, and are bounded by ADR-0014
   clause 2.
8. A test MUST assert that every archetype in the file is reachable by at least
   one fixture, so that ordering cannot starve entries (ADR-0019 clause 3).
9. The interface MUST NOT present an archetype as a population ranking
   (ADR-0013 clause 3).

## Consequences
- Assignment is explainable in one sentence and tunable without code changes.
- Threshold values are visible and auditable rather than embedded in logic.
- Ordering becomes a design responsibility, mitigated by the reachability test.

## Out of scope
- Weighted scoring, machine-learned assignment, model-generated archetypes.
- Orthogonal-axis personality frameworks.

## Assumption
Hand-calibrated thresholds produce a distribution in which archetypes are not
concentrated in a single entry. This must be measured.

## Acceptance criteria
- Archetype definitions live in a data file, not in code.
- Every subject receives exactly one archetype, including edge cases with
  minimal activity.
- Every archetype is reachable by at least one fixture.
- The report records the taxonomy version.

## Dependencies
ADR-0013 and ADR-0020 must be implemented before this one.
