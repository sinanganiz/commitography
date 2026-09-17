# ADR-0067: Paths in diagnostics, by destination

**Status:** Accepted
**Note:** This record narrows ADR-0041 clause 5 and states the scope of
ADR-0033 clause 3. It supersedes neither.

## Context
ADR-0041 clause 5 forbids an absolute local path in any error message or log
line. Its purpose was that shared artifacts do not reveal the machine they were
produced on. Written that wide, it also forbids a command telling the operator
which of their own paths was rejected, which ADR-0041 clause 2 requires by
demanding that a user error name the offending value.

Two conditions were found to collide with it directly: an error that embeds the
path it was given, and a command that prints its output directory. Both are
addressed to the person who typed the path.

## Decision
1. The rule is by **destination**, not by wording.
2. **Artifacts that can leave the machine** — the report, exported images, API
   responses, and any structured log written for collection — MUST NOT contain
   an absolute local filesystem path, a raw email address, or a machine
   hostname. There is no exception, and this is ADR-0033 clause 3 unchanged.
3. **Interactive diagnostics on standard error** MAY name a filesystem path,
   and only under both of these conditions:
   - the path is reproduced **exactly as the operator supplied it**, never
     resolved, absolutised or canonicalised for display;
   - it is a path the operator supplied in this invocation, never one discovered
     by traversal, read from a repository, or received in a request.
4. Interactive diagnostics MUST NOT contain a raw email address or a machine
   hostname under any circumstance.
5. Where a path must be resolved to be checked, the resolved form is used for
   the check and the supplied form is used for the message. A message MUST NOT
   contain both.
6. The leak scan checker MUST cover both destinations with the two different
   rules: clause 2 for artifacts, clauses 3 and 4 for diagnostics. A single
   pattern applied to both is wrong in one direction or the other.

## Consequences
- A command can say which path it rejected, which is the whole point of
  ADR-0041 clause 2.
- A pasted log still reveals nothing the operator did not type themselves.
- An API response cannot name a path at all, which is the case that was
  silently violated.
- Clause 5 removes the temptation to print the resolved path "for clarity",
  which is how an absolute path reaches a log while looking helpful.

## Out of scope
- Redacting a path the operator supplied. They typed it.
- Addresses and hostnames, which clause 4 forbids everywhere.

## Assumption
The supplied form of a path is always available at the point a message is
built. Where it is not, that is a plumbing defect, not a reason to print the
resolved form.

## Acceptance criteria
- No report, exported artifact or API response contains a path, address or
  hostname.
- A command rejecting a relative path names it relatively; given an absolute
  path, it names exactly what was typed and nothing longer.
- No diagnostic anywhere contains an address or a hostname.
- The leak scan applies two rules and fails when either is violated.

## Dependencies
ADR-0033 and ADR-0041 must be implemented before this one.
