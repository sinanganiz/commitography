# WP-0001: Repository audit

**Area:** foundation
**Implements:** —
**Requires:** —

## Goal
`docs/work/audit.md` exists and classifies every file currently in the
repository as keep, change or delete, with a decision record cited for every
classification that is not "keep unchanged".

## In scope
1. Enumerate every tracked file in the repository, excluding `docs/decisions/`,
   `docs/metrics.md`, `docs/conventions.md`, `AGENTS.md` and
   `internal/pipeline/interpret/taxonomy/`, which are current by construction.
2. Classify each file as exactly one of: `keep`, `change`, `delete`.
3. For every `change` and `delete`, cite the ADR number that requires it.
4. For every `change`, state in one line what must change.
5. List every code comment, doc comment and string literal that references a
   document, flag, command or concept removed by the decision set. At minimum,
   search for references to: static HTML output, `--no-blame`, phase names,
   `wrapped-<year>.html` as a CLI output, `report-schema.json`,
   `project-overview.md`, and the removed component library.
6. List every existing test and state whether its subject survives the decision
   set.
7. List every direct Go dependency and every direct frontend dependency, and
   state whether it is still permitted under ADR-0049 and ADR-0022.
8. Record the current `report.json` top-level shape, so that WP-0008 can state
   what changes.

## Out of scope
- Any change to any file other than creating `docs/work/audit.md`.
- Deleting anything.
- Fixing anything found.
- Opinions about implementation quality. The audit records what exists and
  which decision governs it, not whether the code is good.

## Files
**May create or modify:** `docs/work/audit.md`
**Must not touch:** everything else.

## Steps
1. Produce the tracked file list.
2. For each file, read enough to classify it and identify the governing ADRs.
3. Run the reference searches in clause 5 and record every hit with file and
   line.
4. Record dependencies and the current report shape.
5. Write `docs/work/audit.md` as a table per section, using the classifications
   above.

## Definition of done
- `docs/work/audit.md` exists.
- Every tracked file outside the exclusions in clause 1 appears exactly once in
  the audit table.
- Every row classified `change` or `delete` cites at least one ADR number that
  exists in `docs/decisions/`.
- `git status` reports exactly one changed path.

## Verification
```
git status --porcelain            # exactly one line, docs/work/audit.md
git ls-files | wc -l              # compare against row count in audit.md
grep -oE 'ADR-[0-9]{4}' docs/work/audit.md | sort -u   # each must exist
```
