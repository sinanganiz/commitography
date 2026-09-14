# ADR-0043: Versioned REST with server-sent events for progress

**Status:** Accepted

## Context
ADR-0021 places time-series and cross-version data behind the API, so the API is
a contract in its own right, separate from the report document. Analysis is
asynchronous and needs a progress channel. ADR-0029 makes the available
capability set depend on the mode.

## Decision
1. The API MUST be resource-oriented over HTTP and MUST be versioned in the
   path. This version is distinct from the report document version (ADR-0031).
2. Job progress MUST be delivered over server-sent events. Cancellation MUST be
   a separate request, not a message on the stream.
3. **Capability discovery MUST be served by an endpoint.** The frontend MUST
   determine the enabled capability set from the server rather than assuming it,
   so that the ADR-0029 matrix is maintained in one place.
4. State-changing requests MUST use POST and MUST retain the session cookie and
   Origin checks.
5. A report MUST be retrievable whole and in sections, so that large reports do
   not require a single oversized response.
6. Endpoints MUST NOT expose data forbidden in the active mode, including
   through filtering or identifier enumeration. A capability disabled by mode
   MUST be absent, not merely empty.

## Consequences
- Responses are cacheable and readable.
- Progress works through ordinary HTTP infrastructure, with reconnection
  semantics provided by the transport.
- Mode restrictions have one source of truth, verified by the capability matrix
  test in ADR-0056.

## Out of scope
- GraphQL, a single RPC endpoint, WebSocket transport.

## Assumption
One frontend is the only consumer; the API is not a public integration surface.

## Acceptance criteria
- The API version appears in the path and is independent of the document
  version.
- Progress is observable over SSE and a job can be cancelled while streaming.
- Capability discovery reflects the active mode.
- Sectioned retrieval returns the same data as whole retrieval.

## Dependencies
ADR-0021, ADR-0029 and ADR-0031 must be implemented before this one.
