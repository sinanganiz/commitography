# WP-0008: Report document and schema versioning

**Area:** core
**Implements:** ADR-0021, ADR-0031, ADR-0032, ADR-0062, ADR-0010
**Requires:** WP-0005, WP-0006

## Goal
The report document defined by `docs/metrics.md` exists, carries a document
version and a version per family, contains every family in the catalogue with an
explicit status, confines generation metadata to one section, is reproducible,
and has a published schema.

## The golden invalidation

This package replaces the report document, so **every golden file changes at
once**. WP-0004 predicted this. It is the deliberate, explained diff the golden
regime exists to produce. The commit that regenerates them states that this
package replaced the document and why, per ADR-0019 clause 2. Regenerating them
without that statement is a defect.

## In scope
1. Define the report types in `internal/core` following `docs/metrics.md`, which
   is authoritative (ADR-0062). No field exists that the document does not
   define.
2. Add the document version with major and minor components, and a version for
   every family (ADR-0031 clauses 1 and 2).
3. **Every family in ADR-0024 clause 5 is present in every report**, with status
   `ok`, `skipped` or `degraded`, a machine-readable reason code from the
   WP-0006 enumeration, and a confidence indicator where the status is
   `degraded`.
4. Families the current code computes are `ok`. **Families that do not exist yet
   are `skipped` with reason `not_implemented`**, the same treatment
   `static-analysis` receives permanently (ADR-0032 clause 1). Each is filled in
   by its own package later. The report's shape is therefore correct from this
   package onward and never gains a top-level key again.
5. A `skipped` family's data fields are **empty, not null-filled and not
   zero-filled**, so that absence cannot be read as a measured zero.
6. Confine every value that varies between runs of the same commit — generation
   timestamp, tool version, run identifiers — to **one designated metadata
   section**, so comparison tooling excludes it by path (ADR-0021 clause 6,
   ADR-0061 clause 6).
7. Reserve the section that carries the resolved analysis configuration. WP-0010
   fills it; this package defines its place and its absence-is-an-error status.
7a. **Define the identities section, which the catalogue currently lacks.**
   ADR-0010 clause 2 requires the reader to select from a list of resolved
   contributors, and the report is the only artifact that can carry it. Add the
   section to `docs/metrics.md` and to the schema, then emit it.

   It is **not a metric family**. ADR-0024 clause 5 fixes the family set and this
   is not in it. It is a top-level reference section, beside the generation
   metadata and the resolved configuration.

   Each entry carries: the stable identity digest as its `id`, a display name,
   first and last commit dates, and the analysed commit count. The list is
   ordered by **first commit date ascending**, ties broken by `id`
   (ADR-0009 clause 3: no default ordering by output volume).

   The list is bounded as ADR-0018 clause 4 bounds the breakdown: the 200
   identities with the most analysed commits appear individually, and the
   remainder folds into **one** entry flagged as aggregate, whose display name
   states how many identities it represents.

   This package emits `id` and `display_name` using the digest function that
   already exists, so no raw address enters the report. The remaining identity
   fields, the resolution order, the merge suggestions and anonymisation arrive
   with WP-0009.
7b. Adding a top-level section and setting identity representation is a
   **document minor version increment** (ADR-0031 clause 1). Record it.
8. Write the new schema at `docs/report-schema.json` and validate every golden
   file against it in the full gate. The superseded schema stays in
   `docs/legacy/`.
9. Add the metric catalogue checker (ADR-0063 table 2): no metric, reason code
   or cardinality limit exists in the report that `docs/metrics.md` does not
   define.
10. Regenerate every golden file in one commit, with the statement required
    above.

## Out of scope
- Implementing any metric family. Families arrive with their own packages
  (WP-0018 onward); this package only gives them their place and their
  `not_implemented` status.
- The five pipeline stages (WP-0012 onward).
- Time series, cross-version and cross-repository data, which ADR-0021 clause 3
  puts behind the API, never in this document.
- Filling the analysis configuration section (WP-0010).
- Changing any part of `docs/metrics.md` other than adding the identities
  section. If the document and the intended types disagree elsewhere, the
  document is correct (ADR-0062 clause 3).
- Filling the identity fields beyond `id` and `display_name`, resolving
  identities through mailmap, producing merge suggestions, or implementing
  anonymisation. All of that is WP-0009.

## Files
**May create or modify:** `internal/core/**`, `internal/**`, `cmd/**`,
`docs/report-schema.json`, `docs/metrics.md` **for the new identities section
only**, `testdata/**` golden files, `internal/checks/**`.
**Must not touch:** `docs/decisions/**`, any existing section of
`docs/metrics.md`, `docs/legacy/**`,
`internal/pipeline/interpret/taxonomy/**`, `web/**`.

## Steps
1. Read `docs/metrics.md` in full. Build the type set from it, section by
   section.
2. Add document and family versions, and the status and reason fields.
3. Map the current computations onto the new document; mark every family the
   current code cannot produce as `skipped` with `not_implemented`.
4. Move every varying value into the metadata section, and reserve the
   configuration section.
5. Write the schema and wire validation into the full gate.
6. Add the metric catalogue checker; observe it failing on an invented field,
   then revert.
7. Regenerate the golden files in a single commit with the required statement.

## Definition of done
- Every family in ADR-0024 clause 5 appears in every report with a status, and
  the identities section appears beside them without being one of them.
- The identities list is ordered by first commit date ascending and is bounded
  at 200 individual entries plus one aggregate entry.
- No raw address appears in the identities section.
- No report contains a field, reason code or limit absent from
  `docs/metrics.md`; the checker fails when one is introduced.
- A `skipped` family's data fields are empty; a test distinguishes them from a
  measured zero.
- Two runs over one fixture differ only inside the metadata section.
- Every golden file validates against `docs/report-schema.json`.
- The golden regeneration commit states that this package replaced the document.
- `docs/legacy/report-schema-v0.json` is untouched.

## Verification
```
make gate-full
go test ./internal/checks -run 'Catalogue|Schema|Determinism'
jq -r 'keys[]' <report> | sort            # matches ADR-0024 clause 5 family set
jq '.families | map(select(.status=="skipped")) | length' <report>
git log -1 --format=%B <golden commit>    # states the document replacement
```
