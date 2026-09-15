# ADR-0060: Destinations for packages the layout does not name

**Status:** Accepted
**Note:** This record extends ADR-0040. It does not supersede it.

## Context
ADR-0040 clause 1 names the package tree and clause 5 states that a package
which does not fit indicates a missing decision rather than a missing folder.
The repository audit found six packages with no destination in that list:
orchestration used by both the CLI and the server, CLI presentation, container
detection, build metadata, and two build-tagged verification packages. Without a
recorded destination, each migration session would choose differently.

## Decision
1. The following destinations are fixed:

   | Existing package | Destination | Reason |
   |---|---|---|
   | Pipeline orchestration | `internal/pipeline/` root | It composes the five stages; it sits above them and inside none of them |
   | CLI presentation and exit handling | `cmd/commitography/` | Command-surface concern; ADR-0040 already names `cmd/` |
   | Build metadata | `internal/core/` | Read-only data with no dependencies |
   | Container and runtime environment detection | `internal/server/` | Its only consumer is listener behaviour |
   | Build-tagged verification packages | `internal/checks/` | Verification, not application code |

2. `internal/checks/` is added to the ADR-0040 clause 1 tree. It holds the
   checker harness and build-tagged verification packages. It MAY import any
   package it verifies; nothing MAY import it.
3. The dependency direction rule in ADR-0040 clause 2 applies unchanged to these
   destinations. `internal/pipeline/` root MAY import `core`, `metrics` and the
   five stage packages; it MUST NOT import `storage` or `server`.
4. A package that fits none of these destinations and none in ADR-0040 clause 1
   still indicates a missing decision. Do not invent a folder.

## Consequences
- The layout migration has a destination for every existing package, so it is a
  mechanical move rather than a series of judgement calls.
- `internal/checks/` gives the checker harness a home that is exempt from the
  metrics and pipeline import restrictions, because it must reach across the
  tree to verify it.
- CLI presentation moving into `cmd/` means the command surface is one package
  rather than two, which removes the current import from CLI code into the
  server package.

## Out of scope
- Splitting orchestration across the five stage packages.
- A general-purpose utility package. Shared code goes to `core` or to the stage
  that produces it (ADR-0040 clause 4).

## Assumption
The six packages named above are the complete set without a destination. If the
migration finds another, this record is extended by a superseding one rather
than by widening a folder silently.

## Acceptance criteria
- No package remains under `internal/` outside the trees named by ADR-0040
  clause 1 and clause 2 of this record.
- The import direction lint rule covers `internal/checks/` with the exemption in
  clause 2 and rejects any import of it.

## Dependencies
ADR-0040 must be implemented before this one.
