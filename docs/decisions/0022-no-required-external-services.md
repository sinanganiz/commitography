# ADR-0022: The product never requires an external service

**Status:** Accepted

## Context
Server mode requires persistent storage. Requiring the operator to install and
run a separate database before they can look at their own repository would
destroy the download-and-run property, which is one of the few characteristics
worth carrying forward from the project's earlier form.

## Decision
1. The product MUST be fully functional, in every mode, without any external
   service. No database server, cache server, message broker or object store
   may be a prerequisite.
2. Storage MUST be reached through an interface. The default and only required
   implementation MUST be embedded.
3. The embedded storage engine MUST NOT break cross-compilation. A dependency
   requiring a C toolchain to build for every target platform MUST NOT be used
   for default storage.
4. The binary MUST continue to embed the frontend so that a build produces a
   single self-contained executable.
5. An alternative storage implementation MAY be added later. It MUST NOT become
   required for any capability defined in the ADR set.
6. The only runtime dependency outside the binary MUST be the `git` executable.

## Consequences
- Distribution through single-binary channels remains available.
- Storage code is written against an interface from the start, which costs
  little now and would cost a rewrite later.
- The choice of embedded engine is constrained by clause 3 before it is
  evaluated on any other axis.

## Out of scope
- Clustered or highly available deployments.
- Externalised job queues (see ADR-0027).

## Assumption
Embedded storage is sufficient for one operator and tens of repositories.

## Acceptance criteria
- A build produces a single executable that runs server mode with no other
  process installed except `git`.
- The project cross-compiles for all supported platforms from one machine
  without a platform-specific toolchain.
- No code outside the storage implementation references the concrete engine.

## Dependencies
None.
