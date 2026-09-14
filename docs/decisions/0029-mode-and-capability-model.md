# ADR-0029: One mode switch selects a fixed capability set

**Status:** Accepted

## Context
The same image runs as a personal self-hosted tool and as a public instance
(ADR-0005). The two deployments must differ in what they are allowed to do.
Expressing each difference as its own flag creates a misconfiguration surface
where, for example, leaving person records enabled on the public instance would
mean retaining profiles of people who never used the product.

## Decision
1. There MUST be exactly one mode switch with exactly two values: `server`
   (default) and `public`.
2. Each mode MUST map to a fixed capability set. Individual capabilities MUST
   NOT be overridable by separate flags or by a policy file.
3. The capability matrix MUST be:

   | Capability | `server` | `public` |
   |---|---|---|
   | Analyse a local path | yes | no |
   | Clone an unauthenticated remote | yes | yes |
   | Clone an authenticated remote (ADR-0016) | yes | no |
   | Repository registry | yes | no |
   | Recurring re-analysis | yes | no |
   | Version history and time lens | yes | no |
   | Person records (ADR-0025) | yes | no |
   | Person lens | yes | yes, session-scoped only |
   | Repository Wrapped | yes | yes |
   | Person Wrapped | yes | yes, session-scoped only |
   | Language-model prose | yes | no |
   | Report cache | yes | yes |

4. A capability that is `no` for a mode MUST NOT be reachable by any route,
   parameter or configuration value in that mode.
5. Mode MUST NOT be a build-time distinction. One binary and one image MUST
   support both.
6. `server` mode MUST bind to a loopback address by default. Binding to a
   non-loopback address MUST emit a warning and MUST require the operator
   passphrase from ADR-0010 clause 5.

## Consequences
- Public-mode safety rests on one switch rather than on a set of defaults that
  can individually drift.
- No third mode exists; a reader wanting a single repository runs `server` with
  an empty registry.

## Out of scope
- Per-capability policy configuration.
- Separate builds per mode.

## Assumption
The two capability sets are far enough apart that no intermediate configuration
is needed.

## Acceptance criteria
- One binary serves both modes, selected by one value.
- In `public` mode every capability marked `no` returns not-found or
  not-available on every route, verified by test.
- Binding `server` to a non-loopback address without a passphrase is refused.

## Dependencies
ADR-0005 must be implemented before this one.
