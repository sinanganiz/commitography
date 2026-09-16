# WP-0011: Git invocation chokepoint

**Area:** pipeline
**Implements:** ADR-0065, ADR-0044, ADR-0066, ADR-0041
**Requires:** WP-0005, WP-0006, WP-0007

## Goal
Every git invocation in the tree passes one package, carries the full hardening
set, reads NUL-delimited output, runs under a context with a timeout, terminates
its process group on cancellation, and reads output with a size limit.

## In scope
1. All git invocation passes `internal/git`. No other package invokes git,
   directly or indirectly (ADR-0065 clause 1).
2. Apply every hardening rule in ADR-0065 clause 2 on **every** invocation:
   - never through a shell;
   - environment sanitised: system configuration disabled, terminal prompting
     disabled, empty credential prompt helper, fixed C locale;
   - mandatory configuration flags before the subcommand: hooks disabled,
     external transport protocols disallowed, pager disabled, no submodule
     recursion;
   - user-derived arguments separated by `--`;
   - a context and a timeout;
   - NUL-delimited output formats;
   - output read with a size limit, never unbounded.
3. **The record format is NUL-delimited end to end**, and the parser is written
   against attacker-controlled input: file names and commit messages may contain
   newlines, quotes and control characters (ADR-0045). Line-based parsing is
   forbidden anywhere in this package.
4. Cancellation terminates the **process group**, not only the direct child, so
   a descendant is not orphaned (ADR-0044 clause 4).
5. Map git failures onto the WP-0006 error classes: a missing repository, a
   shallow clone and a missing credential are user errors with their reason
   codes; anything else is internal.
6. Tighten the lint rule to the ADR-0065 clause 3 set exactly, test files
   included, and add the file-level record reference to each permitted site.
7. Add the subprocess count checker (ADR-0063 table 2): the number of git
   processes must not grow proportionally to the number of commits. This is the
   check that catches a correct but pathological implementation.
8. Add a test asserting the sanitised environment and the mandatory flags are
   present on every invocation, by capturing the argument vector rather than by
   inspecting the code.

## Out of scope
- The collect stage's logic and its single-pass reading (WP-0012). This package
  provides the invocation surface; that package uses it.
- Cloning remotes and any credential handling (WP-0041, WP-0016).
- The browser opener, which ADR-0065 clause 3 leaves where it is.
- Removing the aggregation stage's remaining git calls. WP-0005 recorded them as
  a deviation naming WP-0013; they route through this package like everything
  else but are not relocated here.
- Parallel reading, which arrives with WP-0012 (ADR-0052 clause 1).

## Files
**May create or modify:** `internal/git/**`, `internal/**`, `cmd/**`,
`internal/checks/**`, linter configuration, `testdata/**` where a fixture is
needed.
**Must not touch:** `docs/decisions/**`, `docs/metrics.md`, `docs/legacy/**`,
`internal/pipeline/interpret/taxonomy/**`, `web/**`.

## Steps
1. Build the invocation surface with the hardening set applied centrally, so no
   caller can omit an item.
2. Convert callers one at a time, running the fast gate after each.
3. Replace line-based parsing with the NUL-delimited format and test it against
   the hostile-names fixture.
4. Add process-group termination and a cancellation test.
5. Map failures onto the error classes.
6. Tighten the lint rule and add the record references.
7. Add the subprocess count checker; observe it failing against a deliberate
   per-commit invocation, then revert.

## Definition of done
- Git is invoked from `internal/git` only, enforced by the lint rule including
  test files.
- The hostile-names fixture parses correctly: no record is split, merged or
  dropped.
- No invocation passes through a shell; the argument-vector test asserts the
  sanitised environment and mandatory flags on every call.
- Cancelling an analysis leaves no git process alive, verified by process
  inspection rather than by absence of error.
- The subprocess count does not grow proportionally to commit count on the large
  fixture.
- A shallow fixture produces a user error with its reason code and exit code 2.
- Every permitted non-git execution site carries a file-level ADR-0065
  reference.

## Verification
```
make gate-full
git grep -l '"os/exec"' -- internal cmd ':!internal/git' ':!internal/checks' \
  ':!*path_windows_test.go'        # only the browser opener
go test ./internal/git -run 'Hostile|Cancel|Environment'
go test ./internal/checks -run SubprocessCount
<binary> testdata/fixtures/shallow; echo $?      # 2
```
