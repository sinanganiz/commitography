# ADR-0002: The project is open source under the MIT license

**Status:** Accepted

## Context
The project's primary goal is reputation and demonstrated engineering capability
rather than revenue (see ADR-0003). In developer tooling, the source itself is
the artifact people evaluate, and open distribution channels — package
registries, code hosts, aggregators — are only available to open projects. A
closed tool of this kind has no distribution mechanism other than paid
promotion.

## Decision
1. The analysis engine, the server, the CLI and the web frontend MUST remain
   open source under the MIT license.
2. The license MUST NOT be changed to a copyleft or source-available license
   without a superseding ADR.
3. If a commercially licensed feature set is ever built, it MUST live in a
   separate repository and MUST NOT be required for any capability described in
   an existing ADR.
4. Third-party dependencies MUST be license-compatible with MIT redistribution.

## Consequences
- Anyone may run, fork, modify and commercially use the project.
- Relicensing after outside contributions accumulate is impractical, so this is
  effectively permanent.
- Any restriction the product imposes on its operator is advisory, because an
  operator can remove it in a fork. Restrictions that matter for safety must
  therefore be enforced where the project itself is the operator (see
  ADR-0029).

## Out of scope
- Contributor licence agreements are not required.
- Dual licensing is not adopted.

## Assumption
Reputation and adoption are worth more to this project than protection against
a third party hosting it commercially.

## Acceptance criteria
- `LICENSE` contains the MIT license text.
- Dependency licenses are compatible with MIT redistribution.

## Dependencies
None.
