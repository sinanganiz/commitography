# Phase 2 — CI/CD Integration: Detailed Work Packages

**Status:** Open for development — opened 2026-09-06
**Scope document:** [`phase-2.md`](phase-2.md). Where this document and that one disagree, this one wins and the disagreement MUST be recorded in §7 below.

**Goal of Phase 2:** A team adds Commitography to an existing repository on GitHub, GitLab, or Bitbucket by copying one template file, and receives a correctly generated, automatically updated dashboard on a schedule, with incremental runs demonstrably faster than full runs, and with no manual steps beyond the initial copy.

This document is the index. The work packages themselves live in [`phase-2/`](phase-2/), one file per milestone. Read this file first: it carries the decisions every work package assumes, the dependency order, and the exit criteria.

---

## 1. Prerequisite waiver

[`phase-2.md`](phase-2.md) §1 requires Phase 1 to be released and to have received real-world usage feedback before Phase 2 begins. **That prerequisite is waived by explicit decision on 2026-09-06.** Phase 1 development is complete; three of its exit criteria remain open, and two of them are closed by Phase 2 work packages rather than before them.

Phase 1's three open criteria are carried into this plan as follows:

| Phase 1 criterion | Closed by | Status |
|---|---|---|
| 11 — Release artifacts published, installable from two package managers | [WP-1.1 – WP-1.4](phase-2/m1-release.md) | Scheduled |
| 12 — Three public reference dashboards live and linked | [WP-5.6 – WP-5.7](phase-2/m5-github.md) | Scheduled |
| 1 — Runs on macOS | **Nothing in this plan** | **Blocked, unscheduled** — see §2 |

## 2. The macOS criterion is still open, and Phase 2 does not close it

Reintroducing the project's own continuous integration was proposed and **declined** on 2026-09-06. That decision is respected throughout this plan: no verification CI workflow is created by any work package.

The consequence is direct and MUST NOT be glossed over. Phase 1 exit criterion 1 — running the built binary once on macOS — had exactly two possible closing paths: macOS hardware, or CI. Neither exists. The criterion therefore stays open for the whole of Phase 2, and Phase 1's rule that criteria 1 through 7 gate any public announcement still stands.

This matters more than an unfinished errand. The `LC_UUID` defect described in [`phase-1-detailed.md`](phase-1-detailed.md) made every macOS binary refuse to start, and it was invisible from every other platform. Phase 2 ships release artifacts (WP-1.3) and a Homebrew formula (WP-1.4) that macOS users will install. Shipping them without one execution on macOS means shipping a platform nobody has run.

**Normative rules that follow from this:**

1. No work package in Phase 2 MAY claim that macOS is verified.
2. [WP-1.3](phase-2/m1-release.md) MUST tag a pre-release, not a stable release, until criterion 1 is met.
3. The moment macOS hardware becomes available, `commitography --version` MUST be run on the released darwin binary and the result recorded in [`phase-1-detailed.md`](phase-1-detailed.md). This is a one-command errand with no dependencies; it is unscheduled only because it cannot be scheduled.
4. If the decision on CI is revisited, a single `macos-latest` job running `make build` and `./commitography --version` closes the criterion and SHOULD be the first thing added.

## 3. Continuous integration versus the dogfood workflow

This plan creates files under `.github/workflows/`. They are **not** the continuous integration that was declined, and the distinction is normative:

| | Declined verification CI (Phase 1 Task 0.5) | Dogfood publish workflow (WP-5.6) |
|---|---|---|
| Trigger | Every push and pull request | Schedule and manual dispatch |
| Purpose | Prove the tree builds and tests pass on three operating systems | Run the shipped product and publish its output |
| Fails the build on | Lint, test, or build failure | Nothing; it publishes or it does not |
| Runs `go test` | Yes | No |

The dogfood workflow exists because the product's central deliverable is a CI template, and a CI template that its own project does not use is untested marketing. It performs no verification of the source tree and MUST NOT grow into doing so; if a work package wants `go test` in a workflow, that is the declined CI and requires a new decision.

---

## 4. Approved decisions

Every work package assumes these. They were reviewed and approved on 2026-09-06. A work package MUST NOT quietly contradict one; changing a decision means changing it here first.

### Sequencing and scope

| # | Decision | Chosen | Rationale |
|---|---|---|---|
| A1 | Relationship to Phase 1's open criteria | Phase 2 packages that close Phase 1 criteria run first | Two of the three criteria close as a side effect of work Phase 2 needs anyway |
| A2 | Reinstate the project's own CI | **No** | Declined. See §2 and §3 |
| A3 | Which CI systems, at what depth | Three templates; Action, Pages and dogfood for GitHub only | Templates are cheap; the Action and Pages plumbing are not. GitLab and Bitbucket templates ship **labelled as never executed** |
| A4 | Azure Pipelines and Jenkins | Document the shallow-clone setting only, ship no template | Holds the scope already described in `phase-2.md` §3 |

### Incremental analysis

| # | Decision | Chosen | Rationale |
|---|---|---|---|
| B1 | Cache format | The existing `model.History` JSON artifact plus a manifest. **No SQLite** | `mattn/go-sqlite3` needs cgo and would break `CGO_ENABLED=0` cross-compilation to six targets. `modernc.org/sqlite` is pure Go but pulls dozens of modules. Neither buys anything here: SQLite's advantage is partial and random reads, and this pipeline already loads every commit into memory to aggregate it |
| B2 | What the cache stores | Normalized commit records **and** the previous report | The report is small (47 KB at 2,584 commits) and is the natural default baseline for D2 |
| B3 | Blame caching | Yes, keyed by `(path, blob oid)` plus two freshness conditions | Measured: blame is half the runtime on a mid-size repository and is the half a commit cache cannot touch. **The original justification for this decision was wrong in a way detailed design caught:** a blob object name does *not* pin blame output, because a change-and-revert restores the content while moving the blame to the reverting commit, and a rebase moves lines between years without touching content. [WP-3.2](phase-2/m3-blame-cache.md) adds two conditions that close both holes using information M2 already computes, at no extra cost |
| B4 | Cache location | `os.UserCacheDir()` by default, `--cache <dir>` to override | CI needs an explicit path to hand to its cache action. Writing into the analyzed repository is forbidden: never writing to the repository under analysis is one of this tool's load-bearing claims |
| B5 | Fingerprint scope | Full effective config, plus date bounds, plus content hashes of `.mailmap` and root `.gitattributes` | `phase-2.md` §3: any inconsistency must cause a full re-read. A silently edited `.mailmap` changing author identities under a valid-looking cache is exactly the failure mode being forbidden |
| B6 | Shard threshold (currently 5,000 commits) | Calibrate it with the same benchmark harness the incremental work needs | Measured: a 2,584-commit repository spends 12 s in a single `git log`, below the threshold and therefore on the slow path |

### CI deliverables

| # | Decision | Chosen | Rationale |
|---|---|---|---|
| C1 | GitHub Action type | Composite | A Docker action adds image-pull latency to every run; a JavaScript action adds Node and a committed `dist/` bundle |
| C2 | How the Action obtains the binary | Download the release asset, verify SHA-256 against the published checksum file | goreleaser already emits `checksums.txt`, so verification costs nothing. `go install` or building from source adds a toolchain and build time to every run |
| C3 | Container registry | GHCR only | [`.goreleaser.yml`](../.goreleaser.yml) already defines GHCR images and a multi-arch manifest. Docker Hub's anonymous pull limits are worse for CI |
| C4 | Dogfood the templates | Yes — the project publishes its own dashboard with its own Action | Verifies the template in a real pipeline, closes part of Phase 1 criterion 12, and fills a `TODO` in the README |
| C5 | Private repositories | Document the Pages recipe **and** an artifact-download fallback | GitHub Pages is unavailable on private repositories outside paid plans, and much of the target audience is private and self-hosted |

### Output and reporting

| # | Decision | Chosen | Rationale |
|---|---|---|---|
| D1 | `--summary` format and destination | Markdown on stdout | Phase 1 deliberately left stdout free by sending progress to stderr. Markdown on stdout redirects into `$GITHUB_STEP_SUMMARY`, a file, or a PR-comment API call without the tool knowing which |
| D2 | Source of the previous-run comparison | The report stored in the cache by default; `--baseline <path>` overrides | The cache is where a previous run already lives in CI; an explicit path covers everything else |
| D3 | Output size budget | A documented budget with a warning when exceeded, plus a cap on the per-author section | Measured: 108 KB total at 2,584 commits, and most report arrays have fixed top-N caps. The only unbounded section is `perAuthor` |
| D4 | Report `schemaVersion` | Stays at 1; additions only | Task 4.1 forbids removing or repurposing fields, not adding them. Bumping the version breaks every consumer of `report-schema.json` for no gain |

### Contract and policy

| # | Decision | Chosen | Rationale |
|---|---|---|---|
| E1 | Third-party dependency policy | Unchanged. Phase 2 adds **zero** new dependencies | A direct consequence of B1, and a claim worth being able to make |
| E2 | Revising `project-overview.md` | Yes, as the first work package | §5 and §8 name SQLite as the Phase 2 cache. Governance (§12) requires the document to be revised before the scope changes, not after |
| E3 | `--strict` and exit code 3 | Implement both | The README already reserves exit code 3. CI is precisely where a caller wants warnings to fail a build, and a reserved-but-unimplemented exit code is a contract no template can use |
| E4 | Phase 2 exit criteria | A numbered, individually checkable table (§6 below) | What made Phase 1 closable was that its criteria could be ticked off one at a time without argument |

### Document structure

| # | Decision | Chosen | Rationale |
|---|---|---|---|
| F1 | One file or a directory | This index plus one file per milestone under `docs/phase-2/` | Phase 1's equivalent ran to 1,278 lines for 32 tasks; the cache milestone alone is comparable to a Phase 1 milestone |
| F2 | Work package format | Phase 1's format: RFC 2119 language, exact paths and signatures, explicit acceptance criteria | The stated requirement is instructions that are closed to interpretation |

---

## 5. Measurements this plan is built on

Taken on the Windows development machine (16 cores) on 2026-09-06 against `C:\Users\sinan\source\repos\ays-msm`: 2,584 commits, 11 contributors, 14,127 files in HEAD, 18,805 distinct paths in history, spanning 2025-12-17 to 2026-09-06.

| Measurement | Result |
|---|---|
| Full run, blame enabled, `--per-author` | 22.5 s |
| Same run with `--no-blame` | 11.1 s |
| Raw `git log --numstat` over the same history, no Go involved | 12.0 s |
| `report.json` / `index.html` at 2,584 commits | 47 KB / 61 KB |
| `report.json` / `index.html` at 11 commits (this repository) | 12 KB / 37 KB |

Three conclusions follow, and they are the load-bearing facts behind B1, B3, B6 and D3:

1. **The Go side is effectively free.** A blame-less run costs what git's diff computation costs and nothing more. An incremental cache that avoids re-reading old history therefore removes almost the entire cost of a repeat run. The cache is justified by measurement, not by assumption — which is exactly the question [`phase-2.md`](phase-2.md) §6 left open.
2. **Blame is the other half, and a commit cache does not touch it.** Blame is HEAD-relative, so a cache keyed by commit range cannot help it. Without B3, an incremental run on this repository improves from 22.5 s to roughly 11.5 s rather than to roughly 1 s.
3. **Published output is small and mostly bounded.** Report size grows with months of history and contributor count, not with commit count, because the expensive arrays are capped at top-N. The size budget in `phase-2.md` §3 is a real concern only for `perAuthor`.

Every performance claim made by a Phase 2 work package MUST be re-measured on completion and recorded in [`m8-performance.md`](phase-2/m8-performance.md). Numbers copied from this table into a later document without re-measurement are inadmissible.

---

## 6. Exit criteria

Phase 2 is complete when all twelve hold. Each is independently checkable; none may be marked met by inspection of code alone where the criterion names an observable behaviour.

| # | Criterion | How it is verified |
|---|---|---|
| 1 | A tagged release publishes archives for all twelve targets, a checksum file, and a multi-arch GHCR image | The release page and `docker pull ghcr.io/sinanganiz/commitography:<tag>` on both architectures |
| 2 | The binary is installable from Homebrew and Scoop | `brew install sinanganiz/tap/commitography` and `scoop install commitography` each produce a binary that answers `--version` |
| 3 | A second run over unchanged history reads zero commits from git and produces a report byte-identical to the first, `generatedAt` excepted | Automated test plus a recorded measurement |
| 4 | Every invalidation trigger in the cache contract causes a full re-read, and no trigger produces a partial or stale result | One automated test per trigger |
| 5 | Deleting the cache directory at any point changes nothing but runtime | Automated test comparing cached and uncached reports field by field |
| 6 | An incremental run on a repository above 100,000 commits is at least five times faster than the equivalent full run | Recorded measurement in `m8-performance.md` |
| 7 | `--summary` emits Markdown on stdout and nothing else, and stdout stays empty without it | Automated test |
| 8 | `--strict` exits 3 when the report carries warnings and 0 when it does not | Automated test |
| 9 | A shallow checkout under each of the five documented CI systems fails with that system's own fix instruction | Automated test over synthesized environments |
| 10 | Copying one template file into a repository on GitHub produces a scheduled, published dashboard with no further manual steps | Performed on a real repository |
| 11 | The project publishes its own dashboard, plus two other public repositories', from its own template | Three live URLs, linked from the README, replacing both `TODO` blocks |
| 12 | Phase 2 added no new third-party dependency | `go.mod` still lists exactly cobra, yaml.v3 and doublestar |

Criteria 1, 2 and 11 also close Phase 1 exit criteria 11 and 12. **No Phase 2 criterion closes Phase 1 criterion 1.**

---

## 7. Departures from `phase-2.md`

Recorded as they are decided, in the style of Phase 1's departures section. Each MUST also be applied to the scope document itself by [WP-0.1](phase-2/m0-contracts.md).

1. **SQLite is not used.** `phase-2.md` §2 and §3, and `project-overview.md` §5 and §8, name SQLite as the Phase 2 cache. Decision B1 replaces it with the existing JSON history artifact plus a manifest. Reason: cgo breaks the cross-compilation story, a pure-Go SQLite violates the dependency policy for no benefit, and this pipeline never performs the partial reads that would justify a database.

2. **A dropped branch does not invalidate the whole cache.** `phase-2.md` §3 lists dropped branches alongside rewritten history as full-invalidation triggers. The refined rule is that cached commits absent from the current `rev-list` output are discarded and the remainder is kept. Git object names are content-addressed: a commit still reachable has byte-identical content to when it was cached, so the retained subset cannot be stale. Rewritten history is still caught, because rewritten commits have different hashes and therefore disappear from the cached set the same way. This is a refinement rather than a weakening, and [WP-2.4](phase-2/m2-cache.md) implements it with a test per case.

3. **The fingerprint is split per artifact rather than being one hash of everything.** Decision B5 as approved says the fingerprint covers the full effective configuration. Detailed design found that the two cached artifacts do not depend on the same inputs: collection stores every commit and every file change unfiltered, and exclusion, identity resolution, merge handling and bulk detection all happen afterwards in stages that re-run on every invocation. So `history.json` depends only on `use_mailmap`, the date bounds, `.mailmap` content and the schema versions, while `report.json` depends on everything.

   A single wide fingerprint would be safe but would force a full re-read of a large repository every time an exclusion pattern was tuned — which is exactly what someone does repeatedly while setting the tool up. Splitting it keeps B5's guarantee intact, because nothing that can change an artifact is omitted from that artifact's fingerprint; it only stops invalidating an artifact for a reason that cannot affect it. [WP-2.2](phase-2/m2-cache.md) specifies both fingerprints field by field, and [WP-2.7](phase-2/m2-cache.md) tests the boundary in both directions.

---

## 8. Milestones and dependency order

| Milestone | File | Depends on | Closes |
|---|---|---|---|
| M0 — Contracts and documents | [`m0-contracts.md`](phase-2/m0-contracts.md) | — | — |
| M1 — Release and distribution | [`m1-release.md`](phase-2/m1-release.md) | M0 | Phase 1 criterion 11; Phase 2 criteria 1, 2 |
| M2 — Cache engine | [`m2-cache.md`](phase-2/m2-cache.md) | M0 | Phase 2 criteria 3, 4, 5 |
| M3 — Blame cache | [`m3-blame-cache.md`](phase-2/m3-blame-cache.md) | M2 | — |
| M4 — CLI surface for CI | [`m4-cli.md`](phase-2/m4-cli.md) | M2 | Phase 2 criteria 7, 8, 9 |
| M5 — GitHub Action, Pages, dogfood | [`m5-github.md`](phase-2/m5-github.md) | M1, M4 | Phase 1 criterion 12; Phase 2 criteria 10, 11 |
| M6 — GitLab and Bitbucket templates | [`m6-gitlab-bitbucket.md`](phase-2/m6-gitlab-bitbucket.md) | M4 | — |
| M7 — Documentation | [`m7-docs.md`](phase-2/m7-docs.md) | M5, M6 | — |
| M8 — Performance verification | [`m8-performance.md`](phase-2/m8-performance.md) | M2, M3 | Phase 2 criterion 6 |

M0 MUST complete before any other milestone starts: it changes the contract that M2 and M3 implement. M1 is placed second because it is the only milestone that unblocks a Phase 1 criterion without depending on new code. M3 is the designated scope-reduction point: if Phase 2 must be cut short, M3 is dropped first, decision B3 reverts to accepting blame as the incremental floor, and exit criterion 6 is renegotiated.

---

## 9. Conventions

- **MUST / MUST NOT / SHOULD / MAY** carry RFC 2119 meaning.
- All file paths are relative to the repository root.
- "The analyzed repository" is the repository being measured. "The project repository" is the Commitography source tree.
- Go version: **1.22 or later**, as declared in `go.mod`. New code MUST NOT require a later version.
- **No new third-party Go dependency may be added.** The permitted list remains cobra, yaml.v3 and doublestar. A work package that appears to need a fourth is a work package with the wrong design; escalate rather than add.
- All exported types and functions MUST have doc comments.
- New behaviour MUST be covered by a test that fails without it. Work packages that cannot be tested automatically say so explicitly and name what is verified by hand instead.
- Every work package that changes user-visible behaviour MUST update `README.md` in the same change, not in M7. M7 covers the CI integration page only.
- Progress and diagnostics go to **stderr**. stdout carries `--summary` output and nothing else.

---

## 10. Status tracking

Each milestone file carries its own status table. This index carries the roll-up. Update both in the same commit.

| Milestone | Packages | Done | Status |
|---|---|---|---|
| M0 | 3 | 0 | Not started |
| M1 | 5 | 0 | Not started |
| M2 | 7 | 0 | Not started |
| M3 | 4 | 0 | Not started |
| M4 | 6 | 0 | Not started |
| M5 | 7 | 0 | Not started |
| M6 | 3 | 0 | Not started |
| M7 | 3 | 0 | Not started |
| M8 | 4 | 0 | Not started |
| **Total** | **42** | **0** | |
