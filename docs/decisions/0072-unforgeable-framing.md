# ADR-0072: Framing that content cannot forge

**Status:** Accepted
**Note:** This record narrows ADR-0065 clause 2. It supersedes nothing.

## Context
ADR-0065 clause 2 requires NUL-delimited output. Its purpose is that the
boundary between two records cannot be forged by what a record contains, because
paths and commit messages are attacker-controlled (ADR-0045).

Replay needs the contents of file versions. At the minimum supported git version,
git offers them only as line-framed patch output, which the rule rightly
forbids, or as a length-prefixed object stream, which the rule's wording forbids
although it meets the rule's purpose. The NUL-framed object form needs a much
newer git, and would still meet the per-record size cap on any large text file.

## Decision
1. The requirement is on **framing**: the boundary between two records MUST be
   impossible for record content to forge.
2. Two framings satisfy it:
   - **NUL-delimited**, for output whose attacker-controlled fields cannot carry
     a NUL;
   - **length-prefixed**, where a header states the byte length of the content
     that follows and **the header itself contains no attacker-controlled
     value**.
3. Line-framed output whose records carry attacker-controlled content remains
   forbidden. Patch output remains forbidden.
4. Within a length-framed record, file content MAY be split into lines, because
   no framing decision depends on that split.
5. A length-framed reader MUST stream and MUST cap each record at the
   single-file-size limit of ADR-0048. A record over the cap is skipped and marks
   its file `degraded`; it does not fail the run.
6. **A header that does not parse as expected aborts the read as an internal
   error.** The reader MUST NOT resynchronise by searching for the next
   plausible header, because resynchronising on a forged boundary is exactly the
   failure this record exists to prevent.

## Consequences
- File contents are readable at the currently supported git version, without
  raising it.
- The framing guarantee is stated at the width of its purpose and is, if
  anything, stronger: a length stated up front cannot be contradicted by
  content, whereas NUL framing depends on content not containing a NUL.
- Oversized text files degrade one file rather than one analysis. Most such
  files are lockfiles and generated output, which are excluded paths and never
  read.

## Out of scope
- Raising the minimum git version.
- Any reader that resynchronises, recovers or guesses after a malformed header.

## Assumption
Git's length-prefixed object headers contain only an object identifier, an
object type and a decimal size, none of which record content can influence.

## Acceptance criteria
- A reader test with a forged length header aborts with an internal error and
  reads nothing further.
- A blob larger than the cap marks its file degraded and the analysis completes.
- No patch output is parsed anywhere.

## Dependencies
ADR-0048 and ADR-0065 must be implemented before this one.
