# ADR-0049: Dependency budget, reproducible builds and bundle verification

**Status:** Accepted

## Context
The realistic supply chain risk here is not a sophisticated attack but
unsupervised dependency growth. A further project-specific hazard exists: the
built frontend bundle is committed and embedded in the binary (ADR-0022
clause 4), so an artifact whose provenance is unverified is shipped directly.

## Decision
1. **Continuous integration MUST rebuild the frontend from source and compare
   the result with the committed bundle. A mismatch MUST fail the build.** The
   frontend build MUST therefore be reproducible: no embedded timestamps, no
   random identifiers.
2. Direct Go dependencies MUST match a recorded allow list. A direct dependency
   absent from the list MUST fail the build. The list MUST be kept narrow.
3. Direct frontend dependencies MUST follow the same rule, with an additional
   bundle size budget (ADR-0053).
4. Lock files MUST be committed and installation MUST use them exclusively.
5. Vulnerability scanning MUST be a gate, not a report.
6. Builds MUST be reproducible: path information trimmed, version information
   injected explicitly.
7. Release artifacts MUST have a software bill of materials and MUST be signed.
8. Third-party continuous integration actions MUST be pinned to immutable
   revisions, not to mutable tags.

## Consequences
- A committed bundle that does not correspond to its source cannot ship.
- Dependency growth requires a deliberate, visible change rather than happening
  incidentally.
- Release artifacts are verifiable by consumers.

## Out of scope
- Vendoring all dependencies into the repository.

## Assumption
The frontend build can be made deterministic. If it cannot, the comparison must
be replaced by a signed build attestation.

## Acceptance criteria
- Modifying the committed bundle without changing its source fails the build.
- Adding a direct dependency not on the allow list fails the build.
- Two builds of the same commit produce identical binaries.

## Dependencies
ADR-0022 and ADR-0036 must be implemented before this one.
