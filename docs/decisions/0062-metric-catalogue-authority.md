# ADR-0062: `docs/metrics.md` is the authoritative metric catalogue

**Status:** Accepted

## Context
ADR-0024 fixes the metric families and their inputs, but not the metrics inside
them. `docs/metrics.md` defines those, and every other document refers to it —
yet nothing made it binding. The repository audit hit this directly: an existing
word-frequency metric could not be classified, because the document that omits
it is not a record.

## Decision
1. `docs/metrics.md` is the authoritative definition of every metric the product
   produces. A metric that is not defined there does not exist and MUST NOT be
   computed, stored in the report, or displayed.
2. Adding, removing or redefining a metric MUST change `docs/metrics.md` in the
   same change as the code, and MUST increment the owning family's version
   (ADR-0031 clause 2).
3. Where the document and the code disagree, the document is correct and the
   code is a defect.
4. Unlike a record, `docs/metrics.md` MAY be edited in place. Its history is the
   repository's history; it is not superseded the way a record is (ADR-0001
   clause 3).
5. Applying clause 1: the existing word-frequency metric and its stopword list
   are removed. `docs/metrics.md` section 4 defines no word metric, and a
   stopword list is language-specific, which would extend the English-bound
   limitation the message family already declares into a second metric.
6. The same authority applies to the reason code set in `docs/metrics.md`
   section 13 and the cardinality limits in section 12. A reason code or limit
   not listed there does not exist.

## Consequences
- "Is this metric in scope?" has a mechanical answer that does not require a new
  record for each metric.
- Records stay at the level of structure and guarantees; metric detail lives in
  a document that can be edited normally.
- The audit's undecided classification resolves to delete.

## Out of scope
- Making any other document binding. `docs/conventions.md` remains guidance
  (ADR-0058).
- Metric families themselves, which ADR-0024 fixes.

## Assumption
Metric definitions change often enough that superseding a record for each change
would be obstructive, and are detailed enough that recording them individually
would swell the decision set past usefulness.

## Acceptance criteria
- No metric is present in the report that `docs/metrics.md` does not define.
- No reason code is emitted that section 13 does not list.
- The word-frequency metric and its stopword list are absent from the tree.

## Dependencies
ADR-0024 and ADR-0031 must be implemented before this one.
