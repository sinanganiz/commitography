# ADR-0047: One git chokepoint with hardened invocation and NUL-delimited output

**Status:** Superseded
**Superseded by:** ADR-0065

## Context
Git is invoked as a subprocess against untrusted repositories (ADR-0045). Two
separate hazards meet here: hostile invocation surfaces, and parsing output
whose fields may contain newlines, quotes and control characters because they
originate in attacker-controlled names and messages.

## Decision
1. **All git invocation MUST pass through a single package.** No other package
   may import the process execution library. This MUST be enforced by a lint
   rule.
2. Git MUST NEVER be invoked through a shell.
3. The environment MUST be sanitised on every invocation, at minimum: system
   configuration disabled, terminal prompting disabled, an empty credential
   prompt helper, and a fixed C locale for determinism and parsing safety.
4. Mandatory configuration flags MUST precede the subcommand on every
   invocation, at minimum: hooks disabled, external transport protocols
   disallowed, pager disabled, no submodule recursion.
5. User-derived arguments MUST be separated by `--`, so that reference and path
   names cannot be interpreted as options.
6. Every invocation MUST carry a context and a timeout (ADR-0044).
7. **Output formats MUST be NUL-delimited.** Line-based parsing of paths and
   commit messages MUST NOT be used, because both may contain newlines and both
   are attacker-controlled. This binds the record format used by the
   single-pass read in ADR-0020.
8. Subprocess output MUST be read with a size limit. Unbounded buffering MUST
   NOT be used.

## Consequences
- Hardening is applied once rather than remembered at each call site.
- A crafted file name or commit message cannot inject a synthetic record into
  the parser.
- The chokepoint is a natural place for the subprocess-count budget in
  ADR-0019.

## Out of scope
- Linking a git library instead of invoking the binary.

## Assumption
Every needed git capability is reachable through the binary with these flags.

## Acceptance criteria
- A lint rule rejects process execution outside the git package.
- A fixture containing newlines, quotes and control characters in file names and
  commit messages parses correctly.
- No invocation passes through a shell; sanitised environment and mandatory
  flags are asserted by test.

## Dependencies
ADR-0020, ADR-0044 and ADR-0045 must be implemented before this one.
