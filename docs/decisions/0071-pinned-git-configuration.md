# ADR-0071: Git configuration that affects output is pinned on every invocation

**Status:** Accepted
**Note:** This record extends ADR-0065 clause 2. It supersedes nothing.

## Context
ADR-0065 clause 2 disables git's system configuration on every invocation. The
operator's global configuration is still read, and several of its keys change
the output the analysis parses: the diff algorithm, rename detection and its
limit, path quoting, and the global attributes file among them. Two machines
analysing the same commit can therefore produce different reports, which
ADR-0021 clause 4 forbids.

Global configuration cannot simply be disabled as well. ADR-0016 clause 4
delegates authentication for private remotes to the host environment, and a
credential helper is most often configured globally.

## Decision
1. Every git configuration key and command-line option that can change output
   the analysis parses MUST be set explicitly on the invocation, so that it
   overrides any global or repository-local value.
2. This MUST be applied centrally in the git package (ADR-0065 clause 1), so
   that no caller can omit it, in the same way the hardening set is applied.
3. The pinned set MUST include at least: rename detection and its limit, the
   diff algorithm, path quoting, and the global attributes file. Any further key
   found to affect parsed output joins the set.
4. Keys that affect only authentication, transport or credentials are **not**
   pinned, so that ADR-0016's delegation keeps working.
5. A test MUST run an analysis under a deliberately hostile global
   configuration — every key in clause 3 set to a non-default value — and
   require a report byte-identical to one produced without it.

## Consequences
- The same commit produces the same report on any machine, whatever the
  operator's personal git setup.
- Credential delegation for private remotes is untouched.
- Adding a git invocation that parses output means checking it against the
  pinned set, which the test in clause 5 enforces.

## Out of scope
- Disabling global configuration wholesale, which would break ADR-0016.
- Repository-local configuration in the analysed repository, which ADR-0065
  clause 2 already neutralises through the mandatory flags and which is
  attacker-controlled content under ADR-0045.

## Assumption
Every output-affecting key can be overridden on the command line. A key that can
only be set in a file would need a different mechanism and its own decision.

## Acceptance criteria
- An analysis under a global configuration setting every clause 3 key to a
  non-default value produces a byte-identical report.
- The pinned set is applied in one place in the git package.
- Analysis of a private remote through a globally configured credential helper
  still succeeds.

## Dependencies
ADR-0021 and ADR-0065 must be implemented before this one.
