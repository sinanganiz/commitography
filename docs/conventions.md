# Conventions

> **Status: guidance, not binding.** These are patterns that produce
> consistency but cannot be checked mechanically. A justified deviation is
> acceptable. This is the opposite of `docs/decisions/`, where a violation is a
> defect.
>
> When a convention here becomes mechanically enforceable, it moves to a checker
> and is deleted from this file. A rule never lives in both places (ADR-0058).
>
> When a convention here starts to bind an interface, an artifact format or a
> product guarantee, it is no longer a convention: write a record (ADR-0058
> clause 5).

---

## Naming

- Metric family package names match the family identifier in the report
  namespace exactly. If the report says `worktype`, the package is `worktype`.
- Reason codes read as conditions, not as apologies: `worktree_unavailable`,
  not `could_not_read_files`.
- Test fixtures are named for what they exercise, not for their size:
  `renamed_file_history`, not `fixture_medium`.
- Exported identifiers do not repeat their package name. `metrics.Family`, not
  `metrics.MetricFamily`.

## Comments

- The file-level comment on a constraint-bearing file states which records
  govern it and, in one line, what the file is responsible for. The record
  references are required by ADR-0059; the responsibility line is convention.
- Comments explain why, not what. A comment restating the next line is noise.
- A deliberate non-obvious choice carries a comment naming the alternative that
  was rejected and why. This is the single highest-value comment type in this
  codebase, because the alternative usually looks better at first glance.

## Tests

- Test names state the behaviour, not the function: `rework_requires_same_owner`,
  not `TestClassify3`.
- Invariant tests live next to the thing they constrain, not in a shared
  invariants package. A reader of the code should encounter them.
- A test that requires a comment to explain what it proves is usually two tests.
- Prefer a fixture repository over a mocked git output. Fixtures catch parsing
  and format assumptions; mocks encode them.

## Metric families

- A family computes and reports; it does not decide what is interesting. Ranking,
  thresholding and labelling belong to the interpret stage.
- When a metric could be defined two defensible ways, the report states which
  definition was used rather than choosing silently.
- New families start with the narrowest useful output. Widening a namespace is
  cheap; narrowing it is a version increment.

## Visualisations

- Each visualisation has one job. A chart that answers two questions answers
  neither.
- Lens behaviour is designed at the same time as the base visualisation, not
  added afterwards. A visualisation whose lens state is an afterthought usually
  gets replaced rather than transformed, which is what ADR-0028 forbids.
- Empty and truncated states are designed, not left to render as blank space.
  Under ADR-0032 they are visible product surfaces.

## Error messages

- Name the offending value. "Path is outside the allowed roots" is weaker than
  quoting the path and the roots.
- State the remedy in the same message, not in documentation the reader has to
  find.
- Write for someone who has just met the tool, not for someone who wrote it.

## Frontend

- Components receive data, not report objects. A component that reaches into a
  report structure is coupled to the schema.
- Layout computation stays outside components. Components render what they are
  given.
- The two visual languages share tokens and diverge in composition. If Wrapped
  needs a new token, ask whether the dashboard needs it too before adding it.

## Commit messages

- The subject says what changed in behaviour, not which files moved.
- A change touching a golden file explains why the output changed. This one is
  required by ADR-0019; it is repeated here because it is the convention most
  often skipped.
- A change that only satisfies a checker says which record it satisfies.
