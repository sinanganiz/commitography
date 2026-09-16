# Work package index

Work packages are numbered identifiers, not phases. There are no phases,
milestones, dates or durations anywhere in this directory (ADR-0004). Ordering
is expressed only through the `Requires` field, and the waves below are derived
from it.

A **wave** is not a time period. It means: nothing in wave *n* can begin before
everything it requires, which lives in an earlier wave, is done. Packages inside
one wave have no dependency on each other and may be worked in parallel — but
only when their `Files` sections do not overlap. Because work happens directly
on `main` rather than on per-package branches, two packages can only be worked
at once from separate working copies; check the `Files` sections first.

Status lives in this table only. Package files carry no status field, so the
two can never disagree.

`Ready` packages carry full step-level detail. `Draft` packages carry goal,
records and dependencies only; each is expanded into a full package before it is
started, using `docs/work/audit.md` and the state of the tree at that point
(M3). Expanding a draft does not change its number, its records or its
dependencies without a stated reason.

## Rules

1. A package is done when every statement in its `Definition of done` is
   satisfied. "It works" is not a criterion (ADR-0004 clause 4).
2. A package must not touch a file outside its `Files` allow list. If the work
   cannot be completed without doing so, stop: either the package is wrong or a
   dependency is missing. Record which, do not widen the list silently.
3. Anything not listed in `In scope` is out of scope, including improvements
   that look obviously correct.
4. A package that introduces a rule introduces its checker in the same change
   (ADR-0055 clause 6).
5. A package that changes analysis output updates its golden files in the same
   change, with the reason stated in the commit body (ADR-0019 clause 2).

## Writing a package

These are notes for whoever writes the packages, learned from packages that had
to be amended mid-flight.

- **A package that moves or renames anything must allow every file that names
  the old location**, including build and release configuration, ignore and
  attribute files, and comments. A build tool that silently accepts a flag
  pointing at a name that no longer exists turns a move into a silent
  behavioural change.
- **A package must define its own boundary, not delegate it to another
  document.** "Fix everything in the audit" made WP-0002 unsatisfiable, because
  the audit listed work the package was forbidden to do.
- **A name is not a verification mechanism.** WP-0004 required renaming
  fixtures to state their conditions; the rename cost forty test references and
  verified nothing. A manifest plus a checker does the job.
- **A prohibition must be written at the width of its purpose.** ADR-0047 meant
  "git hardening happens in one place" and said "no package may start a
  process", which forbade the verification tooling that enforces it. When a rule
  exists to make something happen once, say that, not the widest thing that
  would also achieve it.
- **A move package must state what it deliberately leaves wrong**, and name the
  package that fixes it. Code that contradicts a record but cannot be corrected
  without changing behaviour is a recorded deviation, not a silent one.
- **A package that cannot be split must be resumable rather than smaller.**
  WP-0005 is atomic: its done-conditions only hold once the whole move is
  finished, so halving it satisfies nothing. Where a package has no such
  constraint, prefer the smaller one that merges on its own.
- **Give a structure its final shape before filling it.** WP-0008 puts every
  family in the report at once, most of them `skipped`, so the document never
  gains a top-level key again and each later package only fills a slot.

---

## Packages

| # | Area | Title | Implements | Requires | Status |
|---|---|---|---|---|---|
| [0001](0001-repository-audit.md) | foundation | Repository audit | — | — | Done |
| [0002](0002-neutralise-contradicting-documents.md) | foundation | Neutralise contradicting documents | ADR-0001, ADR-0004, ADR-0021, ADR-0034 | WP-0001 | Done |
| [0003](0003-enforcement-skeleton.md) | foundation | Enforcement skeleton | ADR-0055, ADR-0063, ADR-0057, ADR-0064 | WP-0001 | Done |
| [0004](0004-fixtures-and-golden-harness.md) | foundation | Deterministic fixtures and golden harness | ADR-0019, ADR-0057, ADR-0064 | WP-0003 | Done |
| [0005](0005-package-layout-migration.md) | foundation | Package layout migration | ADR-0040, ADR-0049, ADR-0060, ADR-0061, ADR-0063, ADR-0065, ADR-0066 | WP-0003, WP-0004 | Done |
| [0006](0006-error-model.md) | foundation | Error model and reason codes | ADR-0041, ADR-0032, ADR-0062, ADR-0064 | WP-0005 | Ready |
| [0007](0007-dependency-wiring.md) | foundation | Dependency wiring and ambient state removal | ADR-0042, ADR-0061, ADR-0021 | WP-0005 | Ready |
| [0008](0008-report-document.md) | core | Report document and schema versioning | ADR-0021, ADR-0031, ADR-0032, ADR-0062 | WP-0005, WP-0006 | Ready |
| [0009](0009-identity-and-privacy.md) | core | Identity model and privacy layers | ADR-0033, ADR-0010, ADR-0032 | WP-0008 | Ready |
| [0010](0010-configuration-planes.md) | core | Configuration planes and resolution | ADR-0026, ADR-0021, ADR-0062 | WP-0008 | Ready |
| [0011](0011-git-chokepoint.md) | pipeline | Git invocation chokepoint | ADR-0065, ADR-0044, ADR-0066, ADR-0041 | WP-0005, WP-0006, WP-0007 | Ready |
| 0012 | pipeline | Collect stage | ADR-0020, ADR-0007, ADR-0052 | WP-0011, WP-0009, WP-0010 | Draft |
| 0013 | pipeline | Replay stage and ownership map | ADR-0020, ADR-0051 | WP-0012 | Draft |
| 0014 | pipeline | Work-type classification | ADR-0020, ADR-0018 | WP-0013 | Draft |
| 0015 | pipeline | Aggregate stage and family harness | ADR-0024, ADR-0032, ADR-0052 | WP-0013, WP-0008 | Draft |
| 0016 | pipeline | Interpret stage | ADR-0020, ADR-0014 | WP-0015 | Draft |
| 0017 | pipeline | Render boundary and CLI | ADR-0034, ADR-0021 | WP-0015, WP-0016 | Draft |
| 0018 | metrics | temporal family | ADR-0024 | WP-0015 | Draft |
| 0019 | metrics | commit-size family | ADR-0024 | WP-0015 | Draft |
| 0020 | metrics | messages family | ADR-0024 | WP-0015 | Draft |
| 0021 | metrics | files family | ADR-0024 | WP-0015 | Draft |
| 0022 | metrics | coupling family | ADR-0024, ADR-0053 | WP-0015 | Draft |
| 0023 | metrics | ownership family | ADR-0024 | WP-0015, WP-0013 | Draft |
| 0024 | metrics | worktype family output | ADR-0024, ADR-0018 | WP-0015, WP-0014 | Draft |
| 0025 | metrics | ai-archaeology family | ADR-0024 | WP-0015, WP-0013 | Draft |
| 0026 | metrics | hotspot family | ADR-0024 | WP-0015 | Draft |
| 0027 | metrics | static-analysis placeholder | ADR-0012, ADR-0032 | WP-0015 | Draft |
| 0028 | interpret | Axis layer | ADR-0030, ADR-0013 | WP-0016, WP-0018, WP-0019, WP-0020, WP-0021, WP-0022, WP-0023, WP-0024, WP-0025, WP-0026 | Draft |
| 0029 | interpret | Archetype and badge evaluation | ADR-0014, ADR-0030 | WP-0028 | Draft |
| 0030 | interpret | Taxonomy calibration | ADR-0030, ADR-0019 | WP-0029, WP-0004 | Draft |
| 0031 | interpret | Language model prose surface | ADR-0039 | WP-0029 | Draft |
| 0032 | storage | Storage interface and embedded engine | ADR-0022, ADR-0035 | WP-0005, WP-0007 | Draft |
| 0033 | storage | Report cache and keying | ADR-0017, ADR-0031 | WP-0032, WP-0008, WP-0010 | Draft |
| 0034 | storage | Checkpoint persistence and incremental resume | ADR-0017 | WP-0032, WP-0013 | Draft |
| 0035 | storage | Repository registry and version history | ADR-0011 | WP-0033 | Draft |
| 0036 | storage | Person records | ADR-0025 | WP-0035, WP-0009 | Draft |
| 0037 | server | API skeleton, versioning, capability discovery | ADR-0043, ADR-0029 | WP-0032, WP-0006 | Draft |
| 0038 | server | Job manager, two-class queue, per-repository lock | ADR-0027, ADR-0044 | WP-0037, WP-0034 | Draft |
| 0039 | server | Progress streaming and cancellation | ADR-0043, ADR-0044 | WP-0038 | Draft |
| 0040 | server | Path validation and allowed roots | ADR-0046 | WP-0037 | Draft |
| 0041 | server | Remote repositories without stored credentials | ADR-0016 | WP-0040, WP-0011 | Draft |
| 0042 | server | Recurring re-analysis | ADR-0011 | WP-0038, WP-0035, WP-0041 | Draft |
| 0043 | server | Resource limits and degraded reporting | ADR-0048 | WP-0038, WP-0008 | Draft |
| 0044 | server | Remote URL validation | ADR-0045 | WP-0041 | Draft |
| 0045 | server | Public mode and capability matrix | ADR-0029, ADR-0045 | WP-0037, WP-0044, WP-0043 | Draft |
| 0046 | frontend | Frontend foundation, embedding and meta injection | ADR-0036, ADR-0049 | WP-0037 | Draft |
| 0047 | frontend | Design tokens and headless primitives | ADR-0038 | WP-0046 | Draft |
| 0048 | frontend | Visualisation primitives | ADR-0037 | WP-0047 | Draft |
| 0049 | frontend | Registry and repository view shell | ADR-0028 | WP-0047, WP-0037 | Draft |
| 0050 | frontend | Metric sections, empty and degraded states | ADR-0032 | WP-0049, WP-0048 | Draft |
| 0051 | frontend | Person lens | ADR-0028, ADR-0010 | WP-0050, WP-0024 | Draft |
| 0052 | frontend | Time lens | ADR-0028, ADR-0011 | WP-0050, WP-0035 | Draft |
| 0053 | frontend | Frontend budgets and cardinality verification | ADR-0053 | WP-0050 | Draft |
| 0054 | wrapped | Wrapped shell and card system | ADR-0008, ADR-0023 | WP-0047, WP-0029 | Draft |
| 0055 | wrapped | Repository Wrapped cards | ADR-0023 | WP-0054 | Draft |
| 0056 | wrapped | Person Wrapped cards | ADR-0023, ADR-0009 | WP-0054, WP-0051 | Draft |
| 0057 | wrapped | Client-side image export | ADR-0015 | WP-0055, WP-0056 | Draft |
| 0058 | distribution | Container image and deployment documentation | ADR-0046, ADR-0016 | WP-0045 | Draft |
| 0059 | distribution | Release pipeline | ADR-0049, ADR-0057 | WP-0058, WP-0003 | Draft |
| 0060 | distribution | User documentation rewrite | ADR-0001 | WP-0057, WP-0058 | Draft |

---

## Waves

| Wave | Packages |
|---|---|
| 1 | 0001 |
| 2 | 0002, 0003 |
| 3 | 0004 |
| 4 | 0005 |
| 5 | 0006, 0007 |
| 6 | 0008, 0011, 0032 |
| 7 | 0009, 0010, 0037 |
| 8 | 0012, 0033, 0040, 0046 |
| 9 | 0013, 0035, 0041, 0047 |
| 10 | 0014, 0015, 0034, 0036, 0044, 0048, 0049 |
| 11 | 0016, 0018, 0019, 0020, 0021, 0022, 0023, 0024, 0025, 0026, 0027, 0038, 0050 |
| 12 | 0017, 0028, 0039, 0042, 0043, 0051, 0052, 0053 |
| 13 | 0029, 0045 |
| 14 | 0030, 0031, 0054, 0058 |
| 15 | 0055, 0056, 0059 |
| 16 | 0057 |
| 17 | 0060 |

---

## Goals

| # | Goal |
|---|---|
| 0001 | Every tracked file is classified keep / change / delete with a governing record cited. |
| 0002 | No document states a rule contradicting the decision set. |
| 0003 | Every enforceable rule whose subject exists is enforced; both CI gates run within budget. |
| 0004 | Fixtures are byte-reproducible and output changes fail without a golden update. |
| 0005 | The tree matches the layout, import direction is enforced, goldens unchanged. |
| 0006 | Errors are classified user or internal, with one enumerated reason set and one exit-code mapping. |
| 0007 | No globals, no clock reads, no services in context; wiring is explicit in one place. |
| 0008 | The report document exists, versioned per family, every family present with a status. |
| 0009 | Raw identities stay internal; exported artifacts carry display name and digest only. |
| 0010 | Resolved analysis configuration is embedded in the report and reproduces it exactly. |
| 0011 | All git invocation passes one hardened package, NUL-delimited and context-bound. |
| 0012 | A single pass produces normalized commit records, independently cacheable. |
| 0013 | Chronological replay maintains compact line ownership for the analysed commit. |
| 0014 | Lines are classified during replay with no blame invocation, in a dual breakdown. |
| 0015 | Families declare inputs, own namespaces, carry versions, and report status. |
| 0016 | A stateless stage assigns archetypes and badges over a stored report. |
| 0017 | The CLI emits exactly one file, deterministic and identical to server output. |
| 0018 | Temporal metrics as defined in docs/metrics.md section 2. |
| 0019 | Commit size metrics as defined in docs/metrics.md section 3. |
| 0020 | Message metrics, keyword rules file, and confidence degradation. |
| 0021 | File activity metrics as defined in docs/metrics.md section 5. |
| 0022 | Coupling pairs and bounded graph with cardinality degradation. |
| 0023 | Line-based bus factor, concentration, code age over all tracked lines. |
| 0024 | Dual-breakdown output and repository shares, projectable without recomputation. |
| 0025 | Assisted detection rules file and the assisted versus unassisted comparisons. |
| 0026 | Indentation-based complexity proxy and hotspot scoring, skipped without a worktree. |
| 0027 | The family is present in every report as skipped with reason not_implemented. |
| 0028 | Named axes are computed from families per axes.md, including in-repository ranks. |
| 0029 | Ordered first-match evaluation over taxonomy.yml, with a guaranteed fallback. |
| 0030 | Every archetype is reachable by a fixture and thresholds are calibrated against them. |
| 0031 | One compatible HTTP interface, default off, with model-free equivalence proven. |
| 0032 | Pure Go embedded metadata storage with blobs on disk, behind one interface. |
| 0033 | Reports are cached under a key covering repository, commit, config and versions. |
| 0034 | Replay resumes from a checkpoint, with descendant verification and full rebuild otherwise. |
| 0035 | Registered repositories and their analysis versions survive restart and are comparable. |
| 0036 | Cross-repository person records exist as normalization, carrying no credential. |
| 0037 | A versioned API serves capability discovery reflecting the active mode. |
| 0038 | Interactive jobs preempt background jobs; one active job per repository is enforced. |
| 0039 | Progress streams over SSE; cancellation leaves no running subprocess. |
| 0040 | Canonical containment, no symlink escape, product-generated clone targets. |
| 0041 | Remote analysis works with host-provided authentication and stores no secret. |
| 0042 | Registered repositories re-analyse on an operator-defined recurrence. |
| 0043 | Limits are enforced and reaching one produces a degraded report, never truncation. |
| 0044 | Resolved addresses in private, loopback, link-local and metadata ranges are refused. |
| 0045 | One switch selects a fixed capability set, verified unreachable on every route. |
| 0046 | A static bundle is embedded, verified against source, with route-specific metadata. |
| 0047 | The opinionated component library is gone; raw style values fail lint. |
| 0048 | Layout mathematics is computed, React renders; no library touches the DOM. |
| 0049 | Three navigation levels exist; the repository view is complete with no lens. |
| 0050 | Every family renders, including skipped and degraded, with reachable definitions. |
| 0051 | The person lens is a state over the repository view, with multi-identity selection. |
| 0052 | Version comparison is a state over the repository view, not a separate report. |
| 0053 | Bundle and render budgets are gates; truncation is visible to the reader. |
| 0054 | A full-screen presentation surface with its own visual language, code-split. |
| 0055 | The repository year in review, publishable in public mode. |
| 0056 | The person year within a repository, session-scoped in public mode. |
| 0057 | Cards export to images in the browser, carrying no address, path or hostname. |
| 0058 | One image serves both modes; private remotes are documented via host secrets. |
| 0059 | Release artifacts are reproducible, accompanied by an SBOM and signed. |
| 0060 | README and user documentation describe what exists, with no invalidated claim. |
