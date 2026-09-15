# WP-0003: Enforcement skeleton

**Area:** foundation
**Implements:** ADR-0055, ADR-0056, ADR-0057
**Requires:** WP-0001
**Status:** Ready

## Goal
Every rule in ADR-0056 whose subject already exists is enforced by tooling, the
checker harness exists for rules whose subject does not exist yet, and both CI
gates run within their budgets.

## In scope
1. Configure the linter with every rule in ADR-0056 table 1 whose subject
   exists in the repository today. A rule whose subject does not yet exist is
   recorded in a tracking list inside the linter configuration as a comment
   naming the work package that will introduce it.
2. Create the checker harness: a test package in which each checker is a
   separate test, and each failure message begins with the governing record
   number, in the form `ADR-NNNN: <what is wrong>` (ADR-0055 clause 3).
3. Implement these checkers now, because their subjects already exist:
   - **record integrity** — every file in `docs/decisions/` carries the
     template sections; `INDEX.md` lists every record; no accepted record
     contains a date, duration, milestone or phase name; every `ADR-NNNN`
     reference in any file resolves to an existing record.
   - **taxonomy integrity** — every archetype in both lists has a non-empty
     `id`, `name` and `description`; identifiers are unique across the file;
     every axis referenced in a condition appears in `axes.md`.
   - **decision reference** — every file listed in ADR-0059 clause 1 that
     exists today carries at least one resolvable record reference.
4. Create the two CI gates defined in ADR-0057 with their contents and
   budgets. Assign every implemented check to exactly one gate.
5. Add a `make` target for each gate so both can be run locally with the same
   contents as CI.
6. Add the dependency allow list required by ADR-0049 clause 2 and 3, populated
   from the WP-0001 audit, and the checker that compares it against the
   manifests.

## Out of scope
- Checkers whose subject does not exist yet: determinism, incremental
  equivalence, identity projection, mode capability matrix, model-free
  equivalence, bundle integrity, family contract, namespace violation,
  archetype reachability, subprocess count. Each is introduced by the package
  that creates its subject.
- Fixing any violation the new checkers report in code that later packages
  rewrite. If a checker fails on code scheduled for replacement, record it in
  `docs/work/audit.md` and leave the checker failing only if the failure is in
  a file this package may touch; otherwise narrow the checker's scope and note
  the narrowing with the package number that will widen it.
- Changing any application behaviour.
- Commit hooks.

## Files
**May create or modify:** linter configuration, `Makefile`, CI workflow files,
`internal/checks/**`, the dependency allow list file, `docs/work/audit.md`.
**Must not touch:** `docs/decisions/**`, `docs/metrics.md`, `AGENTS.md`,
`internal/pipeline/interpret/taxonomy/**`, application source outside what a
linter fix requires.

## Steps
1. Read `docs/work/audit.md`.
2. Add linter configuration rule by rule, running the linter after each so that
   an unexpected mass failure is attributed to the rule that caused it.
3. Create `internal/checks/` and the harness.
4. Implement the three checkers in clause 3.
5. Populate the dependency allow list from the audit and add its checker.
6. Write the two CI gate definitions and the matching `make` targets.
7. Measure both gates and record their durations in the CI configuration as a
   comment.

## Definition of done
- Every checker failure message matches `^ADR-[0-9]{4}: `.
- Introducing a deliberate violation of each implemented rule fails the build;
  reverting it passes.
- Adding a direct dependency absent from the allow list fails the build.
- The fast gate completes within 5 minutes and the full gate within 15 minutes
  on the project's CI runner.
- Every implemented check belongs to exactly one gate.

## Verification
```
make gate-fast     # passes, under 5 minutes
make gate-full     # passes, under 15 minutes
# for each implemented rule: introduce a violation, confirm failure, revert
```
