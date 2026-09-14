# ADR-0015: The shareable artifact is a client-side image export

**Status:** Accepted

## Context
The primary deployment is self-hosted (ADR-0005), so most readers have no
public URL to share. The public instance does not pre-compute
(ADR-0005 clause 5), so its URLs do not exist until someone triggers an
analysis. A link is therefore not a viable shareable object for either
deployment.

## Decision
1. Any card in the Wrapped presentation MUST be exportable as a raster image.
2. Export MUST be performed in the browser from the same rendered output the
   reader sees. The server MUST NOT render images, and the container MUST NOT
   contain a headless browser for this purpose.
3. Export MUST work identically in self-hosted and public operation.
4. Exported images MUST NOT contain a raw email address, a local filesystem
   path, or a machine hostname.
5. Publishing a locally produced report to a project-operated service is
   **declared out of scope and MUST NOT be implemented**. It is recorded here
   so that the constraint in clause 6 is understood rather than because it is
   planned.
6. Because such publishing must remain possible without a redesign, the report
   artifact MUST NOT contain any raw email address, local filesystem path, or
   machine hostname, in any mode (see ADR-0033).

## Consequences
- Self-hosted readers can share output without exposing anything to a network.
- Sharing an image is higher friction than sharing a link. This is the
  acknowledged cost of ADR-0005 and is accepted.
- The container stays small because no browser engine is bundled.

## Out of scope
- Video and animated export.
- Report upload, hosted galleries, public profile pages.

## Assumption
Readers will export and share an image. This is the weakest assumption in the
distribution strategy and is accepted knowingly.

## Acceptance criteria
- Every Wrapped card exports to an image from the browser with no server round
  trip.
- The container image contains no browser engine.
- A generated report contains no raw email address, absolute local path, or
  hostname.

## Dependencies
ADR-0033 must be implemented before this one.
