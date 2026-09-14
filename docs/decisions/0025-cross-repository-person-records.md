# ADR-0025: Cross-repository person records are data normalization, not accounts

**Status:** Accepted

## Context
Identity selection under ADR-0010 is per repository and lives in the client.
Trend comparison (ADR-0013) and any future cross-repository view require
knowing that an identity set in one repository and an identity set in another
belong to the same person. Recomputing that in the browser is not possible for
server-side aggregation.

## Decision
1. Server mode MUST support a **person record**: a named entity to which
   `(repository, identity set)` pairs are attached.
2. A person record is **not an account**. It MUST NOT carry a password, a
   session, an email address used for contact, a recovery mechanism, or any
   authentication capability. It is the cross-repository equivalent of
   `.mailmap`.
3. The product MUST propose candidate attachments using the matching signals in
   ADR-0010 clause 4 and MUST require explicit confirmation. It MUST NOT
   attach automatically.
4. Person records MUST NOT exist in public mode (ADR-0029). Creating one there
   would mean retaining cross-repository profiles of people who never used the
   product.
5. The data model MUST allow one person record to hold identity sets from
   several repositories, so that a cross-repository view remains implementable
   without a schema change.

## Consequences
- Trend and any future cross-repository aggregation have a server-side subject
  to attach to.
- ADR-0010's "no accounts" position is preserved, because nothing here
  authenticates anyone.
- Person records are operator-curated and will be filled in only for people the
  operator cares about, not for every contributor.

## Out of scope
- Authentication, authorisation or per-person access control.
- Automatic global identity resolution.

## Assumption
The operator is willing to curate person records manually for the identities
they care about.

## Acceptance criteria
- A person record can hold identity sets from at least two repositories.
- No person record field can hold a credential.
- In public mode, no route creates, reads or lists person records.

## Dependencies
ADR-0011 and ADR-0029 must be implemented before this one.
