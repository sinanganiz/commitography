# WP-0005: Package layout migration

**Area:** foundation
**Implements:** ADR-0040, ADR-0060, ADR-0061
**Requires:** WP-0003, WP-0004

## Goal
The source tree matches ADR-0040 as extended by ADR-0060, the import direction
rule is enforced by the linter, and every golden file is unchanged by the move.

## In scope
1. Create the package tree defined in ADR-0040 clause 1 plus `internal/checks/`
   from ADR-0060 clause 2:
   `internal/core/`, `internal/metrics/`,
   `internal/pipeline/{collect,replay,aggregate,interpret,render}/`,
   `internal/storage/`, `internal/server/`, `internal/git/`,
   `internal/checks/`.
2. Move existing code into the tree according to its responsibility, using the
   destinations fixed by ADR-0060 clause 1 for the packages ADR-0040 does not
   name. Where existing code spans two destinations, split it along the
   responsibility boundary; do not place it in one destination and leave a
   cross-import.
3. **The current aggregation package holds four responsibilities at once**: the
   report type, build orchestration, privacy rewriting and every metric
   computation. Split it: the report type to `core`, orchestration to
   `pipeline/aggregate`, privacy to `core`, metric computations to one package
   per family under `internal/metrics/`. This split moves code; it does not
   rewrite any computation.
4. **The current asset location moves with the render package.** Update every
   place that names it: the build tool's output directory, the attributes file,
   the ignore file comment, and the bundle test. These four are in the allow
   list for this reason only; no other frontend change is permitted.
5. Enable the import direction lint rule from ADR-0040 clause 2, the metrics
   cross-import prohibition from clause 3, and the `internal/checks/` rule from
   ADR-0060 clause 2.
6. Move process execution out of every package except `internal/git/`. The audit
   lists four importers; one of them opens a browser rather than git, so it
   needs its own non-git mechanism rather than a move.
7. Apply ADR-0061: build metadata becomes one file under `internal/core/` with
   one lint suppression naming that record. Its value is read at composition and
   injected onward.
8. Add the file-level record references required by ADR-0059 clause 1 to every
   file in that list that now exists, and enable the decision reference checker
   for the files that exist.
9. Where an import direction violation cannot be resolved by moving code,
   resolve it by moving the shared type into `internal/core/`. Do not resolve it
   by adding an interface whose only purpose is to invert an import.
10. Raise the declared Go version to one that covers the behaviour the code
    already depends on. The audit records that the tree relies on semantics
    newer than the declared version.

## Out of scope
- Any behavioural change. This package moves, splits and renames; it does not
  rewrite.
- Implementing the five pipeline stages. This package creates their packages and
  moves existing equivalents into them; the stage boundaries in ADR-0020 are
  implemented by WP-0011 through WP-0017.
- Deleting code the decision set will eventually remove. Code scheduled for
  removal moves with everything else and is removed by the package that replaces
  it.
- Any frontend change other than the output directory in clause 4.
- Refactoring for readability, renaming symbols beyond what the move requires,
  or reorganising functions within a file.

## Files
**May create or modify:** `internal/**`, `cmd/**`, `Makefile`, `go.mod`,
`go.sum`, `.gitattributes`, `.gitignore`, linter configuration, and
`web/vite.config.ts` **for its output directory only**.
**Must not touch:** `testdata/**`, golden files, `docs/**`, `web/**` other than
the single setting in clause 4.

## Steps
1. Create the empty package tree.
2. Move one package at a time. After each move, run the golden comparison and
   confirm no output change before making the next move.
3. Split the aggregation package last among the moves, because it has the most
   dependents.
4. Update the four asset-location references together, in one commit.
5. Enable the lint rules and resolve violations per clause 9, re-running the
   golden comparison after each resolution.
6. Apply clause 7, then clause 10.
7. Add record references and enable the decision reference checker.

## Definition of done
- Every package under `internal/` matches ADR-0040 clause 1 or ADR-0060
  clause 1.
- The import direction rule, the metrics cross-import rule and the
  `internal/checks/` rule are enabled and the build passes.
- Process execution is imported by `internal/git/` only.
- Exactly one file suppresses the package-variable lint rule, and its
  suppression names ADR-0061.
- **Every golden file is byte-identical to its state before this package.**
- Every file in ADR-0059 clause 1 that exists carries a resolvable record
  reference.
- `make fixtures` still leaves `git status --porcelain` empty.

## Verification
```
git diff --stat <base> -- testdata/     # empty: no golden file changed
make gate-full                           # passes
git grep -l '"os/exec"' -- internal cmd  # only internal/git
```
