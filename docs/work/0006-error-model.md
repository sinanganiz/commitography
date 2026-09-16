# WP-0006: Error model and reason codes

**Area:** foundation
**Implements:** ADR-0041, ADR-0032, ADR-0062, ADR-0064
**Requires:** WP-0005

## Goal
Every error in the tree is one of two classes, every user-facing error carries a
reason code from the single enumerated set, exit codes and HTTP statuses are
produced by one mapping, no message or log line leaks an address, path or
hostname, and no code path reachable from repository content panics.

## In scope
1. Define two error classes in `internal/core`: **user error** and **internal
   error**. Classification is carried by the type, not by a string or a
   convention.
2. Define the reason code enumeration from `docs/metrics.md` section 13. That
   document is authoritative (ADR-0062 clause 6): a code that is not listed
   there does not exist. Add a checker that fails when a code is emitted that
   the document does not list, and when the document lists a code nothing can
   emit.
3. A user error MUST carry: its reason code, the offending value, and a remedy.
   A message naming neither the value nor a remedy is incomplete and fails
   review. Internal errors carry wrapping context sufficient to locate the
   origin.
4. Produce exit code and HTTP status from **one mapping over the class**. No
   call site may choose either. Grep must show a single construction site for
   each.
5. Keep the existing exit code values: `0` success, `1` internal, `2` user,
   configuration and repository validation including a refused shallow clone.
   Behaviour visible to a caller does not change in this package; only where the
   value comes from does.
6. Extend the leak scan checker to cover **log output**, not only report, API
   responses and exported artifacts (ADR-0063 table 2). Logs are the most
   commonly pasted artifact and the least inspected.
7. Add the top-level recovery layer in the server. A recovered panic is reported
   as an internal error, never as a user error and never as a success.
8. Add a test that runs the hostile-names fixture through the analysis path and
   asserts no panic and no leaked value.
9. Migrate existing error handling onto the two classes. Where a test asserts an
   error string, the assertion may be updated; the exit code it accompanies may
   not.

## Out of scope
- Localised or translated messages.
- Per-package error types.
- Changing any exit code value or HTTP status value.
- Changing what any test asserts other than an error message string.
- The reason codes families emit for `skipped` and `degraded` status, which
  WP-0008 wires into the report. This package defines the shared set; WP-0008
  uses it.

## Files
**May create or modify:** `internal/**`, `cmd/**`, `internal/checks/**`, linter
configuration.
**Must not touch:** `docs/decisions/**`, `docs/metrics.md`, `testdata/**`,
golden files, `internal/pipeline/interpret/taxonomy/**`, `web/**`.

## Steps
1. Define the two classes and the reason enumeration, sourced from
   `docs/metrics.md` section 13.
2. Add the enumeration checker in both directions and observe it failing before
   accepting it (ADR-0064 clause 6).
3. Add the single class-to-exit-code and class-to-status mappings.
4. Migrate call sites one package at a time, running the fast gate after each.
5. Extend the leak scan to logs; observe it failing on a deliberately leaked
   path, then revert.
6. Add the server recovery layer and the hostile-input panic test.

## Definition of done
- Every error crossing a package boundary is one of the two classes, enforced by
  the checker.
- Exit code and HTTP status each have exactly one construction site.
- No reason code is emitted that `docs/metrics.md` section 13 does not list, and
  no listed code is unreachable.
- The leak scan covers log output and passes.
- The hostile-names fixture produces no panic and no leaked address, path or
  hostname.
- Exit codes for the shallow and empty fixtures are unchanged from before this
  package.
- Every checker added here has been observed failing and then passing again;
  state the evidence.

## Verification
```
make gate-fast && make gate-full
git grep -n 'os.Exit(' -- cmd internal        # one site
go test ./internal/checks -run 'Leak|ReasonCode'
<binary> testdata/fixtures/shallow; echo $?   # 2, as before
<binary> testdata/fixtures/empty;   echo $?   # unchanged
```
