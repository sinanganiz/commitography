# ADR-0045: The repository is untrusted input in every mode

**Status:** Accepted

## Context
The product exists to analyse repositories the operator did not write. Commit
messages, file and reference names, in-repository git configuration and working
tree content are therefore attacker-controlled. In public mode an anonymous
visitor additionally supplies an arbitrary remote URL.

## Decision
1. The trust boundary MUST be:

   | Party | Trust |
   |---|---|
   | Repository content and history | Untrusted, in every mode |
   | Operator and operational configuration | Trusted |
   | Reader in `server` mode | Semi-trusted; unauthenticated, network exposure controlled by the operator |
   | Visitor in `public` mode | Untrusted |
   | Host environment and mounted secrets | Trusted |

2. Remote URLs supplied in public mode MUST be validated: only `https` and `git`
   schemes; the **resolved address** MUST be rejected if it falls in private,
   loopback, link-local or cloud metadata ranges; redirects MUST be re-validated.
3. The resolved address MUST be the one connected to, so that a name resolving
   differently between check and use cannot bypass validation.
4. Out of scope, explicitly: multi-tenant isolation, defence against the
   operator, side-channel attacks, and protecting an analysed repository's owner
   from a visitor.

## Consequences
- Parsing, path handling and subprocess invocation are written against hostile
  input rather than cooperative input.
- Public mode cannot be used to probe the internal network or a cloud metadata
  service.

## Out of scope
As stated in clause 4.

## Assumption
The deployment serves one operator. A multi-tenant deployment would require
revisiting every decision that rests on this.

## Acceptance criteria
- A URL resolving to a loopback, private, link-local or metadata address is
  refused in public mode.
- A redirect to such an address is refused.
- The trust table is reflected in the documented threat model.

## Dependencies
ADR-0029 must be implemented before this one.
