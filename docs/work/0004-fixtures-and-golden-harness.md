# WP-0004: Deterministic fixtures and golden harness

**Area:** foundation
**Implements:** ADR-0019
**Requires:** WP-0003

## Goal
Fixture repositories are byte-for-byte reproducible on every supported platform,
generating them leaves the tracked tree clean, and the golden harness fails the
build when any analysis output changes without its golden file changing in the
same commit.

## In scope
1. **Generating fixtures must not modify tracked files.** The generator
   currently removes its output root, which deletes a tracked placeholder on
   every run. Generate into an ignored directory, or stop removing tracked
   paths. After `make fixtures`, `git status --porcelain` must be empty.
2. Make fixture generation deterministic: fixed author and committer timestamps,
   fixed timezone offsets, fixed identities, fixed file content, fixed commit
   order. Two generations on different machines must produce identical commit
   hashes.
3. **Remove every platform-conditional step from the generator.** The existing
   binary fixture adds a commit only where the filesystem accepts a quote in a
   file name, so its hashes differ between platforms, which ADR-0019 clause 1
   forbids. Hostile file names that not every filesystem accepts belong in a
   fixture that is generated identically everywhere or not at all: prefer names
   that are legal on every supported platform, and if a name is not, drop it and
   record the gap in the fixture's comment.
4. Reconcile the fixture set. The existing fixtures are `basic`, `mailmap`,
   `merges`, `binary`, `noise`, `bots`, `coupling`, `shallow`, `empty`,
   `single`. Keep each one that exercises a condition, renaming it to state that
   condition, and add the missing ones. The resulting set must cover at least:
   single contributor; one person committing under two addresses; renames and a
   copied block; hostile names within clause 3's constraint; a commit above the
   outlier threshold; generated and vendored paths; agent co-author trailers
   mixed with unassisted commits; a multi-year gap; a shallow clone; and a
   designated large fixture for performance measurement.
5. Add the fixture determinism checker (ADR-0056): generate twice, compare
   hashes. It must run on every supported platform in the full gate.
6. Create the golden harness: for each fixture, the produced analysis output is
   stored as a golden file and compared on every run. A mismatch fails the build
   and prints a diff.
7. Add the commit message rule from ADR-0019 clause 2 as a checker: a commit
   changing a golden file must state the reason in its message body.
8. Assign small-fixture golden comparison to the fast gate and large-fixture
   comparison to the full gate (ADR-0057).

## Out of scope
- Changing any metric, output format or behaviour to make a golden file smaller
  or tidier.
- Fixtures for metrics that do not exist yet. Adding a metric family adds its
  fixture in the same package (ADR-0024 clause 6).
- The performance budget itself. This package produces the large fixture; the
  budget arrives with the measurements it guards (WP-0050 territory).
- Rewriting the generator in another language.

## Files
**May create or modify:** `testdata/**`, `internal/checks/**`, `Makefile`,
`.gitignore`, CI workflow files.
**Must not touch:** application source, `docs/decisions/**`, `docs/metrics.md`,
`internal/pipeline/interpret/taxonomy/**`.

## Steps
1. Fix the tracked-file deletion first, and confirm `git status` is clean after
   `make fixtures`. Do nothing else until this holds.
2. Audit the generator for non-determinism: clock reads, randomness, map
   iteration order, locale dependence, filesystem ordering, platform
   conditionals.
3. Remove each source found, replacing it with a fixed value.
4. Add the determinism checker; confirm it passes twice in a row and in a clean
   checkout.
5. Reconcile the fixture set, one commit per fixture.
6. Generate golden files and commit them.
7. Add the golden comparison to both gates as specified in clause 8, and the
   golden commit message checker.

## Definition of done
- `make fixtures` leaves `git status --porcelain` empty.
- Two consecutive generations produce identical commit hashes for every fixture,
  on every supported platform.
- No conditional in the generator depends on the host filesystem or operating
  system.
- A golden file exists for every fixture.
- Altering any analysis output without updating its golden file fails the build.
- Updating a golden file without a stated reason in the commit body fails the
  build.
- Every condition in clause 4 is covered by a fixture whose name states it.

## Verification
```
make fixtures && git status --porcelain          # empty
make fixtures && <record hashes>                  # identical to previous run
make gate-fast                                    # golden comparison passes
grep -rnE 'uname|OSTYPE|\$OS|case .*mingw' testdata/   # no platform branches
```

## Note on golden content
The golden files produced here describe the current output format. WP-0008
replaces the report document, which invalidates every golden file at once. That
invalidation is expected and is exactly what this harness exists to make
visible: a large, deliberate, explained golden diff rather than a silent change.
