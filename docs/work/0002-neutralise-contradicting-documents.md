# WP-0002: Neutralise contradicting documents

**Area:** foundation
**Implements:** ADR-0001, ADR-0004, ADR-0021, ADR-0034
**Requires:** WP-0001

## Goal
No document in the repository states a rule that contradicts the accepted
decision set, no document links to a file that does not exist, and no prose
describes a capability the tree no longer has.

## The line this package does not cross

This package changes **documentation and prose**. It changes no behaviour.

A reference is in scope only when it describes something that **does not exist
in the tree after this package**. A reference to a capability that still exists
is accurate, and making it inaccurate would be a defect. In particular, the
following are **behaviour**, not prose, and stay exactly as they are until the
package that removes the capability:

- flag names, option names and JSON keys
- arguments passed to a program in a test or script
- test names, assertion messages and fixture values
- CSS selectors, class names and dependency entries
- identifiers such as `NoBlame`, `IndexFile`, `RenderWrapped`, `WrappedFileName`
- comments and log lines that accurately describe code that still runs

WP-0001's audit section 2 lists every reference to a capability the decision set
removes. Most of those capabilities are still present. Section 2 is therefore an
inventory for the whole migration, not a work list for this package.

## In scope
1. Replace `docs/project-overview.md` with a document that describes only what
   the decision set establishes, and that names `docs/decisions/INDEX.md` as the
   binding source. It must not restate individual decisions; it summarises and
   points.
2. Rewrite `README.md` as an interim document that is accurate about the current
   state: what the project is, that it is pre-release, how to build, and where
   the decisions live. Remove every claim the decision set has invalidated.
3. Remove every link to a file that is not tracked. Delete the link and the
   sentence that depends on it. Do not create the missing file, and do not
   replace the link with a description of a document that does not exist.
4. Move `docs/report-schema.json` to `docs/legacy/report-schema-v0.json`, update
   its `$id` to the new path, and update the three tracked references to the old
   path. **Those two tests must keep asserting exactly what they assert now;
   only the path changes.** Do not write a new schema; that is WP-0008.
5. Truncate `CHANGELOG.md` to a single entry stating that no version has been
   released and that history before this point predates the current decision set.
6. Correct **prose only**: comments, doc comments and documentation text that
   name a phase, a milestone, a prior plan task or exit criterion, a document
   that is not tracked, or a capability that this package removes. Leave every
   item listed under "The line this package does not cross".
7. Classify and handle `PROMPTS.md`, which the audit inventory omits. If it
   contains prose contradicting the decision set or links to untracked files,
   correct it under clause 3 and 6. If it is superseded by `AGENTS.md` and
   `docs/work/`, delete it. State which, and why, in the report.
8. Add a row for `PROMPTS.md` to `docs/work/audit.md` section 1 with its
   classification. This is the only permitted edit to the audit.

## Out of scope
- Any change to code behaviour, including renaming an identifier, changing a
  string a test asserts on, or removing a dependency.
- Writing the final user-facing README. That is WP-0060.
- Writing the new report schema. That is WP-0008.
- Deleting phase documents. None is tracked; only links to them exist.
- Correcting `docs/work/` documents other than the one row in clause 8. The
  audit and the packages describe removed capabilities on purpose.
- Rebuilding the frontend bundle. This package's frontend edits are comments.

## Files
**May create or modify:** `README.md`, `CHANGELOG.md`, `PROMPTS.md`,
`docs/project-overview.md`, `docs/legacy/**`, `.goreleaser.yml`,
`internal/aggregate/schema_test.go`, `internal/server/securitymatrix_test.go`,
`docs/work/audit.md` (clause 8 only), and comment-only edits in the source files
listed in audit section 2.
**Must not touch:** `docs/decisions/**`, `docs/metrics.md`, `docs/conventions.md`,
`AGENTS.md`, `CLAUDE.md`, `internal/pipeline/interpret/taxonomy/**`, any other
file in `docs/work/`, and any source file in a way that changes behaviour.

## Steps
1. Read `docs/work/audit.md`, sections 1 and 2.
2. Move the schema, fix its `$id`, update the three references. Run the test
   suite and record the baseline result.
3. Rewrite `docs/project-overview.md`, `README.md`, `CHANGELOG.md`.
4. Remove dangling links from every file that carries one.
5. Handle `PROMPTS.md` and add its audit row.
6. Apply the prose corrections from clause 6, one file at a time, skipping every
   item under "The line this package does not cross".
7. Run the test suite again and confirm the result is identical to step 2.

## Definition of done
- Every relative link in every tracked Markdown file resolves, when resolved
  **relative to the file that contains it**.
- No prose in the tree describes a phase, a milestone, a prior plan task or exit
  criterion, or a document that is not tracked. Outside `docs/decisions/`,
  `docs/work/` and `docs/legacy/`, no prose describes a capability the decision
  set removes as though it were current.
- `docs/report-schema.json` does not exist; `docs/legacy/report-schema-v0.json`
  does, and its `$id` names the new path.
- `PROMPTS.md` is handled per clause 7 and appears in the audit inventory.
- `go test ./...` produces results identical to the baseline in step 2, file by
  file.
- `gofmt -l` prints nothing and `go vet ./...` passes.
- No identifier, flag name, JSON key, test name, assertion string, selector or
  dependency entry changed.

## Verification
```
# links resolved relative to their own file
git ls-files '*.md' | while read -r f; do
  grep -ohE '\]\([^)#][^)]*\)' "$f" | tr -d '()' | sed 's/^]//' |
  while read -r p; do [ -e "$(dirname "$f")/$p" ] || echo "DANGLING: $f -> $p"; done
done

# prose references, excluding the inventory and the packages that describe them
git grep -nE 'Phase [0-9]|phase-[0-9]|exit criterion|Task [0-9]' -- . \
  ':!docs/decisions' ':!docs/work' ':!docs/legacy'

test ! -e docs/report-schema.json && test -e docs/legacy/report-schema-v0.json
go test ./... && gofmt -l . && go vet ./...
git diff <baseline> -- . ':!*.md' | grep -E '^[+-]' | grep -vE '^[+-]{3}|report-schema'
# the last command shows only comment lines
```
