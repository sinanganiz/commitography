# ADR-0016: Remote repositories are supported without storing credentials

**Status:** Accepted

## Context
Recurring re-analysis (ADR-0011) requires the server to update repositories
itself, which implies cloning and fetching remotes, which implies
authentication for private repositories. Storing tokens or keys in the
product's own database would recreate the credential custody risk that
ADR-0005 exists to avoid — with the operator's own secrets rather than a
stranger's, but with the same failure mode and no security team behind it.

## Decision
1. The product MUST support analysing repositories from a local path and from a
   remote URL.
2. The product MUST NOT store, encrypt, transmit or log repository credentials
   of any kind: passwords, personal access tokens, SSH private keys, or OAuth
   tokens.
3. There MUST NOT be a user interface field that accepts a repository
   credential.
4. Authentication for private remotes MUST be delegated to the host
   environment, using mechanisms git already provides: a mounted read-only
   deploy key, a forwarded SSH agent socket, or a configured git credential
   helper. The product invokes git and never handles the secret.
5. If a clone or fetch fails due to authentication, the product MUST report
   that the remote requires credentials the environment did not provide, and
   MUST NOT prompt for a secret.
6. Public mode MUST support unauthenticated remotes only (ADR-0029).

## Consequences
- The statement "Commitography stores no credentials" is true without
  qualification and requires no encryption key management.
- Configuring private remote access requires the operator to mount a key or
  forward an agent, which is documented deployment work rather than an
  in-product step.

## Out of scope
- In-product credential entry, credential vaults, key rotation.
- OAuth flows of any kind.

## Assumption
The target operator can mount a deploy key or forward an SSH agent into a
container.

## Acceptance criteria
- No schema field, configuration key, environment variable, or log line holds a
  repository credential.
- Analysing a private remote succeeds when a read-only deploy key is mounted,
  with no secret passing through the product.
- Authentication failure produces an explicit message and no prompt.

## Dependencies
ADR-0011 must be implemented before this one.
