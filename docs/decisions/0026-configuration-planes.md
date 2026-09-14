# ADR-0026: Configuration is split into an operational plane and an analysis plane

**Status:** Accepted

## Context
ADR-0021 requires that the same repository, commit and analysis configuration
always produce the same report, and that the CLI and the server agree. If the
server could influence analysis semantics through settings that are not part of
the report, two reports with the same name would contain different numbers.
Requiring the configuration to live only inside the repository is also
unworkable, because remote and read-only mounted repositories cannot be
edited.

## Decision
1. Configuration MUST be separated into two planes:
   - **Operational**: listen address, storage location, allowed roots,
     recurrence settings, person records, mode. This plane MUST NOT appear in
     the report and MUST NOT affect any metric value.
   - **Analysis**: exclusion lists, thresholds, mailmap usage, merge counting,
     date source, work-type recency window, anonymisation. This plane MUST
     affect metric values.
2. The analysis plane MAY be supplied from the repository file, from server
   settings, from a registered repository's settings, or from request
   parameters. Wherever it comes from, the **fully resolved, flattened analysis
   configuration MUST be embedded in the report**.
3. The report MUST contain the resolved result only. Intermediate layers and
   their precedence MUST NOT be represented in the report; they MAY be logged.
4. Taking the analysis configuration from a report and passing it to the CLI
   MUST reproduce that report (ADR-0021 clause 4).
5. The report cache key MUST include a normalized digest of the resolved
   analysis configuration (ADR-0017 clause 2).
6. Resolution order MUST be documented and MUST be: built-in defaults, then
   repository file, then explicitly supplied configuration which replaces the
   repository file rather than merging with it, then explicit parameters.
   Exclusion lists append to the built-in lists; setting a list to empty
   disables the built-in list.
7. Unknown configuration keys MUST produce a warning and MUST NOT fail the run.

## Consequences
- A report always carries enough information to be reproduced.
- Servers can offer configuration flexibility without becoming a source of
  hidden divergence.
- Changing an analysis setting produces a new cache entry rather than
  overwriting an existing one.

## Out of scope
- Per-metric-family configuration overrides beyond the analysis plane keys.

## Assumption
The analysis plane stays small enough to embed in every report.

## Acceptance criteria
- Extracting the embedded analysis configuration and passing it to the CLI
  reproduces the report byte-identically apart from generation metadata.
- No operational-plane key appears in the report.
- Two analyses differing only in an analysis-plane value produce two cache
  entries.

## Dependencies
ADR-0021 must be implemented before this one.
