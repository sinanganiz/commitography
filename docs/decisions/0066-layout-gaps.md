# ADR-0066: The git package's position, and subpackages under core

**Status:** Accepted
**Note:** This record extends ADR-0040 and ADR-0060. It supersedes neither.

## Context
ADR-0040 clause 2 states the dependency direction but omits `internal/git`,
which every collection path must reach. ADR-0040 clause 1 also names `core` as a
single package, and the code moving into it carries colliding type names, so a
flat package would force renames that ADR-0040's own out-of-scope section
forbids in a move.

## Decision
1. `internal/git` sits beside `core` in the dependency direction: it MAY import
   `core` and nothing else from the tree, and it MAY be imported by
   `metrics`, `pipeline`, `storage`, `server` and `cmd`.
2. `internal/core` MAY contain subpackages. Report types, privacy and build
   metadata stay in `core` itself; configuration, identity, the domain model and
   filtering live in subpackages beneath it.
3. A subpackage under `core` MUST obey the same direction as `core`: it imports
   nothing else from the tree. Subpackages under `core` MAY import one another
   only in one direction, which the lint rule records; a cycle among them means
   the split is wrong.
4. Creating a subpackage under `core` to resolve a name collision during a move
   is permitted. Creating one to group unrelated helpers is not; that is the
   general-purpose utility package ADR-0060 forbids.

## Consequences
- The collection path can reach git without an inverted interface whose only
  purpose is to satisfy the direction rule, which ADR-0040 clause 9 forbids.
- Moves do not force renames, so a move stays a move.
- `core` remains the only place with no upward dependencies, subpackages
  included.

## Out of scope
- Subpackages under any other tree entry. `metrics`, `pipeline`, `storage` and
  `server` keep the structure ADR-0040 and ADR-0060 give them.

## Assumption
The four subpackages named in clause 2 are the set the current code needs. A
fifth is permitted under clause 4 only for the same reason.

## Acceptance criteria
- The import direction rule covers `internal/git` and every subpackage under
  `core`, and the build passes.
- No package under `core` imports anything outside `core`.
- No type was renamed to complete the move.

## Dependencies
ADR-0040 and ADR-0060 must be implemented before this one.
