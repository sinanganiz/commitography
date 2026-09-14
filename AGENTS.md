# Agent instructions

## Required reading

Before making any change to this repository, read
[`docs/decisions/INDEX.md`](docs/decisions/INDEX.md) and any record it lists
that touches the area you are changing.

The architecture decision records in `docs/decisions/` are binding. They
describe what this project is, what it refuses to be, and why. A change that
contradicts an accepted record is wrong, regardless of how reasonable it looks
in isolation.

## Rules

1. **Do not contradict an accepted record.** If a task requires contradicting
   one, stop and write a superseding record first, following ADR-0001. Do not
   implement the contradiction and document it afterwards.

2. **Do not edit an accepted record.** Supersede it. Typo and link fixes are
   the only permitted edits.

3. **Update the index in the same commit** that adds or supersedes a record.

4. **No dates.** Do not add schedules, milestones, phases, sprints, estimates or
   any statement about when work will happen, to any file. Ordering is expressed
   only as implementation dependency. See ADR-0004.

5. **No adjectives as requirements.** Every requirement must be a number or a
   prohibition. "Fast", "modern", "intuitive", "clean" and "user-friendly" are
   not requirements. See ADR-0001 clause 8.

6. **Golden files are the review surface.** If your change alters any metric
   output, the affected golden files must change in the same commit and the
   commit message must state why the output changed. Never regenerate golden
   files to make a test pass without understanding the cause. See ADR-0019.

7. **Changing a metric's meaning requires a version increment.** Including a
   threshold change or a default change. See ADR-0031.

8. **Adding a metric family** requires: an interface implementation, a declared
   input set, an owned namespace, a family version, and golden fixture
   coverage. Nothing else in the pipeline may need to change. See ADR-0024.

9. **Do not introduce a dependency** that requires a C toolchain for
   cross-compilation, or that would make an external service necessary at
   runtime. See ADR-0022.

10. **Do not add a hosting provider API call** to the analysis path. See
    ADR-0007.

## When a record is silent

If the records do not cover a question, the question is an implementation
detail and your judgement applies. If the question determines an interface, an
artifact format, or a product guarantee, it is not an implementation detail —
write a record.
