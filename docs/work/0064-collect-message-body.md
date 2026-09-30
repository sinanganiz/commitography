# WP-0064: Collect records each commit's message body

**Area:** pipeline
**Implements:** ADR-0020, ADR-0072, ADR-0065, ADR-0045, ADR-0048, ADR-0033
**Requires:** WP-0012

## Goal
Every commit record carries the commit's message body as `docs/metrics.md`
section 1 defines it, read through length-framed output that no message can
forge, so that the messages and ai-archaeology families can apply their rules to
it. No report changes.

## In scope
1. **Add `Body` to `model.Commit`**, and increment `model.SchemaVersion` to 6
   with a line in its history comment. `Body` is the message after its first
   blank line (section 1), and is empty for a message with none.
2. **Read each commit's message from its commit object** through the git
   package's length-framed object reader (`git.OpenObjects`, ADR-0072), not by
   adding a message placeholder to the `git log` pretty format. A message may
   contain any byte, NUL and the field separator included, so no delimited
   format can frame it (ADR-0072, ADR-0045). The message is everything after
   the commit object's header, the first empty line of the object ending the
   header.
3. **Bound the body** by the single-file-size limit (ADR-0048 clause 1): a
   commit object over the limit is not read whole, its record's `Body` holds
   what the reader returned within the limit, and the record carries
   `BodyTruncated bool`. Nothing in this package reads `BodyTruncated`; the
   families that read the body decide what it means for them.
4. **Leave `Subject` as it is.** It remains what the `git log` header gives.
   Add the checker `TestCollectSubjectIsTheFirstLine`: on every fixture, every
   record's `Subject` equals the first line of the message read from the commit
   object. If it fails on any fixture, **stop and report**: the header's
   subject and the catalogue's definition then differ, and deciding which moves
   is a change to report values.
5. **Checkers**, each observed failing (ADR-0064):
   - `TestCollectBodyMatchesTheObject`: on every fixture, every record's `Body`
     equals the body parsed from `git cat-file commit` for that commit, run in
     the test through the git package.
   - `TestCollectBodyFramingIsUnforgeable`, beside the git package's existing
     hostile-input tests: a commit whose message contains a NUL byte, the
     collect stage's field separator, a line that imitates a commit header,
     and a body over the size limit is collected with every other record intact
     and with its own body as the object holds it, truncated where it is over
     the limit.
   - The count of git subprocess invocations during a full analysis still does
     not grow with the number of commits (ADR-0019 clause 5): the object reader
     is one process for the whole collection.
6. The collect artifact's writer and reader round-trip both new fields.

## Out of scope
- Classifying, matching or counting anything in a body. The messages family
  (WP-0020) and the ai-archaeology family (WP-0025) do that.
- Changing `Subject`, or any report value. No golden file changes.
- Storing a body anywhere but the collect artifact, which is the internal layer
  (ADR-0033 clause 1). No body reaches the report.

## Files
**May create or modify:** `internal/pipeline/collect/**`,
`internal/core/model/**`, `internal/git/**` **only to expose reading a commit
object through the existing length-framed reader, if it does not already**,
`internal/checks/**`, and `testdata/build-fixtures.sh`,
`testdata/fixture-conditions.txt` and `testdata/fixture-hashes.txt` **only if a
fixture is needed for clause 5's hostile message and no existing fixture
carries one**, together with that fixture's golden file.
**Must not touch:** `internal/metrics/**`, `internal/pipeline/replay/**`,
`internal/pipeline/aggregate/**`, `internal/core/` outside `model`, `cmd/**`,
`docs/decisions/**`, `docs/metrics.md`, `docs/report-schema.json`, and every
existing golden file.

## Steps
1. Write `TestCollectBodyMatchesTheObject` and see it fail on the current
   records, which carry no body.
2. Read commit objects through the length-framed reader and parse the body;
   add `Body`, `BodyTruncated` and the schema version.
3. Add `TestCollectSubjectIsTheFirstLine`; stop if it fails on any fixture.
4. Add the hostile-message test, and a fixture only if no fixture can carry
   one; add its golden file in the same commit (ADR-0019 clause 2).
5. Confirm the subprocess-count check still passes.

## Definition of done
- Every record of every fixture carries a `Body` equal to the one the commit
  object holds.
- A message containing NUL, the field separator or an imitation header leaves
  every other record intact.
- A body over the size limit is truncated to it and marked.
- Every record's `Subject` equals its message's first line on every fixture.
- `model.SchemaVersion` is 6, and the artifact round-trips both fields.
- No existing golden file changed.

## Verification
```
make gate-full
go test ./internal/checks -run '^TestCollect'
go test ./internal/pipeline/collect/... ./internal/git/...
git log --oneline --grep='^WP-0064' --diff-filter=M -- testdata/golden
# no output: no existing golden file was modified
```
