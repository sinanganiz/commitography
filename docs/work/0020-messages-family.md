# WP-0020: messages family

**Area:** metrics
**Implements:** ADR-0076, ADR-0078, ADR-0062, ADR-0031, ADR-0032
**Requires:** WP-0018, WP-0063, WP-0064

## Goal
The `messages` family computes every metric of `docs/metrics.md` section 4,
classifies free-form subjects by an ordered rules file rather than code, reads
subjects and bodies from the commit record, and carries person-and-year cells
for every scoped metric.

## In scope
1. **The rules file.** Move the keyword rules, the typo pattern and the
   work-in-progress pattern out of Go into `internal/metrics/messages/rules.yml`,
   embedded in the binary. Its keyword rules are the classes of section 4 **in
   section 4's order** — `revert`, `merge`, `fix`, `feat`, `docs`, `test`,
   `refactor`, `style`, `build`, `ci`, `chore`, then `other` — first match
   winning. The Go list today holds them in another order and has no `build`
   class; the file follows the catalogue. A change to the file changes values,
   so it increments the family version (ADR-0031 clause 2).
2. **Read section 1 values from the record**: `Subject`, `Body` and
   `BodyTruncated` (WP-0064), `ActiveDate`, and the year through
   `core.CommitYear`.
3. **The population** of every metric but one is the analysed commits.
   `merge_subject_ratio` is computed over every commit the analysis reads whose
   author is not excluded, merges included whether or not merges are counted,
   and is a repository metric only; write this into section 4.
4. **Compute every metric of section 4**, adding those the family lacks:
   `prefix_distribution`, `keyword_distribution`, `median_subject_length`,
   `body_ratio`, `very_short_count`, `longest_message`, `emoji_ratio`,
   `wip_ratio`, `issue_reference_ratio` and `merge_subject_ratio`. Write these
   definitions into section 4:
   - a length is a count of Unicode code points;
   - `longest_message` is the message with the most code points in its subject
     and body together, with that count, its hash and its date; ties by earlier
     date, then smaller hash;
   - an emoji is a code point in one of the Unicode blocks the section lists by
     range; list them;
   - an issue reference is a match of `#[0-9]+` or of
     `\b[A-Z][A-Z0-9]+-[0-9]+\b` in the subject or body;
   - `revert_count` counts subjects matching `^Revert "`, and subjects the
     keyword rules classify `revert`.
5. **A truncated body** (WP-0064) degrades the family with
   `limit_reached_size`, confidence `partial`, when any analysed commit's body
   was truncated: the values that read the body may undercount.
6. **Cells** (ADR-0078 clause 3), one per identity and year with an analysed
   commit, folded through the identity table: the cell identity and `year`;
   `commits`; `conventional`; `prefixes` and `keywords`, each a count per class;
   `subject_lengths`, the commits counted by subject length, ordered by length;
   `bodies`; `very_short`; `emoji`; `reverts`; `typo_fixes`; `wip`;
   `issue_references`; and `longest`, the cell's longest message as clause 4
   defines it. Summing adds every count and histogram and keeps the longer
   `longest`.
7. **One derivation** (ADR-0078 clause 5): an exported pure function over the
   sum of the cells a scope includes, filling the family's projection slot. The
   repository values of the scoped metrics are that function over every cell.
8. **Confidence.** The family is degraded with
   `low_classification_confidence` when the repository's `conventional_ratio`
   is below 0.30. It is decided on the repository scope; write this into
   section 4.
9. The language limitation of section 4 is carried by the family's method
   statement in its declaration (ADR-0032 clause 8).
10. **Ratios carry 6 significant digits** through `core.RoundSignificant`; add
    it with a unit test if it is not there. Mean lengths keep one decimal.
11. **Mark the scopes in section 4**: the `Scope` column, `person, year` on
    every metric row but `merge_subject_ratio`, which is `repository`; the row
    `cells`, marked `repository`; the `Cell field` table.
12. Add the metric and cell types to `internal/core/metrics.go` and
    `docs/report-schema.json`.
13. **Increment the family version to 2.0**: existing values change, through
    the rounding and the rules file.
14. Regenerate every golden file in one commit whose body names each cause.
15. Every test this package adds is named with the prefix `TestMessages`.
    Beside the unit tests, one test on the fixture `basic`, which carries
    conventional and free-form subjects, asserts every subject's class.

## Out of scope
- Any other family, and the assisted-commit markers in bodies (WP-0025).
- Classifying messages in languages other than English.
- Storing any message text in the report.

## Files
**May create or modify:** `internal/metrics/messages/**`,
`internal/core/metrics.go`, `internal/core/stats.go`,
`internal/core/stats_test.go`, `internal/pipeline/aggregate/registry.go`,
`internal/checks/**` **only where a test names or calls the messages family**,
`docs/metrics.md` **section 4 only**, `docs/report-schema.json`,
`testdata/golden/**`.
**Must not touch:** every other package under `internal/metrics/`,
`internal/pipeline/collect/**`, `internal/pipeline/replay/**`, `cmd/**`,
`docs/decisions/**`, and every other section of `docs/metrics.md`.

## Steps
1. Move the rules into the file in the catalogue's order and embed it.
2. Read subjects and bodies from the record; add the missing metrics.
3. Build the cells and the derivation; fill the projection slot.
4. Edit section 4, the core types and the schema.
5. Increment the version and regenerate the golden files in one commit.

## Definition of done
- `internal/metrics/messages/rules.yml` exists, holds the keyword classes in
  section 4's order, and no keyword pattern remains in Go source.
- On the fixture `basic`, every metric section 4 defines is present.
- `TestMetricCatalogueCells` and `TestProjectionEqualsRecomputation` pass, the
  latter listing `messages`.
- The family's version is 2.0.

## Verification
```
make gate-full
go test ./internal/metrics/messages/...
go test ./internal/checks -run '^(TestMetricCatalogue|TestProjection|TestReportSchema|TestGolden)'
git grep -n '(?i)' -- 'internal/metrics/messages/*.go' ':!*_test.go'
# no output: every case-insensitive pattern lives in rules.yml
```
