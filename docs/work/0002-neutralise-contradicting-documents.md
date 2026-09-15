# WP-0002: Neutralise contradicting documents

**Area:** foundation
**Implements:** ADR-0001, ADR-0004, ADR-0021, ADR-0034
**Requires:** WP-0001

## Goal
No document in the repository states a rule that contradicts the accepted
decision set, and no document links to a file that does not exist.

## In scope
1. Replace `docs/project-overview.md` with a document that describes only what
   the decision set establishes, and that names `docs/decisions/INDEX.md` as the
   binding source. It must not restate individual decisions; it summarises and
   points.
2. Rewrite `README.md` as an interim document that is accurate about the current
   state: what the project is, that it is pre-release, how to build, and where
   the decisions live. Remove every claim the decision set has invalidated, at
   minimum: static HTML output, `--no-blame`, `--wrapped` HTML pages, `--json`,
   per-author opt-in framing, "React and MUI application", and the
   `report-schema.json` references.
3. **Remove every link to a file that is not tracked.** The audit lists eight
   such targets, referenced from `README.md`, `CHANGELOG.md`,
   `docs/project-overview.md` and `web/src/api/client.ts`. Delete the link and
   the sentence that depends on it. Do not create the missing file, and do not
   replace the link with a description of a document that does not exist.
4. Move `docs/report-schema.json` to `docs/legacy/report-schema-v0.json`. Update
   the three tracked references to the old path: `internal/aggregate/schema_test.go`,
   `internal/server/securitymatrix_test.go` and `.goreleaser.yml`. **These two
   tests must keep asserting exactly what they assert now; only the path
   changes.** Do not write a new schema; that is WP-0008.
5. Truncate `CHANGELOG.md` to a single entry stating that no version has been
   released and that history before this point predates the current decision set.
6. Remove or correct every comment, doc comment and string literal listed in the
   WP-0001 audit section 2, without changing any runtime behaviour.
7. If `docs/docker.md` is created in a later package, its content is that
   package's concern. This package neither creates nor describes it.

## Out of scope
- Writing the final user-facing README. That is WP-0060.
- Writing the new report schema. That is WP-0008.
- Deleting phase documents. **None is tracked**; only links to them exist, and
  clause 3 covers those.
- Any change to code behaviour. Comment, literal and path edits must not alter
  control flow or any assertion other than the schema file's location.
- Adding documentation about features that do not exist.

## Files
**May create or modify:** `README.md`, `CHANGELOG.md`, `docs/project-overview.md`,
`docs/legacy/**`, `.goreleaser.yml`, `internal/aggregate/schema_test.go`,
`internal/server/securitymatrix_test.go`, `web/src/api/client.ts`, and the
specific source files listed in WP-0001 audit section 2.
**Must not touch:** `docs/decisions/**`, `docs/metrics.md`, `docs/conventions.md`,
`AGENTS.md`, `CLAUDE.md`, `internal/pipeline/interpret/taxonomy/**`, any source
file not listed in the audit.

## Steps
1. Read `docs/work/audit.md`, sections 1 and 2.
2. Move the schema to `docs/legacy/` and update the three references. Run the
   test suite; it must pass with the same results as before.
3. Rewrite `docs/project-overview.md`, `README.md`, `CHANGELOG.md`.
4. Remove the dangling links from every file that carries one.
5. Apply the comment and literal edits from audit section 2, one file at a time.
6. Run the test suite again and confirm the result is unchanged from step 2.

## Definition of done
- Every relative link in every tracked Markdown file resolves to a tracked file.
- No file outside `docs/legacy/` and `docs/decisions/` mentions `--no-blame`,
  `--wrapped` as an HTML output, or `report-schema.json` except as a removed or
  superseded capability.
- `docs/report-schema.json` does not exist; `docs/legacy/report-schema-v0.json`
  does.
- Every reference listed in audit section 2 is removed or corrected.
- The test suite passes with the same results as before this package, apart from
  the two path updates in clause 4.

## Verification
```
# every relative markdown link resolves
git grep -ohE '\]\([^)#][^)]*\)' -- '*.md' | tr -d '()' | sed 's/^]//' \
  | while read -r p; do [ -e "$p" ] || echo "DANGLING: $p"; done
git grep -n -- "--no-blame" -- . ':!docs/legacy' ':!docs/decisions'
test ! -e docs/report-schema.json && test -e docs/legacy/report-schema-v0.json
make test
```
