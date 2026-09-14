# ADR-0036: Embedded static SPA with server-injected meta tags

**Status:** Accepted

## Context
ADR-0022 requires a single binary whose only runtime dependency is `git`, which
rules out any frontend framework needing a JavaScript runtime in production.
The interface needs client-side lens transitions, force-directed layout and a
scroll-driven presentation, which rules out server-rendered templates.

## Decision
1. The frontend MUST be a statically built single-page application, embedded in
   the binary and served by the Go server.
2. The application MUST communicate with the Go HTTP API only.
3. A JavaScript runtime MUST NOT be required at runtime. Node is a build-time
   tool only.
4. The built bundle MUST be committed so that `go build` works without Node, and
   its integrity MUST be verified per ADR-0049.
5. The Go server MUST inject route-specific document metadata into the served
   HTML shell — title, description and social preview tags. This is metadata
   injection only; the server MUST NOT render application markup.
6. Injected metadata MUST obey ADR-0033: no raw email address, absolute local
   path or hostname, and in public mode no person-scoped content.

## Consequences
- Distribution through single-binary channels is preserved.
- Shared repository URLs in public mode carry a usable preview without
  server-side rendering.
- The build requires Node; the runtime does not.

## Out of scope
- Server-side rendering of application markup, streaming SSR, hydration.
- Any framework requiring a Node process in production.

## Assumption
A report can be delivered to the client in a form the application can hold. Very
large reports are handled by the sectioned retrieval required in ADR-0043.

## Acceptance criteria
- The container image contains no JavaScript runtime.
- `go build` succeeds with Node absent.
- Two different routes produce different injected metadata in the served HTML.

## Dependencies
ADR-0022 must be implemented before this one.
