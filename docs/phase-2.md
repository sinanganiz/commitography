# Phase 2 — CI/CD Integration

**Status:** Planned. Scope defined, implementation not yet broken down.

**Prerequisite:** Phase 1 exit criteria fully met. Phase 2 work does not begin before the MVP is released and has received real-world usage feedback.

**Phase 1.5 relationship:** The Local Web Dashboard and Runner is an
independent local experience. It does not satisfy Phase 2 exit criteria and
does not require the Phase 2 cache. Phase 2 may later provide an optional cache
interface consumed by both the CLI and the local runner.

---

## 1. Purpose

Phase 1 answers "I want to look at this repository right now." Phase 2 answers "I want this dashboard to stay current without anyone remembering to run a command."

The goal is scheduled, automated regeneration inside existing CI systems, publishing to whatever static hosting that CI system already provides. No new infrastructure is introduced. The analysis engine from Phase 1 is reused without modification.

---

## 2. Scope

### In scope

- Ready-to-copy pipeline definitions for GitHub Actions, GitLab CI, and Bitbucket Pipelines.
- A published GitHub Action wrapping the binary, so integration is a few lines rather than a shell script.
- A published Docker image suitable for use as a CI job image, with `git` preinstalled.
- Incremental analysis backed by a local SQLite cache, so scheduled runs on large repositories are cheap.
- Cache invalidation that correctly handles force-pushes and rewritten history.
- Publishing recipes for GitHub Pages, GitLab Pages, and Bitbucket artifact downloads.
- Automatic detection of the CI environment, with a clear failure when the checkout is shallow.
- A machine-readable summary suitable for posting as a build annotation or pull request comment.
- Optional comparison against the previous run, so the output can state what changed since the last analysis.

### Out of scope

- Any hosted service operated by the project.
- Storing history or reports anywhere other than the CI system's own artifact storage.
- Per-pull-request analysis of individual contributions. This remains outside the product's stated positioning.
- Provider API access for pull request or review data. That remains a separate optional enricher, not a Phase 2 deliverable.

---

## 3. Key Design Considerations

### Shallow clones are the dominant failure mode

Every major CI system shallow-clones by default. Phase 1 already detects and refuses this condition. Phase 2 must go further: the shipped templates must configure full clones correctly, and the documentation must lead with this rather than bury it.

| System | Required setting |
|---|---|
| GitHub Actions | `actions/checkout` with `fetch-depth: 0` |
| GitLab CI | `GIT_DEPTH: 0` |
| Bitbucket Pipelines | `clone: depth: full` |
| Azure Pipelines | `fetchDepth: 0` |
| Jenkins | Disable shallow clone in the Git SCM step |

### Incremental analysis

A full re-read of a large repository on every scheduled run is wasteful. The intended approach is a SQLite cache file storing normalized commit records plus the set of tip commits observed at the end of the previous run, allowing subsequent runs to read only new history.

The cache is a performance optimization and never a source of truth. Any inconsistency must cause a full re-read rather than a partial or silently wrong result. Rewritten history, dropped branches, and changed exclusion configuration all invalidate the cache.

The cache file must be safe to commit to a CI cache, safe to delete at any time, and must never be required for correctness.

### Configuration drift

When exclusion rules, identity mappings, or merge handling change, previously cached aggregates no longer correspond to current configuration. The cache must record a fingerprint of the effective configuration and invalidate itself when that fingerprint changes.

### Output size

Repositories with long histories produce large reports. Phase 2 must establish a size budget for published artifacts and a strategy for staying inside it, since CI artifact storage and Pages deployments both have limits.

### Privacy in published output

Publishing to Pages makes output world-readable. Phase 2 documentation must make the privacy defaults explicit at the point of decision, and the shipped templates should default to hashed emails, with anonymization presented as the recommended setting for public publishing.

---

## 4. Deliverables

1. `.github/workflows/` template with scheduled and manual triggers, publishing to GitHub Pages.
2. `.gitlab-ci.yml` template publishing to GitLab Pages.
3. `bitbucket-pipelines.yml` template producing a downloadable artifact.
4. A published composite GitHub Action with documented inputs.
5. A documented Docker image tagged per release.
6. SQLite-backed incremental cache with a documented invalidation contract.
7. A `--summary` output mode emitting a short plain-text or Markdown digest for build annotations.
8. A documentation page covering CI integration, with the shallow-clone warning first.

---

## 5. Exit Criteria

Phase 2 is complete when a team can add Commitography to an existing repository on GitHub, GitLab, or Bitbucket by copying one template file, and receive a correctly generated, automatically updated dashboard on a schedule, with incremental runs demonstrably faster than full runs on a large repository, and with no manual steps beyond the initial copy.

---

## 6. Sequencing Note

Detailed task breakdown for Phase 2 is deliberately deferred. Phase 1 usage will reveal which CI systems matter most to actual users, where the performance ceiling actually sits, and whether incremental analysis is genuinely needed or merely assumed to be. Planning Phase 2 in detail before that feedback exists would encode guesses as requirements.
