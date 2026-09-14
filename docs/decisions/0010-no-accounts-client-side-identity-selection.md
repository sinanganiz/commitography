# ADR-0010: No accounts; identity selection happens in the client

**Status:** Accepted

## Context
The person lens requires knowing who the reader is. Authenticating that claim
would require an account system, session storage, and in public mode a user
table containing people who never signed up — reintroducing exactly the
custodial responsibility avoided in ADR-0005. The claim does not need to be
authenticated, because the consequence of a wrong claim is only that the reader
sees the wrong lens.

## Decision
1. The product MUST NOT contain user accounts, passwords, password recovery,
   email addresses, or third-party sign-in for the purpose of identifying a
   reader.
2. After an analysis completes, the reader MUST be able to select their own
   identity from the list of resolved contributors. The selection MUST be
   representable in the URL so that it survives a reload and can be
   bookmarked.
3. The reader MUST be able to select more than one contributor identity as
   being the same person, because one person commonly commits under several
   names or addresses in the same repository.
4. The product MUST propose candidate merges and MUST NOT apply them
   automatically. Candidates MUST be derived from at least: identical display
   name after normalisation, identical email local part, and the hosting
   provider `noreply` address pattern. The reader confirms; the system never
   decides.
5. Self-hosted operation MAY be protected by a single shared operator
   passphrase when bound to a non-loopback address. This is an access control
   for the deployment, not a user account, and MUST NOT be used to identify a
   reader.

## Consequences
- No credential, email address or personal record is stored for readers.
- Identity selection is a per-reader, per-repository choice, not a stored
  profile. Cross-repository persistence is addressed separately in ADR-0025.
- Merging identities changes the meaning of the work-type metrics, which is why
  ADR-0018 exists.

## Out of scope
- Sign-in with a hosting provider, even for identity only, is not implemented.
- Automatic global identity resolution is forbidden.

## Assumption
Selecting oneself from a list is acceptable friction, and is preferable to
frequently incorrect automatic matching.

## Acceptance criteria
- No user, account, session-owner or credential table exists in the schema.
- The identity selection, including a multi-identity selection, round-trips
  through the URL.
- Candidate merge suggestions are presented as suggestions and require explicit
  confirmation.

## Dependencies
ADR-0018 must be implemented before multi-identity selection can produce
correct work-type figures.
