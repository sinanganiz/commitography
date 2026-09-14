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

## When a record is silent

If the records do not cover a question, the question is an implementation detail
and your judgement applies. If the question determines an interface, an artifact
format, or a product guarantee, it is not an implementation detail — write a
record.
