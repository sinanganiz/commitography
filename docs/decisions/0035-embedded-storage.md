# ADR-0035: Embedded SQLite for metadata, files for blobs

**Status:** Accepted

## Context
ADR-0022 forbids requiring an external service and forbids anything that breaks
cross-compilation. ADR-0017 stores three kinds of data with different shapes:
registry, job records, person records and version indexes want relational
querying and schema evolution; reports and replay checkpoints are large, opaque
and read whole.

## Decision
1. Metadata MUST be stored in an embedded SQLite database: registry, job
   records, person records, version index, and references to blobs.
2. The SQLite driver MUST be a pure Go implementation. A driver requiring a C
   toolchain MUST NOT be used. This is an elimination criterion evaluated before
   any other property of a candidate driver.
3. Cross-compilation for every supported platform from a single machine without
   a platform-specific toolchain MUST be verified in continuous integration.
4. Reports and replay checkpoints MUST be stored as compressed files outside the
   database. The database MUST hold references to them, not their contents.
5. Schema changes MUST be applied through versioned, forward-only migrations
   that run at startup. An unrecognised newer schema version MUST cause a
   refusal to start rather than an attempted downgrade.
6. All access MUST go through the storage interface required by ADR-0022
   clause 2. No package outside the storage implementation may reference SQLite.

## Consequences
- Schema evolution has a defined mechanism, which matters because entities will
  be added over time without close human review.
- The database file stays small and does not require vacuuming, because blobs
  live outside it.
- Blobs are independently inspectable and backupable.

## Out of scope
- Server-based databases, clustering, replication.
- Storing analysis blobs inside the database.

## Assumption
A pure Go SQLite driver performs adequately for one operator and tens of
repositories. This must be measured, not assumed.

## Acceptance criteria
- `go build` produces working binaries for every target platform from one
  machine with no C toolchain.
- No package outside `internal/storage` imports the database driver.
- Starting against a newer schema version refuses rather than migrating down.

## Dependencies
ADR-0017 and ADR-0022 must be implemented before this one.
