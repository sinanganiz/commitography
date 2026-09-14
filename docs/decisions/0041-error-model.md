# ADR-0041: Two error classes with enumerated user-facing reasons

**Status:** Accepted

## Context
ADR-0034 distinguishes exit code 2 for user, configuration and repository errors
from exit code 1 for internal errors. Without that distinction in the type
system, the correct exit code is guessed. ADR-0032 already requires enumerated
reason codes, so user-facing errors can draw from the same enumeration.

## Decision
1. Errors MUST be classified as **user error** or **internal error**.
2. A user error MUST carry an enumerated reason code from a documented set, and
   MUST state both what happened and what to do about it. A message naming
   neither the offending value nor a remedy is insufficient.
3. An internal error MUST carry wrapping context sufficient to locate its
   origin.
4. Exit code and HTTP status MUST be derived from the class, never chosen at the
   call site.
5. **No error message, and no log line, may contain a raw email address, an
   absolute local filesystem path, or a machine hostname** (ADR-0033 clause 3).
6. `panic` MUST be reserved for programmer error. No code path reachable from
   user input, repository content or network input may panic. The server MUST
   have a top-level recovery layer that reports a recovered panic as an internal
   error.
7. Reason codes for skipped and degraded metric families (ADR-0032) and reason
   codes for user errors MUST come from one enumeration, so that the same
   condition has one identity wherever it surfaces.

## Consequences
- Exit codes and statuses are consistent without per-site judgement.
- Wrapping a user error as an internal error becomes a reviewable type error
  rather than an invisible behaviour.
- Logs are safe to share, which matters because the most common support artifact
  is a pasted log.

## Out of scope
- Localised error messages.
- Per-package error types.

## Assumption
The reason code set stays finite and enumerable.

## Acceptance criteria
- Exit codes and HTTP statuses are produced from the error class by a single
  mapping.
- A scan of error messages and log output finds no email, absolute path or
  hostname.
- No panic is reachable from repository content in fuzz or fixture tests.

## Dependencies
ADR-0032, ADR-0033 and ADR-0034 must be implemented before this one.
