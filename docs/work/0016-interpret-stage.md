# WP-0016: Interpret stage

**Area:** pipeline
**Implements:** ADR-0020, ADR-0014, ADR-0030, ADR-0077, ADR-0078, ADR-0070, ADR-0031, ADR-0032, ADR-0062
**Requires:** WP-0061, WP-0063

## Goal
The interpret stage exists as a pure function of a report, a scope and a
taxonomy; the report carries one interpretation section, the repository's, in
its final shape, reported `skipped` with `not_implemented` until archetype
evaluation exists (WP-0029); and the section is defined in the catalogue and
held to it.

## In scope
1. **The function** (ADR-0077 clauses 2 and 3, ADR-0078 clause 11). In
   `internal/pipeline/interpret`, export one function taking a stored report, a
   `core.Scope` and a taxonomy, and returning an interpretation. It reads
   nothing else: no file, no clock, no process, no environment, no git. It
   validates the scope against the report's identities section: every `id` it
   names is an individual entry, and the aggregate entry is never named (ADR-0078
   clause 9). Load the taxonomy from `internal/pipeline/interpret/taxonomy/taxonomy.yml`,
   embedded in the binary, with its `version`.
2. **Until WP-0029**, the function returns, for every valid scope, the
   interpretation `skipped` with the reason `not_implemented`, and no archetype
   and no badge. It never returns the fallback archetype: nothing was evaluated,
   so assigning one would be a claim the stage cannot make (ADR-0030 clause 4).
3. **The interpretation section.** Add the top-level report section
   `interpretation`, the interpretation of the scope of every identity and
   every year (ADR-0077 clause 1), in the shape it will keep:
   - `status`, `ok` or `skipped`, and `reasons` on a skipped section, from the
     family status codes of section 13;
   - `taxonomy_version`, the taxonomy's version, present where the taxonomy was
     evaluated (ADR-0030 clause 6);
   - `archetype`, present where evaluated: `id`, and `conditions`, the
     conditions that matched, each with `axis`, `operator`, `threshold` and
     `value` (ADR-0030 clause 4);
   - `badges`, present where evaluated: each awarded or skipped badge in
     taxonomy order, with `id`, `status` (`awarded` or `skipped`), `conditions`
     on an awarded badge and `reasons` on a skipped one. A badge that was
     evaluated and not awarded is absent; a skipped badge is never dropped
     (`internal/pipeline/interpret/taxonomy/axes.md`, ADR-0032).

   The section names no identity and holds no scope: the report carries no
   person's interpretation (ADR-0077 clause 1). Names and descriptions are the
   taxonomy's translatable layer and are not copied into the report
   (ADR-0030 clause 5).
4. **Define the section in `docs/metrics.md`** as a new section 15,
   `The interpretation section`, as section 14 defines the identities: a
   table headed `Field`, the status rule, and that it is not a metric family.
   In section 13, extend the sentence introducing the family status codes to
   say that the interpretation section carries them too.
5. **Versions.** Add `interpretation` to the report's `sections` object at 1.0
   (ADR-0070 clause 1), and increment the document's minor version to 2.1: a
   top-level key is added and nothing is removed (ADR-0031 clause 1). Record
   both in the version history comments.
6. **Run the stage in the pipeline**, after aggregation, over the report
   aggregation produced, with the scope of every identity and every year. Both
   `Analyzer.Run` and `Analyzer.Aggregate` return the report with the section
   filled, so that `TestAggregateWithoutTheRepository` still compares like with
   like. The aggregate stage itself leaves the section empty.
7. **Checkers**, each observed failing (ADR-0064):
   - `TestInterpretIsRerunnable` (ADR-0020 clause 6): for every fixture, the
     golden report decoded from JSON and interpreted again yields the section
     the golden report holds.
   - `TestInterpretReportCarriesNoPerson` (ADR-0077 acceptance): the section's
     Go type, walked recursively, holds no field for an identity, an `id` or a
     scope.
   - `TestInterpretMergedSelection` (ADR-0077 acceptance, ADR-0078 clause 7):
     on the fixture `basic`, the interpretation for a two-identity selection
     equals the interpretation for the merged identity of an analysis whose
     configuration merged the two. Demonstrate it failing against a scratch
     function in the test file that returns a value depending on the number of
     identities selected.
   - `TestMetricCatalogueInterpretation`: the section's fields and those
     section 15 defines are one set, in both directions, as
     `TestMetricCatalogueIdentities` holds section 14.
8. Add the section to `docs/report-schema.json`, and regenerate every golden
   file in one commit whose body states the new section, its version and the
   document version.
9. Every test this package adds is named with the prefix `TestInterpret` or
   `TestMetricCatalogueInterpretation`.

## Out of scope
- Computing axes (WP-0028) and evaluating archetypes and badges (WP-0029).
- Language-model prose (WP-0031).
- Any API route or cache for a person's interpretation (ADR-0077 clause 5,
  the server packages).
- Changing `taxonomy.yml` or `axes.md`.

## Files
**May create or modify:** `internal/pipeline/interpret/**` **except
`internal/pipeline/interpret/taxonomy/*`**, `internal/pipeline/run.go`,
`internal/pipeline/*_test.go`, `internal/core/**`, `internal/checks/**`,
`docs/metrics.md` **section 13's introducing sentence and the new section 15
only**, `docs/report-schema.json`, `testdata/golden/**`, and
`internal/server/*_test.go` **only for assertions on the document version or
the report's top-level keys**.
**Must not touch:** `internal/pipeline/interpret/taxonomy/*`,
`internal/metrics/**`, `internal/pipeline/aggregate/**`,
`internal/pipeline/collect/**`, `internal/pipeline/replay/**`, `cmd/**`,
`docs/decisions/**`.

## Steps
1. Write `TestMetricCatalogueInterpretation` and section 15; see the checker
   fail until the report carries the section.
2. Add the section's types, the section version and the document version.
3. Write the function and the embedded taxonomy loading.
4. Run the stage from the pipeline root in both paths.
5. Add the remaining checkers.
6. Update the schema and regenerate the golden files in one commit.

## Definition of done
- `internal/pipeline/interpret` exports one function of a report, a scope and a
  taxonomy, and its tests hold it to reading nothing else.
- Every golden report carries `interpretation` with status `skipped` and reason
  `not_implemented`, and no archetype.
- `sections.interpretation` is 1.0 and the document version is 2.1.
- `docs/metrics.md` section 15 exists, and
  `TestMetricCatalogueInterpretation` passes.
- `TestInterpretIsRerunnable`, `TestInterpretReportCarriesNoPerson` and
  `TestInterpretMergedSelection` pass and were observed failing.
- `TestAggregateWithoutTheRepository` passes.

## Verification
```
make gate-full
go test ./internal/pipeline/interpret/...
go test ./internal/checks -run '^(TestInterpret|TestMetricCatalogue|TestAggregate|TestReportSchema|TestGolden)'
```
