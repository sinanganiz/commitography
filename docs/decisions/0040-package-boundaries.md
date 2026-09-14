# ADR-0040: Package boundaries follow pipeline stages, with a one-way dependency rule

**Status:** Accepted

## Context
Five stages, ten metric families, storage and two modes do not fit in one
internal package. Layered naming schemes require a judgement call about which
layer a type belongs to, and that judgement is re-made differently by every
session. The pipeline stages and metric families are already defined by
decision, so they can supply the boundary instead.

## Decision
1. The layout MUST be:
   - `internal/core/` — report types, identity, configuration, shared derived
     data;
   - `internal/metrics/<family>/` — one package per family in ADR-0024;
   - `internal/pipeline/{collect,replay,aggregate,interpret,render}/`;
   - `internal/storage/`, `internal/server/`, `internal/git/`;
   - `cmd/`, `web/`.
2. Dependency direction MUST be one-way and MUST be enforced by a lint rule:
   - `core` imports nothing from the list above;
   - `metrics/*` may import `core` only;
   - `pipeline/*` may import `core` and `metrics/*`;
   - `storage` and `server` may import the above; `pipeline` and `metrics` MUST
     NOT import `storage` or `server`.
3. `internal/metrics/<a>` MUST NOT import `internal/metrics/<b>`. This makes
   ADR-0024 clause 3 a compile-time property rather than a convention.
4. Shared derived data needed by more than one family MUST live in `core` or be
   produced by an earlier stage. It MUST NOT be reached through a family.
5. New code MUST be placed according to this layout. A package that does not fit
   indicates a missing decision, not a missing folder.

## Consequences
- Package placement is determined by existing decisions rather than debated.
- Family isolation is enforced by the compiler and the linter.
- Test isolation is straightforward because lower packages have no upward
  dependencies.

## Out of scope
- Hexagonal ports and adapters, or a layered naming scheme requiring
  per-type classification.

## Assumption
Stage-shaped packages accommodate shared derived data via `core`. If not, a
shared derived-data package is added under `core`; the rule does not change.

## Acceptance criteria
- A lint rule fails the build on any import violating clause 2 or 3.
- Every package in the tree matches the layout in clause 1.

## Dependencies
ADR-0020 and ADR-0024 must be implemented before this one.
