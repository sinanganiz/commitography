# ADR-0065: Git passes one chokepoint; other subprocesses are a closed set

**Status:** Accepted
**Supersedes:** ADR-0047

## Context
ADR-0047 forbade importing the process-execution library outside the git
package. Its purpose was to apply git hardening once rather than remember it at
each call site, but the prohibition was written wider than that purpose and
forbids three uses that have nothing to do with git: verification packages that
run the toolchain, the container runtime and the built binary; a Windows test
that creates a directory junction, which the standard library cannot do; and
opening a browser, which on two platforms means starting a process.

The intent survives. The blanket prohibition does not.

## Decision
1. **All git invocation MUST pass through a single package.** No other package
   may invoke git, directly or indirectly.
2. Every git invocation MUST obey the hardening rules, unchanged from ADR-0047:
   - never through a shell;
   - environment sanitised on every invocation: system configuration disabled,
     terminal prompting disabled, an empty credential prompt helper, a fixed C
     locale;
   - mandatory configuration flags before the subcommand: hooks disabled,
     external transport protocols disallowed, pager disabled, no submodule
     recursion;
   - user-derived arguments separated by `--`;
   - a context and a timeout on every invocation (ADR-0044);
   - **NUL-delimited output formats**, because paths and commit messages may
     contain newlines and are attacker-controlled;
   - output read with a size limit, never unbounded.
3. Non-git subprocess execution is permitted **only** at these sites, and this
   list is closed:
   - `internal/checks/**`, which exists to run the toolchain, the container
     runtime and the built binary;
   - the browser opener;
   - test setup that creates a platform construct the standard library cannot
     express.
4. Every permitted site MUST: use a fixed argument vector; never pass any value
   through a shell; carry a context and a timeout; and carry a file-level record
   reference naming this record (ADR-0059).
5. The browser opener MUST pass only a URL the product generated for itself,
   namely its own listen address. A URL from configuration, from a request, or
   from a repository MUST NOT reach it.
6. Adding a site to clause 3 requires a superseding record. Widening it in code,
   in a lint exception, or by relocating code into a permitted package is
   forbidden.
7. The lint rule enforcing this MUST allow the process-execution library only in
   the git package and the sites in clause 3, and MUST reject it everywhere
   else, including in tests.

## Consequences
- Git hardening is still applied in exactly one place.
- Verification tooling, which must start processes to verify anything, is no
  longer in conflict with the record it is meant to enforce.
- Clause 3 being closed means the exception cannot grow quietly; clause 6 makes
  growth visible.
- Clause 5 closes the one permitted site where an externally supplied value
  could otherwise reach a process.

## Out of scope
- Linking a git library instead of invoking the binary.
- Sandboxing subprocesses, which ADR-0046 clause 6 delegates to the container.

## Assumption
The three categories in clause 3 are the complete set of legitimate non-git
subprocess uses. A fourth requires a record, not an exception.

## Acceptance criteria
- A lint rule rejects process execution outside the git package and the clause 3
  sites, and the rule covers test files.
- A fixture containing newlines, quotes and control characters in file names and
  commit messages parses correctly.
- No invocation anywhere passes through a shell.
- The browser opener rejects any URL other than the product's own listen
  address.
- Every clause 3 site carries a file-level reference to this record.

## Dependencies
ADR-0020, ADR-0044 and ADR-0045 must be implemented before this one.
