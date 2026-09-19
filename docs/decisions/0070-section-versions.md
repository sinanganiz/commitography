# ADR-0070: Top-level report sections carry their own version

**Status:** Accepted
**Note:** This record extends ADR-0031. It supersedes it in no part.

## Context
ADR-0031 gives the document a version governing structure, and every metric
family a version governing the meaning of its values. The report also carries
top-level sections that are not families: generation metadata, the resolved
analysis configuration, and the identities list. Nothing versions the meaning of
their values.

The gap became concrete when the derivation of merge candidates changed. The
field's structure is unchanged, so the document version does not move; the
section is not a family, so no family version moves; and a stored report would
carry candidates computed one way beside a new report carrying candidates
computed another, under the same name and the same version.

## Decision
1. Every top-level section of the report that is not a metric family MUST carry
   its own version, with major and minor components, governed by the same rules
   ADR-0031 clause 2 applies to a family:
   - any change to how the section's values are computed increments it;
   - adding a field without changing existing values increments the minor
     component only.
2. The document version continues to govern structure alone: top-level shape,
   identity representation, status fields, and the metadata section's place.
3. The report cache key MUST include every section version alongside the
   document version and the family versions (ADR-0017 clause 2). Without it,
   newer code reads an older cached section as though it meant the same thing.
4. Changing a section's computation without incrementing its version is a
   defect, and the golden regime (ADR-0019) makes it visible as a diff.

## Consequences
- The three non-family sections evolve on the same terms as families, so there
  is one rule rather than two.
- A cached report cannot be silently reinterpreted after a derivation changes.
- Adding a top-level section in future carries a version from the start rather
  than acquiring one after the first change to it.

## Out of scope
- Automatic migration of stored reports between section major versions.
- Versioning anything below a top-level section.

## Assumption
The set of top-level sections stays small. A report whose sections outnumber its
families indicates a structure problem, not a versioning one.

## Acceptance criteria
- Every top-level non-family section carries a version.
- Changing a section's computation without incrementing its version fails a
  test.
- Changing any section version produces a different cache key.

## Dependencies
ADR-0017 and ADR-0031 must be implemented before this one.
