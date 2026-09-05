# Commitography — Project Overview

**Status:** Pre-release, design locked
**One-line description:** Turn any git repository's history into a beautiful, self-contained static dashboard with a single command — no API tokens, no server, no data leaving your machine.

---

## 1. What Commitography Is

Commitography reads a git repository's commit history and produces a static HTML dashboard that tells the *story* of that repository: when it is awake, which files it keeps returning to, where knowledge is concentrated, how its code is aging, and what its commit messages reveal about how the team works.

It treats a repository as an organism to be described, not a workforce to be measured.

### Core principles

1. **The git clone is the database.** Everything required for the core feature set already exists in a full clone. No hosting-provider API is needed.
2. **Host-agnostic.** Works identically with GitHub, GitLab, Bitbucket, Gitea, Azure DevOps, and self-hosted git servers, because it never talks to any of them.
3. **Zero configuration to start.** A single binary with no runtime dependency other than `git` itself.
4. **Nothing leaves the machine.** No telemetry, no upload, no cloud component.
5. **Repository-level by default.** Per-contributor breakdowns exist but are opt-in behind an explicit flag.

---

## 2. Positioning

### The gap being filled

Existing tools cluster into two groups:

- **Open-source tools** that are unmaintained, CLI-only, or visually dated.
- **Commercial SaaS platforms** that are expensive, require granting API access to a third party, and are built around individual developer productivity scoring.

Both groups are overwhelmingly GitHub-centric. Teams on Bitbucket, self-hosted GitLab, or Gitea have almost nothing available to them.

| Attribute | Commitography |
|---|---|
| Installation | Single binary, no dependencies beyond `git` |
| Data source | Local git clone only |
| Host support | Any git host, including self-hosted |
| Data privacy | Nothing transmitted anywhere |
| Output | Self-contained static HTML |
| Cost | Free and open source |

### Deliberate stance on productivity metrics

Commitography is explicitly **not** a developer productivity measurement tool. This is a design decision, not an oversight, for three reasons:

1. Per-developer ranking tools are widely and correctly criticized as surveillance tooling, and adopting that framing would undermine community trust.
2. Individual-level measurement carries real legal exposure under GDPR and comparable regimes when applied to employees.
3. Repository-level metrics such as bus factor, change coupling, and code churn are more actionable for engineering leadership anyway, without pitting contributors against one another.

Per-author breakdowns are available behind the `--per-author` flag. The README states plainly why that flag is opt-in.

---

## 3. Project Goals

- Produce a complete, self-contained HTML dashboard from a local git repository with one command.
- Require no configuration for a correct first run on a typical repository.
- Work correctly on any git host without provider-specific code.
- Ship as a single cross-platform binary with the frontend embedded.
- Produce statistically honest output: filter bots, generated files, and vendored dependencies by default, and resolve contributor identities before aggregating anything.
- Expose the computed metrics as a documented, stable JSON artifact that other tools can consume.
- Support anonymized output so results can be published without leaking contributor email addresses.
- Complete a full analysis of a repository with 100,000 commits in under 60 seconds on commodity hardware.
- Provide a shareable "Repo Wrapped" year-in-review mode.
- Grow through a low-friction trial experience: one command, no signup, no account, no server.

---

## 4. Non-Goals

The following are explicitly out of scope. Contributions implementing them will be declined unless the scope is formally revised.

- **Individual developer performance scoring or ranking.** No leaderboards, no productivity scores, no per-person rating.
- **Real-time or continuous monitoring.** Commitography runs on demand or on a schedule and produces a snapshot.
- **Static code analysis or code quality assessment.** Commitography reads history, not source semantics.
- **Issue tracking, project management, or sprint metrics.** No Jira, Linear, or board integration in the core.
- **A mandatory cloud or hosted component.** No feature will require an account or a remote service.
- **Telemetry of any kind.** The tool never phones home.
- **A required database server.** Postgres, MySQL, and similar are never a dependency.
- **LLM dependency in the core.** Commit message classification uses deterministic rules. Any model-assisted classification is an optional plugin, never a requirement.
- **Write access to repositories.** Commitography is strictly read-only and never creates commits, branches, tags, or pushes.
- **Language-specific or framework-specific analysis.** Analysis is based on git history and is language-agnostic.
- **Replacing a git host's native insights.** Commitography is complementary, not a mirror of GitHub Insights.

---

## 5. Architecture

### Three-stage pipeline

```
collect    →  commits.json    (raw, normalized history)
aggregate  →  report.json     (computed metrics)
render     →  out/index.html  (static site with JSON embedded)
```

These stages are strictly separated:

- `collect` output can be cached, so changing metric logic never requires re-reading the full history.
- `report.json` is a product in its own right and is consumable by external tooling.
- `render` is fully replaceable, allowing alternative themes or exporters without touching analysis code.

### Reading git history

History is read in a **single pass** using `git log` with a custom record format, rather than many separate git commands. On large repositories this is the difference between seconds and minutes.

One diff per commit is the invariant; a git invocation per commit is the pathology being avoided. Because `git log --numstat` computes those diffs on a single thread and is what actually bounds a large run, a repository above roughly five thousand commits has its commit list split across a handful of concurrent `git log` processes and reassembled in order. The pass over history is still single; it simply uses the cores that are there. See departure 8 in `phase-1-detailed.md`.

Commitography shells out to the `git` binary rather than linking a git library. Any environment analyzing a repository already has git installed, and this keeps the dependency surface minimal.

### Storage

- No backend service.
- No database server.
- SQLite is used from Phase 2 onward purely as a local incremental cache file, not as an application database.
- The published artifact is JSON.

---

## 6. Phases

Commitography ships in three phases. Each phase is a thin layer around the phase before it; the core analysis engine is written once in Phase 1 and reused unchanged.

### Phase 1 — Local CLI (MVP)

```bash
commitography ./repo -o out/
open out/index.html
```

No server, no database, no configuration required. This is the flagship experience and the entirety of the initial release. Detailed breakdown in `phase-1-detailed.md`.

### Phase 2 — CI/CD integration

Templates and documentation for GitHub Actions, GitLab CI, and Bitbucket Pipelines. Scheduled runs publishing to Pages or build artifacts, plus incremental caching so repeated runs are cheap. Scope described in `phase-2.md`.

### Phase 3 — Server mode

A long-running service managing multiple repositories, performing periodic mirror updates, and serving an aggregated web interface with on-demand refresh. Scope described in `phase-3.md`.

---

## 7. Key Analytical Decisions

These decisions are locked and apply across all phases.

| Decision | Value | Rationale |
|---|---|---|
| Merge commits | Excluded by default | They carry no original change, `--numstat` can double-count their lines, and excluding them makes squash-merge and merge-commit workflows comparable. Configurable. |
| Date source | Author date | Rebase rewrites committer date and squash destroys original timestamps. Author date is closer to "when the work happened". Committer date is still stored. |
| Time zone | Author's local time | Hour-of-day analysis is meaningless in UTC. The commit's own timezone offset is preserved and used. |
| Identity resolution | `.mailmap` first, then config override | Unresolved identities silently corrupt every aggregate. Git's built-in mechanism is used before any custom logic. |
| Generated files | Excluded by default | Without path exclusion, lockfiles and vendored dependencies dominate every line-based metric. |
| Bulk commits | Flagged and excluded above a threshold | Initial imports and vendor drops otherwise distort all distributions. They are still reported separately as a curiosity. |
| Emails in output | Hashed by default | Publishing a dashboard should not publish a team's mailing list. |

---

## 8. Technology

| Layer | Choice |
|---|---|
| Core | Go |
| Git access | `git` binary via subprocess |
| Frontend | TypeScript, custom SVG rendering |
| Distribution | Single static binary with embedded frontend (`embed.FS`) |
| Intermediate cache | SQLite, single file, Phase 2 onward |

**Why Go:** cross-compilation to every supported platform from a single machine, `embed.FS` for shipping the frontend inside the binary, and a distribution ecosystem (Homebrew, Scoop, `go install`) that developer-tool users already expect.

**Why not Grafana as the primary interface:** it requires installing Grafana and configuring a data source, which destroys the "download and run" experience, and its panel model cannot express the narrative visualizations that define the product. An optional exporter for existing Grafana users may be added later.

**Why custom SVG over a charting library:** most of the signature visualizations — the chronotype ring, the code-age strata, the coupling graph — do not map onto standard chart types, and a general-purpose charting library adds bundle weight without covering them.

---

## 9. Metric Catalogue

All metrics below are repository-level and do not rank individuals.

### Temporal

- Chronotype: 24-bucket hour-of-day histogram in author local time
- Day-of-week distribution
- Friday-after-17:00 commit count ("brave deploys")
- Busiest single day on record
- Longest period of silence
- Longest consecutive-day commit streak
- Repository age and first commit date

### Code

- Surviving line ratio from the first commit to today (sampled blame)
- Code age distribution by year
- Most frequently modified files
- Oldest file never modified since creation
- Largest single commit and average commit size
- **Bus factor** per directory
- **Change coupling**: file pairs that consistently change together
- **Churn**: files repeatedly modified within short windows

### Messages

- Conventional Commit prefix distribution, with an explicit confidence indicator
- Very short message counter
- Longest commit message
- Emoji usage
- Revert count
- "fix typo" counter

### Repo Wrapped

A vertically scrolling, card-based year-in-review presentation with its own template and visual language, exportable as shareable images.

```bash
commitography ./repo --wrapped 2026
```

---

## 10. Configuration

Configuration is optional. When present, `.commitography.yml` is read from the analyzed repository root or supplied via `--config`.

```yaml
identities:
  - name: Jane Doe
    emails:
      - jane@example.com
      - jane.doe@corp.example.com

exclude_authors:
  - "dependabot[bot]"
  - "renovate[bot]"

exclude_paths:
  - "**/*.lock"
  - "vendor/**"
  - "dist/**"
  - "**/*.min.js"

outlier_threshold_lines: 10000

count_merges: false
date_source: author        # author | committer
use_mailmap: true

anonymize: false
hash_emails: true

output_dir: ./out
theme: default
```

---

## 11. Known Pitfalls

| Pitfall | Consequence | Mitigation |
|---|---|---|
| Shallow clone | Silently truncated history, all statistics wrong | Detected at startup, refuses to proceed without an explicit override flag |
| Unresolved identities | Contributors split across multiple entries | `.mailmap` plus config override, applied before aggregation |
| Generated and vendored files | Line metrics dominated by machine-written code | Default exclusion list plus `.gitattributes` `linguist-generated` support |
| Non-conventional commit messages | Low classification accuracy | Accuracy percentage is reported in the UI rather than hidden |
| Very large repositories | Slow analysis | Single-pass reading, `--no-blame` fast mode, incremental cache in Phase 2 |

CI systems shallow-clone by default. GitHub Actions requires `fetch-depth: 0`; Bitbucket Pipelines requires `clone: depth: full`. This is the single most common cause of incorrect output and is documented at the top of the README.

---

## 12. License and Governance

- License: MIT
- Contributions accepted via pull request against the documented scope
- Features listed under Non-Goals are out of scope by default; changing that requires revising this document first
