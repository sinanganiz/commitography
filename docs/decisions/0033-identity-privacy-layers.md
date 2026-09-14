# ADR-0033: Raw identities stay internal; exported artifacts carry no raw email

**Status:** Accepted

## Context
Email hashing was previously applied everywhere. Candidate identity matching
(ADR-0010, ADR-0025) needs readable identities, because similarity cannot be
derived from a hash. The original reason for hashing was never that the tool
should not know the address; it was that a shared artifact should not publish a
team's address list.

## Decision
1. The **internal working layer** — replay state, identity index, candidate
   matching, person records — MAY hold raw names and email addresses. This is
   data the tool already reads from the repository.
2. `report.json` MUST contain a display name and a stable identity digest. It
   MUST NOT contain a raw email address.
3. No exported artifact — report, image export, API response — may contain a
   raw email address, an absolute local filesystem path, or a machine hostname.
4. The identity digest MUST be stable across repositories for the same email
   address, so that cross-repository matching is possible without raw
   addresses leaving the internal layer.
5. Anonymised output MUST remain available as an explicit option that replaces
   display names with stable pseudonyms in addition to the rules above.
6. In public mode, person-scoped content MUST NOT have its own URL, MUST NOT be
   addressable, MUST NOT be indexable, and MUST NOT persist beyond the
   visitor's session. Repository-scoped content MAY be addressable.
7. The product MUST NOT store any person-scoped record in public mode
   (ADR-0025 clause 4).

## Consequences
- Candidate matching works, because the layer that performs it has raw data.
- Nothing that can leave the machine carries an address list.
- The constraint in clause 3 is what would make report publishing possible
  later without redesign, even though publishing is out of scope (ADR-0015).
- Public mode cannot accumulate profiles of people who never used the product,
  because there is no addressable person surface to accumulate into.

## Out of scope
- Encrypting the internal working layer at rest.
- Consent or takedown workflows, which are unnecessary because no person-scoped
  record is published.

## Assumption
A stable digest of an email address is sufficient for cross-repository
matching. Where a person uses different addresses, the candidate matching layer
handles it using raw data internally.

## Acceptance criteria
- Scanning a generated report, an exported image and every API response for an
  email pattern, an absolute path or a hostname yields no match.
- The same email address produces the same digest in two different
  repositories.
- In public mode, no route returns person-scoped content addressable by a
  stable identifier.

## Dependencies
ADR-0010 and ADR-0029 must be implemented before this one.
