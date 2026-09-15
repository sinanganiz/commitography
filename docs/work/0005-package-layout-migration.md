# WP-0005: Package layout migration

**Area:** foundation
**Implements:** ADR-0040
**Requires:** WP-0003, WP-0004

## Goal
The source tree matches the layout in ADR-0040, the import direction rule is
enforced by the linter, and every golden file is unchanged by the move.

## In scope
1. Create the package tree defined in ADR-0040 clause 1:
   `internal/core/`, `internal/metrics/`, `internal/pipeline/{collect,replay,aggregate,interpret,render}/`,
   `internal/storage/`, `internal/server/`, `internal/git/`.
2. Move existing code into the tree according to its responsibility. Where
   existing code spans two destinations, split it along the responsibility
   boundary; do not place it in one destination and leave a cross-import.
3. Enable the import direction lint rule from ADR-0040 clause 2 and the
   metrics cross-import prohibition from clause 3.
4. Add the file-level record references required by ADR-0059 clause 1 to every
   file in that list that now exists, and enable the decision reference checker
   for the files that exist.
5. Where an import direction violation cannot be resolved by moving code,
   resolve it by moving the shared type into `internal/core/`. Do not resolve
   it by adding an interface whose only purpose is to invert an import.

## Out of scope
- Any behavioural change. This package moves and renames; it does not rewrite.
- Introducing the five pipeline stages as working code. This package creates
  their packages and moves existing equivalents into them; the stage boundaries
  defined in ADR-0020 are implemented by WP-0011 through WP-0017.
- Deleting code that the decision set will eventually remove. Code scheduled
  for removal moves with everything else and is removed by the package that
  replaces it.
- Refactoring for readability, renaming symbols beyond what the move requires,
  or reorganising functions within a file.

## Files
**May create or modify:** `internal/**`, `cmd/**`, `Makefile`, linter
configuration.
**Must not touch:** `testdata/**`, golden files, `docs/**`, `web/**`.

## Steps
1. Create the empty package tree.
2. Move one package at a time. After each move, run the golden comparison and
   confirm no output change before making the next move.
3. After all moves, enable the import direction rule and resolve violations
   according to clause 5, re-running the golden comparison after each
   resolution.
4. Enable the metrics cross-import rule.
5. Add record references and enable the decision reference checker.

## Definition of done
- Every package in the tree matches ADR-0040 clause 1.
- The import direction lint rule is enabled and the build passes.
- The metrics cross-import rule is enabled and the build passes.
- **Every golden file is byte-identical to its state before this package.**
- Every file in ADR-0059 clause 1 that exists carries a resolvable record
  reference.

## Verification
```
git diff --stat <base> -- testdata/    # empty: no golden file changed
make gate-full                          # passes
```
