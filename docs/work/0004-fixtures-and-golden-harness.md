# WP-0004: Deterministic fixtures and golden harness

**Area:** foundation
**Implements:** ADR-0019
**Requires:** WP-0003
**Status:** Ready

## Goal
Fixture repositories are byte-for-byte reproducible on any machine, and the
golden harness fails the build when any analysis output changes without its
golden file changing in the same commit.

## In scope
1. Make fixture generation fully deterministic: fixed author and committer
   timestamps, fixed timezone offsets, fixed identities, fixed file content,
   fixed commit order. Two generations on different machines must produce
   identical commit hashes.
2. Add the fixture determinism checker (ADR-0056): generate twice, compare
   hashes.
3. Define the fixture set. Each fixture exists to exercise a named condition,
   and its name states that condition. At minimum:
   - `single-contributor` — one identity, no in-repository comparison possible
   - `two-identities-one-person` — one person committing under two addresses
   - `renamed-and-moved` — renames and a copied block, for ownership divergence
   - `hostile-names` — newlines, quotes and control characters in paths and
     subjects
   - `bulk-import` — a commit above the outlier threshold
   - `generated-paths` — lockfiles, vendored trees, generated markers
   - `assisted-commits` — agent co-author trailers, mixed with unassisted
   - `long-silence` — a multi-year gap between commits
   - `shallow` — a shallow clone, for refusal behaviour
   - `large` — the designated performance fixture
4. Create the golden harness: for each fixture, the produced analysis output is
   stored as a golden file and compared on every run. A mismatch fails the
   build and prints a diff.
5. Add the commit message rule from ADR-0019 clause 2 as a checker: a commit
   changing a golden file must contain a stated reason in its message body.
6. Assign small-fixture golden comparison to the fast gate and large-fixture
   comparison to the full gate (ADR-0057).

## Out of scope
- Changing any metric, output format or behaviour to make a golden file
  prettier or smaller.
- Fixtures for metrics that do not exist yet. Adding a metric family adds its
  fixture in the same package (ADR-0024 clause 6).
- The performance budget itself. This package produces the `large` fixture; the
  budget is introduced with the measurements it guards.
- Cross-platform line ending normalisation of fixture content beyond what is
  required for determinism.

## Files
**May create or modify:** `testdata/**`, `internal/checks/**`, `Makefile`,
CI workflow files.
**Must not touch:** application source, `docs/decisions/**`,
`internal/pipeline/interpret/taxonomy/**`.

## Steps
1. Audit the current fixture generator for non-determinism: clock reads,
   randomness, map iteration order, locale dependence, filesystem ordering.
2. Remove each source found, replacing it with a fixed value.
3. Add the determinism checker and confirm it passes twice in a row and in a
   clean checkout.
4. Add each fixture in clause 3, one commit per fixture.
5. Generate golden files and commit them.
6. Add the golden comparison to both gates as specified in clause 6.
7. Add the golden commit message checker.

## Definition of done
- Two consecutive fixture generations produce identical commit hashes for every
  fixture.
- A golden file exists for every fixture.
- Modifying any analysis output without updating its golden file fails the
  build.
- Updating a golden file without a stated reason in the commit message body
  fails the build.
- Every fixture in clause 3 exists and its name matches its condition.

## Verification
```
make fixtures && git -C testdata/<name> rev-parse HEAD   # record
rm -rf testdata/<name> && make fixtures                  # same hash
make gate-fast                                           # golden comparison passes
```

## Note on golden content
The golden files produced here describe the current output format. WP-0008
replaces the report document, which will invalidate every golden file at once.
That invalidation is expected and is exactly the behaviour this harness exists
to make visible: it will appear as a large, deliberate, explained golden diff
rather than as a silent change.
