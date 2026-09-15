# WP-0002: Neutralise contradicting documents

**Area:** foundation
**Implements:** ADR-0001, ADR-0004, ADR-0021, ADR-0034
**Requires:** WP-0001

## Goal
No document in the repository states a rule that contradicts the accepted
decision set, and no document describes phases, schedules or removed
capabilities.

## In scope
1. Replace `docs/project-overview.md` with a document that describes only what
   the decision set establishes, and that names `docs/decisions/INDEX.md` as
   the binding source. It must not restate individual decisions; it summarises
   and points.
2. Rewrite `README.md` as an interim document that is accurate about the
   current state: what the project is, that it is pre-release, how to build,
   and where the decisions live. Remove every claim the decision set has
   invalidated, at minimum: static HTML output, `--no-blame`, per-author
   opt-in framing, "no database", phase descriptions, and any Docker or CLI
   instruction that no longer holds.
3. Delete every phase document under `docs/`.
4. Mark `docs/report-schema.json` superseded: move it to
   `docs/legacy/report-schema-v0.json` and add a one-line header comment in
   the accompanying documentation stating it does not describe the current
   report. Do not write a new schema here; that is WP-0008.
5. Truncate `CHANGELOG.md` to a single entry stating that no version has been
   released and that history before this point predates the current decision
   set.
6. Remove or correct every comment, doc comment and string literal listed in
   the WP-0001 audit under clause 5, without changing any runtime behaviour.
7. Update `docs/docker.md` to remove instructions for capabilities the decision
   set removed. If a section describes behaviour that no longer exists and has
   no replacement yet, delete the section rather than describing a future one.

## Out of scope
- Writing the final user-facing README. That is WP-0060.
- Writing the new report schema. That is WP-0008.
- Any change to code behaviour. Comment and string edits must not alter
  control flow, output text that tests assert on, or CLI help semantics beyond
  removing references to removed capabilities.
- Adding aspirational documentation about features that do not exist.

## Files
**May create or modify:** `README.md`, `CHANGELOG.md`, `docs/project-overview.md`,
`docs/docker.md`, `docs/legacy/**`, and the specific source files listed in the
audit under clause 5.
**Must not touch:** `docs/decisions/**`, `docs/metrics.md`, `docs/conventions.md`,
`AGENTS.md`, `internal/pipeline/interpret/taxonomy/**`, any source file not
listed in the audit.

## Steps
1. Read `docs/work/audit.md`.
2. Delete the phase documents.
3. Move the old schema to `docs/legacy/`.
4. Rewrite `docs/project-overview.md`, `README.md`, `CHANGELOG.md`.
5. Edit `docs/docker.md`.
6. Apply the comment and literal edits from the audit, one file at a time.
7. Run the existing test suite and confirm no behavioural change.

## Definition of done
- No file under `docs/` outside `docs/legacy/` contains the words `Phase 1`,
  `Phase 1.5`, `Phase 2` or `Phase 3`.
- No file outside `docs/legacy/` and `docs/decisions/` mentions `--no-blame`
  except as a removed capability.
- `README.md` contains no instruction that fails when followed against the
  current build.
- Every reference listed in the audit under clause 5 is either removed or
  corrected.
- The existing test suite passes with the same results as before this package.

## Verification
```
grep -rn "Phase 1\|Phase 2\|Phase 3" docs/ --exclude-dir=legacy   # no output
grep -rn -- "--no-blame" . --exclude-dir=docs/legacy --exclude-dir=docs/decisions
make test                                                          # unchanged result
```
