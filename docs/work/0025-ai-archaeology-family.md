# WP-0025: ai-archaeology family

**Area:** metrics
**Implements:** ADR-0076, ADR-0078, ADR-0079, ADR-0033, ADR-0062, ADR-0031, ADR-0032
**Requires:** WP-0018, WP-0023, WP-0062, WP-0063, WP-0064

## Goal
The `ai_archaeology` family recognises assisted commits by a versioned rules
file applied to commit bodies, computes every metric of `docs/metrics.md`
section 9, including the comparison of rework and survival between the lines
assisted and unassisted commits wrote, and carries person-and-year cells.

## In scope
1. **The rules file**, `internal/metrics/aiarchaeology/rules.yml`, embedded in
   the binary, with a `version` and a list of tools, each with an `id`, a list
   of co-author trailer patterns and a list of signature-line patterns.
   `detection_rules_version` echoes its `version`. A change to the file
   increments its version and the family's (ADR-0031 clause 2).
2. **Detection**, written into section 9:
   - a co-author trailer is a line of the body's last paragraph whose key,
     compared without regard to case, is `Co-authored-by`; it is recognised
     when its value matches a tool's trailer pattern;
   - a signature line is any line of the body matching a tool's signature
     pattern;
   - a commit is assisted when it carries a recognised trailer or signature
     line, and it counts once for every tool recognised in it.

   The first version of the file recognises the three tools the fixture
   `agent-coauthor-trailers` carries, by their trailer addresses, and must not
   recognise the fixture's person co-author. Recognising another tool, or a
   signature line, is a later rules change made with a fixture that carries
   it. The body is internal-layer data (ADR-0033 clause 1): no body, trailer
   or address reaches the report, only tool identifiers.
3. **A truncated body** (WP-0064) degrades the family with
   `limit_reached_size`, confidence `partial`, when any analysed commit's body
   was truncated, since its trailers may have been cut.
4. **Populations**, written into section 9. The assisted and unassisted
   populations are the analysed commits recognised and not recognised. The
   lines a population wrote are the `Written` lines of its commits in replay's
   per-commit authorship (WP-0062); lines written by commits that are not
   analysed belong to neither. `assisted_mean_lines` and
   `unassisted_mean_lines` are over the population's non-bulk commits that are
   not merges, as `commit_size` counts size.
5. **Compute every metric of section 9.** Write these definitions into it:
   - a population's **rework ratio** is the share of the lines it wrote that a
     counted commit replaced or deleted at an age less than
     `recency_window_days`, the window applied here and not in replay
     (ADR-0079 clause 4);
   - a population's **survival ratio** is the share of the lines it wrote that
     the ownership map holds at the analysed commit, each line traced to its
     authoring commit (ADR-0079);
   - `assisted_line_share` divides the map's lines whose authoring commit is
     assisted by every line of the map;
   - `tool_distribution` is a list of objects with `tool` and `commits`,
     ordered by `tool`;
   - `assisted_identity_ratio` is the share of the identity table's identities
     with at least one assisted commit.
6. **Cells** (ADR-0078 clause 3), per identity and year, folded through the
   identity table: for commit figures, of the analysed commit; for line
   figures, of the authoring commit. Fields: the cell identity and `year`;
   `commits`; `assisted`; `tools`, a count per tool; `first_assisted`, the
   earliest assisted commit's date or absent; for each population, its
   non-bulk non-merge commits and their effective lines; for each population,
   its lines written, rewritten within the window, and surviving; and
   `surviving`, every surviving line the cell's commits wrote. Summing adds
   every count and keeps the earlier `first_assisted`. Write into section 1's
   `Recency window` row that ai-archaeology is its second consumer.
7. **One derivation** (ADR-0078 clause 5), an exported pure function over a
   scope, filling the family's projection slot. `assisted_identity_ratio` and
   `detection_rules_version` are repository metrics; every other metric is
   scoped `person, year`, and its repository value is the derivation over every
   cell. Extend `core.Input.RestrictToYear` so that the per-commit authorship
   it returns holds that year's commits only.
8. **Status.** A zero assisted count is `ok` with zeros (section 9). Where
   replay holds no ownership map, the commit figures are computed, the line
   figures are absent, and the family is degraded with `history_incomplete`
   (WP-0023), confidence `partial`.
9. The method statement in the family's declaration states that detection is
   a heuristic over voluntarily emitted markers (section 9, ADR-0032 clause 8).
10. **Ratios carry 6 significant digits** through `core.RoundSignificant`; add
    it with a unit test if it is not there.
11. **Mark the scopes in section 9**: the `Scope` column; the row `cells`,
    marked `repository`; the `Cell field` table.
12. Add the metric and cell types to `internal/core/metrics.go` and
    `docs/report-schema.json`; the family leaves `not_implemented`, and its
    version becomes 1.0.
13. Regenerate every golden file in one commit whose body states that the
    family is computed for the first time.
14. Every test this package adds is named with the prefix `TestAIArchaeology`.
    One holds the fixture `agent-coauthor-trailers` to its assisted commits,
    tools and person co-author by construction.

## Out of scope
- Recognising tools the fixture does not carry, and signature lines.
- Inferring assistance from anything but the markers of clause 2.
- Any other family, and the interpretation axes (WP-0028).

## Files
**May create or modify:** `internal/metrics/aiarchaeology/**`,
`internal/core/metrics.go`, `internal/core/input.go`, `internal/core/stats.go`,
`internal/core/stats_test.go`, `internal/pipeline/aggregate/registry.go`,
`internal/checks/**` **only where a test names or calls the ai-archaeology family**,
`docs/metrics.md` **section 9, and section 1's `Recency window` row, only**,
`docs/report-schema.json`, `testdata/golden/**`.
**Must not touch:** every other package under `internal/metrics/`,
`internal/pipeline/collect/**`, `internal/pipeline/replay/**`, `cmd/**`,
`docs/decisions/**`, and every other section of `docs/metrics.md`.

## Steps
1. Write the rules file and the detection, with the fixture test.
2. Compute the commit figures, then the line figures from replay's authorship
   and map.
3. Build the cells and the derivation; extend `RestrictToYear`; fill the
   projection slot.
4. Edit sections 9 and 1, the core types and the schema.
5. Regenerate the golden files in one commit.

## Definition of done
- `internal/metrics/aiarchaeology/rules.yml` exists and its version appears in
  the report as `detection_rules_version`.
- On the fixture `agent-coauthor-trailers`, the assisted commits and their
  tools equal the fixture's construction, and the person co-author is
  unassisted.
- On the fixture `basic`, which carries no marker, the family is `ok` with a
  zero assisted count.
- `TestMetricCatalogueCells` and `TestProjectionEqualsRecomputation` pass, the
  latter listing `ai_archaeology`.
- No golden file contains an address from a trailer.
- The family's version is 1.0 and its status is computed.

## Verification
```
make gate-full
go test ./internal/metrics/aiarchaeology/...
go test ./internal/checks -run '^(TestMetricCatalogue|TestProjection|TestReportSchema|TestGolden|TestAIArchaeology)'
git grep -n -i 'anthropic.com\|cursor.com\|users.noreply.github.com' -- testdata/golden
# no output
```
