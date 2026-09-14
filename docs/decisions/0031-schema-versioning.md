# ADR-0031: Document-level and family-level versioning

**Status:** Accepted

## Context
The report is a contract (ADR-0021) while metric families evolve independently
(ADR-0024). A single version number would invalidate every stored report
whenever one family changed. No versioning at all would let a family's meaning
change silently, which under ADR-0017 means stored reports with the same field
names hold incomparable values.

## Decision
1. The report MUST carry a **document version** with major and minor
   components, governing the structural contract: top-level shape, identity
   representation, status fields, metadata section.
   - A minor increment MUST be additive only.
   - A major increment MAY break structure.
   - Within a major version, a field MUST NOT be removed and its meaning MUST
     NOT change.
2. Every metric family MUST carry its **own version**, governing the meaning of
   that family's output.
   - Any change to how a family's values are computed, including a threshold
     change or a default change, MUST increment that family's version.
   - Adding a field within a family's namespace without changing existing
     values MUST increment the family version's minor component only.
3. Consumers MUST ignore unknown fields.
4. The report cache key MUST include the document version and every present
   family version (ADR-0017 clause 2), so that a version change invalidates
   affected cached reports rather than allowing them to be read by newer code.
5. Changing a family's computation without incrementing its version MUST be
   treated as a defect. The golden regime (ADR-0019) makes such a change
   visible as a diff.
6. The taxonomy version (ADR-0030) is versioned separately from families and is
   recorded in the report.

## Consequences
- A family can evolve without invalidating unrelated stored data.
- Agents have a mechanical rule: change a family's meaning, increment its
  version, update its golden file.
- Stale cached reports cannot be silently reinterpreted by newer code.

## Out of scope
- Automatic migration of stored reports between major document versions.

## Assumption
External consumers check versions, or at worst assume older behaviour — which
is still better than silent divergence.

## Acceptance criteria
- The report contains a document version and a version for every present
  family.
- A change to a family's computation that is not accompanied by a version
  increment fails a test.
- Changing any version produces a different cache key.

## Dependencies
ADR-0021 must be implemented before this one.
