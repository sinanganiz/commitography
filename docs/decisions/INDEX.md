# Architecture Decision Index

This file lists every architecture decision record in this directory. It is
**required reading** before making any change to this repository.

Rules for working with these records are defined in
[ADR-0001](0001-record-decisions-as-adrs.md). In short:

- One decision per file. Accepted records are never edited to change meaning;
  they are superseded by a new record.
- Records contain no dates, durations, schedules or phase names. Ordering is
  expressed only as implementation dependency.
- Requirements are written as numbers or prohibitions, never as adjectives.

`docs/decisions/` is **binding**; a violation is a defect.
`docs/conventions.md` is **guidance**; a justified deviation is acceptable.

If a proposed change contradicts an accepted record, the change is wrong unless
a superseding record is written first.

---

## Records

| # | Title | Status |
|---|---|---|
| [0001](0001-record-decisions-as-adrs.md) | Record architecture decisions as ADRs | Accepted |
| [0002](0002-open-source-mit.md) | The project is open source under the MIT license | Accepted |
| [0003](0003-monetization-is-not-a-goal.md) | Monetization is not a goal | Accepted |
| [0004](0004-no-phases-or-schedule.md) | Scope is complete and unphased; no schedules are recorded | Accepted |
| [0005](0005-hybrid-distribution.md) | Hybrid distribution — self-hosted primary, public instance secondary | Accepted |
| [0006](0006-repository-first-person-as-lens.md) | The repository is the unit of analysis; the person is a lens | Accepted |
| [0007](0007-local-clone-is-the-only-data-source.md) | The local git clone is the only data source | Accepted |
| [0008](0008-wrapped-is-a-mode.md) | Wrapped is a mode of the product, not the product | Accepted |
| [0009](0009-individual-metric-visibility.md) | Individual metrics are self-first and mode-dependent | Accepted |
| [0010](0010-no-accounts-client-side-identity-selection.md) | No accounts; identity selection happens in the client | Accepted |
| [0011](0011-server-mode-with-repository-registry.md) | Server mode with a persistent repository registry | Accepted |
| [0012](0012-metric-scope.md) | Metric scope includes static analysis as a non-priority capability | Accepted |
| [0013](0013-three-comparison-axes.md) | Comparison uses three axes and no global corpus | Accepted |
| [0014](0014-archetype-plus-badges.md) | Interpretation is one archetype plus a bounded set of badges | Accepted |
| [0015](0015-shareable-artifact-is-a-client-side-image.md) | The shareable artifact is a client-side image export | Accepted |
| [0016](0016-remote-repositories-without-stored-credentials.md) | Remote repositories are supported without storing credentials | Accepted |
| [0017](0017-persistence-model.md) | Persistence stores reports, normalized commit records and an incremental checkpoint | Accepted |
| [0018](0018-dual-breakdown-work-type.md) | Work-type data is stored as a dual breakdown by editor and prior owner | Accepted |
| [0019](0019-verification-regime.md) | Golden fixtures, invariants and a performance budget | Accepted |
| [0020](0020-five-stage-pipeline.md) | The pipeline has five stages | Accepted |
| [0021](0021-report-json-is-the-snapshot-contract.md) | `report.json` is the snapshot contract; history lives behind the API | Accepted |
| [0022](0022-no-required-external-services.md) | The product never requires an external service | Accepted |
| [0023](0023-wrapped-subjects.md) | Wrapped exists for two subjects — the repository and the person within it | Accepted |
| [0024](0024-metric-family-interface.md) | Metric families declare their inputs and never read each other | Superseded by ADR-0076 |
| [0025](0025-cross-repository-person-records.md) | Cross-repository person records are data normalization, not accounts | Accepted |
| [0026](0026-configuration-planes.md) | Configuration is split into an operational plane and an analysis plane | Accepted |
| [0027](0027-concurrency-model.md) | Two job classes, interactive priority, and one active job per repository | Accepted |
| [0028](0028-information-architecture.md) | Three levels, and lenses are states rather than pages | Accepted |
| [0029](0029-mode-and-capability-model.md) | One mode switch selects a fixed capability set | Accepted |
| [0030](0030-archetype-taxonomy.md) | Archetypes are assigned by an ordered rule list over absolute thresholds | Accepted |
| [0031](0031-schema-versioning.md) | Document-level and family-level versioning | Accepted |
| [0032](0032-family-status-reporting.md) | Every family reports a status; absence is never silent | Accepted |
| [0033](0033-identity-privacy-layers.md) | Raw identities stay internal; exported artifacts carry no raw email | Accepted |
| [0034](0034-cli-output.md) | The CLI emits `report.json` only | Accepted |
| [0035](0035-embedded-storage.md) | Embedded SQLite for metadata, files for blobs | Accepted |
| [0036](0036-frontend-delivery.md) | Embedded static SPA with server-injected meta tags | Accepted |
| [0037](0037-visualization-approach.md) | Layout mathematics from libraries, DOM ownership by React alone | Accepted |
| [0038](0038-styling-and-design-tokens.md) | Token-enforced styling with headless primitives | Accepted |
| [0039](0039-llm-integration-surface.md) | One compatible HTTP interface, and full function without it | Accepted |
| [0040](0040-package-boundaries.md) | Package boundaries follow pipeline stages, with a one-way dependency rule | Accepted |
| [0041](0041-error-model.md) | Two error classes with enumerated user-facing reasons | Accepted |
| [0042](0042-dependency-wiring.md) | Explicit constructor injection and no ambient state | Accepted |
| [0043](0043-http-api-shape.md) | Versioned REST with server-sent events for progress | Accepted |
| [0044](0044-concurrency-primitives.md) | Owned goroutines, context-bound subprocesses, persistent locks | Accepted |
| [0045](0045-threat-model.md) | The repository is untrusted input in every mode | Accepted |
| [0046](0046-filesystem-boundary.md) | Canonical path containment and product-controlled clone targets | Accepted |
| [0047](0047-subprocess-hardening.md) | One git chokepoint with hardened invocation and NUL-delimited output | Superseded by ADR-0065 |
| [0048](0048-resource-limits.md) | Enforced limits, and truncation is never silent | Accepted |
| [0049](0049-supply-chain.md) | Dependency budget, reproducible builds and bundle verification | Accepted |
| [0050](0050-performance-budget-expression.md) | Duration budgets are ratios, memory budgets are absolute | Accepted |
| [0051](0051-replay-memory-representation.md) | Compact line ownership representation | Accepted |
| [0052](0052-parallelism-placement.md) | Collect is parallel, replay is sequential, aggregate is parallel by family | Accepted |
| [0053](0053-frontend-performance-and-cardinality.md) | Bundle budget and report-side cardinality limits | Accepted |
| [0054](0054-budget-violations-are-gates.md) | Budgets are gates, and loosening one requires a record | Accepted |
| [0055](0055-enforcement-strategy.md) | Rules are enforced by tooling, and checker messages name the record | Accepted |
| [0056](0056-enforced-rule-set.md) | The enforced rule set | Superseded by ADR-0063 |
| [0057](0057-ci-gate-structure.md) | Two gates and a release path, with duration budgets | Accepted |
| [0058](0058-conventions-document.md) | Non-enforceable patterns live in a guidance document | Accepted |
| [0059](0059-decision-to-code-binding.md) | Constraint-bearing code carries record references | Accepted |
| [0060](0060-package-destinations.md) | Destinations for packages the layout does not name | Accepted |
| [0061](0061-link-time-build-metadata.md) | Link-time build metadata is the one exception to the no-package-variable rule | Accepted |
| [0062](0062-metric-catalogue-authority.md) | `docs/metrics.md` is the authoritative metric catalogue | Accepted |
| [0063](0063-enforced-rule-set-corrected.md) | The enforced rule set | Accepted |
| [0064](0064-checks-must-be-able-to-fail.md) | A check that cannot fail is not a check | Accepted |
| [0065](0065-subprocess-execution.md) | Git passes one chokepoint; other subprocesses are a closed set | Accepted |
| [0066](0066-layout-gaps.md) | The git package's position, and subpackages under core | Accepted |
| [0067](0067-diagnostics-versus-artifacts.md) | Paths in diagnostics, by destination | Accepted |
| [0068](0068-configuration-identity-references.md) | The embedded configuration carries identity references, not identities | Accepted |
| [0069](0069-candidate-evidence-from-history.md) | Merge candidates are evidence from the analysed history | Accepted |
| [0070](0070-section-versions.md) | Top-level report sections carry their own version | Accepted |
| [0071](0071-pinned-git-configuration.md) | Git configuration that affects output is pinned on every invocation | Accepted |
| [0072](0072-unforgeable-framing.md) | Framing that content cannot forge | Accepted |
| [0073](0073-replay-follows-the-graph.md) | Replay follows the commit graph | Accepted |
| [0074](0074-worktype-unit-and-window.md) | What work-type classification counts, and where its window applies | Accepted |
| [0075](0075-files-family-inputs.md) | The files family declares replay state as well as commit records | Superseded by ADR-0076 |
| [0076](0076-family-contract-three-inputs.md) | Metric families declare three input kinds, and none is the working tree | Accepted |
| [0077](0077-person-interpretation-at-view-time.md) | Person interpretation is computed at view time | Accepted |

ADR-0024 and ADR-0075 are superseded by ADR-0076. ADR-0047 is superseded by ADR-0065. ADR-0056 is superseded by ADR-0063.

---

## Dependency tiers

Derived from the `Dependencies` sections. This is an ordering constraint, not a
schedule (ADR-0004). Records in the same tier have no dependency on each other.

| Tier | Records |
|---|---|
| 1 | 0001, 0002, 0003, 0005, 0006, 0010, 0013, 0019, 0020, 0021, 0022 |
| 2 | 0004, 0007, 0008, 0018, 0026, 0028, 0029, 0030, 0031, 0034, 0036, 0042, 0055 |
| 3 | 0014, 0017, 0024, 0033, 0037, 0038, 0043, 0045, 0049, 0056, 0058, 0059, 0063 |
| 4 | 0009, 0011, 0012, 0015, 0027, 0032, 0035, 0040, 0046, 0052, 0057, 0061, 0062 |
| 5 | 0016, 0025, 0039, 0041, 0044, 0048, 0050, 0053, 0060, 0064 |
| 6 | 0023, 0047, 0051, 0054, 0065, 0066, 0067, 0068, 0070, 0071, 0072 |
| 7 | 0069, 0073 |
| 8 | 0074, 0075, 0076, 0077 |

Two records carry partial dependencies stated in prose rather than as a
whole-record dependency, and are therefore placed earlier than their text
implies:

- **0010** can be implemented before 0018, except that multi-identity selection
  produces correct work-type figures only once 0018 exists.
- **0019** can be established before the records it tests; each invariant
  becomes assertable when its subject record is implemented.

---

## Prohibitions, collected

Every item below is stated in a record and repeated here so that it can be
checked without reading the full set. The governing record is authoritative.

**Product and data**

- No hosting provider API is used for analysis data. (0007)
- No repository credential is stored, encrypted, transmitted or logged, and no
  interface field accepts one. (0016)
- No user accounts, passwords, sessions or third-party sign-in. (0010)
- No telemetry, no corpus collection, no transmission of analysis results. (0013)
- No population percentile, global benchmark or cross-user comparison is stated
  or implied. (0013)
- No contributor list ordered by output volume; no leaderboard, score, rating,
  rank or grade. (0009)
- A language model never assigns, invents, renames or overrides an archetype or
  badge, and is never required for any capability. (0014, 0039)
- Only archetype, badges and numeric summaries may be sent to a model; never raw
  messages, paths, identities or addresses. (0039)
- No skipped family is absent from a report, and none is zero-filled. (0032)
- No artifact that can leave the machine — report, exported image, API response,
  collected log — contains a path, address or hostname. An interactive
  diagnostic may name a path only exactly as the operator supplied it, and never
  an address or hostname. (0033, 0041, 0067)
- In public mode, person-scoped content is never addressable, indexable or
  persisted. (0029, 0033)
- No report contains a person's interpretation; it is computed on request.
  (0077)
- No static HTML generation; the CLI emits one file. (0034)
- No flag to skip blame exists; replay makes it unnecessary. (0020)
- No billing, entitlement, licence-key or quota code. (0003)
- Exactly one file declares link-time variables, and build metadata never enters
  a metric, a cache key or a golden comparison. (0061)
- No metric, reason code or cardinality limit exists that `docs/metrics.md` does
  not define, and no cardinality limit is operator-settable. (0062, 0053)
- No raw address appears in the embedded configuration; identifying values are
  digests, and pseudonyms under anonymisation. (0068)
- No merge candidate is derived from a value the analysed commits do not record.
  (0069)
- No top-level section's derivation changes without its version moving, and
  every section version is in the cache key. (0070)
- No build metadata is derived from the clock; two builds of one commit are
  identical. (0063)
- No check runs over the working tree; tracked files only. (0063)
- No check skips because the gate failed to provide its precondition; a missing
  precondition fails. (0064)
- No check is accepted until it has been observed failing. (0064)

**Architecture**

- No metric family reads another family's output, and no metric package imports
  another metric package. (0024, 0040)
- Resolving a family's declared inputs never changes its status; if it does, the
  declaration is wrong. No family reads the working tree. (0076)
- No package sits outside the named layout; `internal/checks/` may import
  anything and nothing may import it. (0060)
- Nothing under `internal/core` imports anything outside `core`, subpackages
  included, and a subpackage exists only to resolve a name collision. (0066)
- No package outside the git package invokes git. Non-git subprocesses run only
  at the closed set of sites in ADR-0065 clause 3. (0065)
- No subprocess anywhere is invoked through a shell. Record framing is
  NUL-delimited or length-prefixed, never line-framed, and a malformed
  length header aborts rather than resynchronises. (0065, 0072)
- No operator's personal git configuration can change a report; every
  output-affecting key is pinned on the invocation. (0071)
- No globals, package-level mutable singletons, services in `context`, or direct
  process clock calls. (0042)
- No bare `go` statement; every goroutine has an owner. (0044)
- No panic is reachable from repository content or user input. (0041)
- No external service is required in any mode; the only runtime dependency
  outside the binary is `git`. (0022)
- No JavaScript runtime at runtime; no C toolchain for cross-compilation.
  (0036, 0035)
- No visualisation library touches the DOM; no autonomous charting library is
  used. (0037)
- No raw colour, spacing, typography or radius value in the frontend. (0038)
- Replay is never parallelised; parallelism degree never changes the report.
  (0052)
- Replay never walks history as a single running state, never reads the working
  tree, and never uses git's diff output for line ownership. (0073)
- Replay applies no analysis parameter; a rewrite counts once, and the recency
  window is applied in aggregation. (0074)
- No visualisation renders unbounded cardinality; limits are applied in the
  report. (0053)

**Process**

- No dates, durations, schedules or phase names in any document. (0004)
- No accepted record is edited to change meaning; it is superseded. (0001)
- No checker is disabled, loosened or excepted without a record change. (0055)
- No budget is loosened without a record; no check is deleted to fit a gate
  budget. (0054, 0057)
- No scheduled pipeline is the sole location of any check. (0057)
- No direct dependency outside the allow list; no committed frontend bundle that
  does not match its source. (0049)
