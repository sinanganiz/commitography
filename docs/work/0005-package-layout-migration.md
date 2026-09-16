# WP-0005: Package layout migration

**Area:** foundation
**Implements:** ADR-0040, ADR-0049, ADR-0060, ADR-0061, ADR-0063, ADR-0065, ADR-0066
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
6. Move **git invocation** out of every package except `internal/git/`. The
   audit counted only non-test files; the tree has nine importers of the
   process-execution library. Every one that runs git switches to the git
   package. The sites permitted by ADR-0065 clause 3 keep their own execution
   and gain a file-level reference to that record: the verification packages
   under `internal/checks/`, the browser opener, and the Windows test that
   creates a directory junction. Configure the lint rule to the same set.
6a. `internal/git` takes the position ADR-0066 clause 1 gives it. `internal/core`
   takes the subpackages ADR-0066 clause 2 permits, so that the move requires no
   rename.
6b. **Record the aggregation stage's remaining git calls as a known deviation.**
   The current code calls git from aggregation, and ADR-0020 clause 3 makes
   replay the only stage with working tree access. Moving the calls now would
   reorder the server's progress events, which is a behavioural change this
   package forbids. Leave them, record the deviation next to the code and in the
   linter's tracking list, and name **WP-0013** as the package that removes it.
6c. **The golden harness no longer calls the command in process.** Once the
   command is `package main`, nothing can import it. The harness calls the
   pipeline root and the report writer directly and takes exit classification
   from the root's exported error types. The command's own tests move with it
   and keep asserting exit code 2 for the shallow and empty repositories.
   **The CLI half of the CLI/server equivalence checker (ADR-0063 table 2)
   therefore lives in the command's own test package**, because nothing else can
   reach it. Confirm it still runs after the move.
7. Apply ADR-0061: build metadata becomes one file under `internal/core/` with
   one lint suppression naming that record. Its value is read at composition and
   injected onward. **The release configuration's three linker flags name the
   old package path directly.** The linker ignores a flag naming a variable that
   does not exist, without an error, so leaving them would silently produce
   release binaries reporting a default version. Repoint all three at the new
   location in the same commit as the move.
8. Add the file-level record references required by ADR-0059 clause 1 to every
   file in that list that now exists, and enable the decision reference checker
   for the files that exist.
9. Where an import direction violation cannot be resolved by moving code,
   resolve it by moving the shared type into `internal/core/`. Do not resolve it
   by adding an interface whose only purpose is to invert an import.
10. Raise the declared Go version to one that covers the behaviour the code
    already depends on. The audit records that the tree relies on semantics
    newer than the declared version. WP-0003 pinned the gates to a working
    toolchain without editing the manifest; this package makes the manifest
    agree.
11. **Remove every clock-derived build timestamp.** Two builds of one commit
    currently differ, which ADR-0049 forbids and ADR-0063 clause 3 prohibits by
    name. Replace the value with the commit's own timestamp **in both the local
    build and the release configuration**; fixing only one leaves a binding
    record broken in the other. Enable the reproducible build checker from
    ADR-0063 table 2.
12. Update the citations in the linter configuration header from ADR-0056 to
    ADR-0063, which supersedes it.
13. Correct the three comments in the frontend that name packages this package
    moves. These are comment-only edits and change no behaviour.

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
`go.sum`, `.gitattributes`, `.gitignore`, linter configuration,
`web/vite.config.ts` **for its output directory only**, the release
configuration **for its three linker flags and its build date only**, and
`web/index.html`, `web/src/types.ts`, `web/src/app/format.ts` **for their
package-name comments only**.
**Must not touch:** `testdata/**`, golden files, `docs/**`, and `web/**` other
than the four files named above, each for the single reason named.

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
- Git is invoked from `internal/git/` only.
- The process-execution library is imported only by `internal/git/` and the
  sites ADR-0065 clause 3 permits, each carrying a reference to that record.
- The aggregation stage's remaining git calls are recorded as a deviation naming
  WP-0013.
- Exactly one file suppresses the package-variable lint rule, and its
  suppression names ADR-0061.
- **Every golden file is byte-identical to its state before this package.**
- Every file in ADR-0059 clause 1 that exists carries a resolvable record
  reference.
- `make fixtures` still leaves `git status --porcelain` empty.
- Two builds of the same commit produce identical binaries, verified by the
  reproducible build checker, for the local build and the release build alike.
- Every linker flag names a variable that exists. A build with the release
  configuration reports a real version, not a default.
- No **comment or identifier in source** names a package path this package
  moved. Documents under `docs/` are historical records of what the tree was and
  are not corrected by this package.

## Verification
```
git diff --stat <base> -- testdata/     # empty: no golden file changed
make gate-full                           # passes
# only internal/git and the ADR-0065 clause 3 sites
git grep -l '"os/exec"' -- internal cmd \
  ':!internal/git' ':!internal/checks' ':!*path_windows_test.go'
# the only remaining hit is the browser opener

git grep -n 'exec.Command' -- internal cmd | grep -v internal/git | \
  xargs -I{} true   # each remaining file must carry an ADR-0065 reference

# every -X flag target must exist
git grep -ohE '\-X [^ "]+' .goreleaser.yml Makefile | sed 's/-X //;s/=.*//' |
  while read -r s; do git grep -q "${s##*.}" -- internal || echo "MISSING: $s"; done

git grep -nE 'internal/(version|aggregate|analysis|render)' -- web  # no output
```
