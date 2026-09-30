# ADR-0079: Replay attributes each line to its authoring commit

**Status:** Accepted
**Note:** This record extends ADR-0051 and ADR-0074. It supersedes neither.

## Context
The ai-archaeology family asks what share of the surviving lines assisted
commits wrote, and whether the lines they wrote are rewritten sooner or survive
longer than other lines (`docs/metrics.md` section 9). Whether a commit is
assisted is a property of the commit, decided by the family from its rules
file.

The ownership map holds each line's owner and day (ADR-0051 clause 1). That
cannot tell two commits by one identity on one day apart, so no line can be
traced to the commit that wrote it. ADR-0051 clause 2 chose day resolution
because the recency window was the only consumer of a line's time. That remains
true of the time; the new consumer needs the commit, which is a different
datum.

## Decision
1. Each line of the ownership map MUST carry, beside its owner and day
   (ADR-0051 clause 1), its **authoring commit**, as an integer index into a
   table of commits the map holds. The table holds commit object names and no
   address (ADR-0051 clause 3, ADR-0033).
2. A line's authoring commit is the commit that gave the line its current
   owner: the commit whose addition or replacement wrote it, or the merge
   whose own line it is (ADR-0073 clause 5). A line that replay attributes to
   its commit because the version it came from could not be read
   (ADR-0072 clause 5) carries that commit.
3. Replay MUST record, for each authoring commit, the number of lines it wrote,
   and the ages at which its lines were replaced and at which they were
   deleted, as two histograms. An age is the one ADR-0074 clause 5 defines.
   The histograms count exactly the replacements and deletions ADR-0074
   clause 8 records, so their sum over every authoring commit equals the sum of
   the ADR-0074 histograms over every editor and owner.
4. These are inputs. Replay applies no recency window and no detection rule to
   them (ADR-0074 clause 9). The ai-archaeology family decides which commits
   are assisted and applies the window when it is aggregated.
5. Owner and day remain stored as ADR-0051 clause 1 requires. Neither is
   replaced by the commit.
6. The map's commit table holds exactly the commits its lines reference, in
   the order the lines first reference them, files ordered by path.
7. The budget `replay-bytes-per-line` counts the index with every line, and MAY
   rise by at most 4 bytes, the index's size, over its value before this
   record. The commit table is held once per referenced commit rather than per
   line, so it is not counted there: it is a budget of its own,
   `replay-commit-table-bytes`, measured on the same fixture, whose first value
   is the implementing package's measurement. Both are recorded citing this
   record (ADR-0054 clause 3), and any further rise of either needs its own
   record.

## Consequences
- The ai-archaeology family can attribute surviving lines, rewrites and
  deletions to assisted and unassisted commits.
- The ownership map, and the checkpoint that persists it (ADR-0017), grow by
  one index per line and a table of commit names.
- The map can say which commit wrote any surviving line, which a comparison
  with `git blame` (ADR-0019 clause 6) can also use.

## Out of scope
- Carrying commit names in the report. They remain replay state.
- Blame's copy and move detection, which replay still does not reproduce.

## Assumption
Four more bytes per line fit within the memory ceiling of ADR-0048 for the
repositories the product targets. The fixtures measure it; real repositories
have not yet been measured.

## Acceptance criteria
- Every line of the map carries an authoring commit in the analysed commit's
  ancestry.
- On a fixture, every line's authoring commit equals the one known by
  construction.
- The per-commit histograms, summed over every authoring commit, equal the
  ADR-0074 histograms summed over every editor and owner.
- `replay-bytes-per-line` rose by no more than 4.00, and
  `replay-commit-table-bytes` exists with a measured value.

## Dependencies
ADR-0051, ADR-0073, ADR-0074 and ADR-0076 must be implemented before this one.
