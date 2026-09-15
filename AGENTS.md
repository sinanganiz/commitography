# Agent instructions

## Required reading

Before making any change to this repository, read
[`docs/decisions/INDEX.md`](docs/decisions/INDEX.md) and any record it lists
that touches the area you are changing.

**If you are modifying a file that carries record references in its file-level
comment, read those records first.** The references are there because the file
carries constraints that are not obvious from the code (ADR-0059).

## Binding versus guidance

| Location | Status |
|---|---|
| `docs/decisions/` | **Binding.** A violation is a defect. |
| `docs/conventions.md` | **Guidance.** A justified deviation is acceptable. |

A rule never lives in both places. Do not treat guidance as binding, and do not
treat a record as negotiable.

## Rules

1. **Do not contradict an accepted record.** If a task requires contradicting
   one, stop and write a superseding record first, following ADR-0001. Do not
   implement the contradiction and document it afterwards.

2. **Do not edit an accepted record.** Supersede it. Typo and link fixes are the
   only permitted edits.

3. **Update the index in the same commit** that adds or supersedes a record.

4. **No dates.** Do not add schedules, milestones, phases, sprints, estimates or
   any statement about when work will happen, to any file. Ordering is expressed
   only as implementation dependency. See ADR-0004.

5. **No adjectives as requirements.** Every requirement is a number or a
   prohibition. "Fast", "modern", "intuitive", "clean" and "user-friendly" are
   not requirements. See ADR-0001 clause 8.

6. **Golden files are the review surface.** If your change alters any metric
   output, the affected golden files change in the same commit and the commit
   message states why the output changed. Never regenerate golden files to make
   a test pass without understanding the cause. See ADR-0019.

7. **Changing a metric's meaning requires a version increment.** Including a
   threshold change or a default change. See ADR-0031.

8. **Adding a metric family** requires: an interface implementation, a declared
   input set, an owned namespace, a family version, and golden fixture coverage.
   Nothing else in the pipeline may need to change. See ADR-0024.

9. **Adding a rule requires adding its enforcement** in the same change. A rule
   without a checker accumulates violations and is later weakened. See ADR-0055.

10. **Never disable a checker or loosen a budget to make a build pass.** Fix the
    cause, reduce the fixture, or move the check from the fast gate to the full
    gate. Loosening a budget requires a record. See ADR-0054, ADR-0055,
    ADR-0057.

11. **Do not introduce a dependency** that is absent from the allow list,
    requires a C toolchain for cross-compilation, or makes an external service
    necessary at runtime. See ADR-0022, ADR-0035, ADR-0049.

12. **Do not add a hosting provider API call** to the analysis path. See
    ADR-0007.

13. **Treat every repository as hostile input.** Commit messages, file names,
    reference names and working tree content are attacker-controlled in every
    mode. See ADR-0045.

### Git

- Work directly on `main`. Do not create a branch for a work package.
- Commit in logical steps as you go. Do not put a whole package in one commit,
  and do not leave the package uncommitted at the end.
- Every commit message names its package: `WP-NNNN: <what changed>`.
- Every commit must leave the repository in a working state: the fast gate
  passes. Do not commit a broken intermediate step and fix it in the next one.
- A commit that changes a golden file states why in its body (ADR-0019). A
  commit that changes a budget value states the measurement behind it
  (ADR-0054).
- Do not amend, rebase, reset or force-push a commit that already exists.
- Do not push without being asked.

### Reporting

When you finish, or when you stop, report exactly this:

1. Each `Definition of done` item, with the evidence that satisfies it.
2. The actual output of every command in `Verification`. Paste it; do not
   summarise it and do not describe what it would show.
3. Anything in `In scope` you did not do, and why.
4. Anything you noticed that is out of scope and left alone.
5. If you stopped early, which rule stopped you.

Do not claim an item is satisfied without its evidence. Do not change the
package's status in `docs/work/INDEX.md`.

## When a record is silent

If the records do not cover a question, the question is an implementation detail
and your judgement applies. If the question determines an interface, an artifact
format, or a product guarantee, it is not an implementation detail — write a
record.

## Work

Work is defined in [`docs/work/INDEX.md`](docs/work/INDEX.md) as numbered work
packages. There are no phases, sprints or dates.

When you are given a work package number:

1. Read that package file in full before making any change.
2. Read every record listed in its `Implements` field.
3. Confirm every package in its `Requires` field has status `Done`. If one does
   not, stop and say so.
4. Work only inside its `Files` allow list. If the task cannot be completed
   without touching a file outside that list, **stop and report it**. Do not
   widen the list. A package that needs a file it may not touch is either
   mis-scoped or missing a dependency, and both are decisions for the author,
   not for you.
5. Do nothing that is not in `In scope`, including improvements that look
   obviously correct. Note them at the end of your report instead.
6. Finish only when every statement in `Definition of done` is satisfied. Run
   the commands in `Verification` and report their actual output. "It works" is
   not a completion criterion.

If a package contradicts a record, the record wins. Stop and report the
contradiction rather than choosing.

A package is never marked `Done` by the agent that implemented it. Report the
result; status lives only in `docs/work/INDEX.md` and is changed by the author.
Package files carry no status field.