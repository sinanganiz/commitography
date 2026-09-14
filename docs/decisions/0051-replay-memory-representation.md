# ADR-0051: Compact line ownership representation

**Status:** Accepted

## Context
The replay ownership map is the only structure whose memory grows with tracked
lines, so it determines the scale ceiling. A naive representation storing a
string owner and a full timestamp per line costs tens of bytes per line and adds
garbage collection pressure.

## Decision
1. Line ownership MUST be stored compactly: the owner as an **integer index into
   an identity table**, never a string; the authoring time as an integer with
   **day resolution**; lines held in contiguous per-file slices.
2. Day resolution is chosen because the only consumer is the work-type recency
   window in ADR-0020 clause 4, which is expressed in days. Finer resolution
   would cost space and change no metric.
3. Storing the owner as an index rather than a raw address has a second required
   effect: the checkpoint written to disk contains no raw email address,
   satisfying ADR-0033 clause 3 for persisted state.
4. Per-line memory MUST be measured against a fixture and recorded under
   ADR-0050 clause 3.
5. If the memory ceiling in ADR-0048 is approached, cold per-file maps MAY be
   spilled to disk. Spilling is a fallback, MUST NOT be the default path, and
   MUST NOT change the report.
6. Holding the ownership map entirely on disk MUST NOT be implemented; replay
   access is random within a chronological walk and would degrade full analysis.

## Consequences
- The scale ceiling rises by roughly an order of magnitude against a naive
  representation.
- Checkpoints are smaller and contain no raw identities.
- Day resolution is a deliberate limit: any future metric needing finer
  authoring resolution must take it from commit records, not from this map.

## Out of scope
- Disk-resident ownership maps as the default strategy.

## Assumption
Compact representation keeps per-line cost low enough for target repository
sizes. This must be measured, not assumed.

## Acceptance criteria
- Per-line memory cost is measured and within the recorded budget.
- A written checkpoint contains no raw email address.
- Spilling, when triggered, produces a report identical to the non-spilled path.

## Dependencies
ADR-0020, ADR-0033 and ADR-0048 must be implemented before this one.
