# ADR-0068: The embedded configuration carries identity references, not identities

**Status:** Accepted

## Context
Three accepted records meet at one point and cannot all hold as written.
ADR-0026 clause 2 requires the resolved analysis configuration to be embedded in
the report, and clause 4 requires that passing it back reproduces the report.
ADR-0033 clause 2 forbids a raw address anywhere in the report. Two analysis
settings carry addresses: the identity merge list, and the author exclusion
list, which matches against addresses as well as names.

Omitting those settings breaks reproduction. Embedding them breaks the privacy
rule the leak scan enforces.

## Decision
1. Every value in the embedded analysis configuration that identifies a person
   is stored as a **reference**, never as a raw address.
2. An address is stored as its identity digest, computed by the same function
   ADR-0033 clause 4 defines, so a value is identical wherever it appears.
3. **The configuration loader MUST accept a digest wherever it accepts an
   address.** A supplied address is digested before matching; a supplied digest
   is matched directly. Both produce the same analysis.
4. Display names are stored literally, **except under anonymisation**, where an
   operator-supplied display name is replaced by the stable pseudonym the report
   uses for that identity, and the loader accepts a pseudonym wherever it
   accepts a display name. Without this, a configuration section would reveal
   the names anonymisation exists to hide.
5. A pattern that matches a class rather than a person — a bot name suffix, for
   example — is stored literally, because it identifies nobody.
6. The round trip required by ADR-0026 clause 4 holds under both
   representations and with anonymisation on or off.
7. The leak scan covers the configuration section exactly as it covers the rest
   of the report. The section is not an exception to ADR-0033 clause 2; this
   record is how it complies.

## Consequences
- A published report's configuration section can be handed to anyone and
  reused without revealing a team's addresses, which is ADR-0033's purpose
  applied to the one part of the report that still carried them.
- A configuration file written by hand with addresses and the same file taken
  from a report with digests produce the same analysis, so the operator never
  has to work with digests unless they want to.
- Matching is exact matching on a normalised value, in both representations.

## Out of scope
- Reversing a digest, or any form of lookup from digest to address.
- Substring, prefix or pattern matching against an address. A matcher that
  needs it cannot use a digest and requires its own decision.

## Assumption
Identity merging and author exclusion match addresses exactly after
normalisation. This is what the current matchers do; a matcher that stops doing
it invalidates this record.

## Acceptance criteria
- No raw address appears in any report's configuration section, verified by the
  leak scan.
- A configuration file naming addresses and one naming the corresponding
  digests produce byte-identical reports.
- With anonymisation on, the configuration section contains no display name
  that does not also appear in the report's identities section.
- Extracting the configuration section and passing it back reproduces the
  report, with anonymisation on and off.

## Dependencies
ADR-0026 and ADR-0033 must be implemented before this one.
