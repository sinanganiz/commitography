# ADR-0021: `report.json` is the snapshot contract; history lives behind the API

**Status:** Accepted

## Context
The report was previously a product in its own right with a documented,
versioned schema that external tools could depend on. Server mode introduces
persistent storage and cross-version comparison, which is data that does not
belong to any single moment. Allowing the report to grow history would destroy
its clarity and make every comparison require loading large documents.

## Decision
1. `report.json` MUST describe exactly one repository, at one commit, under one
   analysis configuration. It MUST NOT contain data from any other analysis
   run.
2. `report.json` MUST remain a documented, versioned, stable contract
   (ADR-0031).
3. Time-series data, cross-version comparison and cross-repository data MUST be
   served by the HTTP API and MUST NOT be embedded in `report.json`.
4. The report MUST be reproducible offline: given the same repository, the same
   commit and the same analysis configuration, the produced report MUST be
   byte-identical apart from explicitly designated generation metadata fields.
5. A report produced by the CLI and a report produced by server mode for the
   same inputs MUST be identical. There MUST NOT be a server-only or CLI-only
   metric.
6. Generation metadata fields that legitimately vary — generation timestamp,
   tool version, host-neutral run identifiers — MUST be confined to a single
   designated section so that comparison tooling can exclude them.

## Consequences
- External tools can depend on the report without depending on the server.
- The golden regime in ADR-0019 is possible because output is deterministic.
- Comparison features require the API and are therefore unavailable in
  deployments that do not retain history.

## Out of scope
- Embedding previous analyses, trend series or other repositories in the
  report.
- Removing the report in favour of an API-only design.

## Assumption
External consumers want a snapshot. Consumers that want history call the API.

## Acceptance criteria
- Two runs over the same repository, commit and configuration produce reports
  that differ only within the designated metadata section.
- The CLI and the server produce identical reports for identical inputs.
- No time-series field exists in the report schema.

## Dependencies
None. ADR-0031 depends on this one.
