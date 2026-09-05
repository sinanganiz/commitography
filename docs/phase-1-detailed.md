# Phase 1 — Local CLI (MVP): Detailed Implementation Plan

**Goal of Phase 1:** A single cross-platform binary that reads a local git repository and produces a self-contained static HTML dashboard, with no server, no database, and no configuration required for a correct first run.

**Definition of done for Phase 1:** Running `commitography ./repo -o out/` on any full (non-shallow) git repository produces `out/index.html` that opens correctly in a browser with no network access and no adjacent files required, and every number displayed is reproducible from the repository's git history.

This document is written to be executed sequentially. Each task lists its exact deliverables and acceptance criteria. Tasks within a milestone may be parallelized only where explicitly stated.

---

## Status: CLOSED for development — 2026-09-05

**Phase 1 development is complete and the phase is closed.** Nine of the twelve exit criteria are met. The remaining three are not development work: they need a macOS machine, package-manager repositories, and somewhere to host static pages. They are carried forward as the release checklist below rather than holding the phase open.

**This closure does not mean the three open criteria were met.** They were not. Nothing here should be read as clearance to announce the project publicly — the plan's own rule is that criteria 1 through 7 gate any announcement, and criterion 1 is not verified.

### Release checklist — carried forward

| # | Criterion | What it needs | Blocked on |
|---|---|---|---|
| 1 | Runs on macOS | Execute the built binary once: `commitography --version` is enough to prove dyld accepts it | Access to macOS hardware |
| 11 | Installable from two package managers | Tag a release and run goreleaser | `homebrew-tap` and `scoop-bucket` repositories, a `TAP_GITHUB_TOKEN` secret, and goreleaser installed. The `{{ if ne .Os "darwin" }}` ldflag template has never been validated by `goreleaser check` |
| 12 | Three reference dashboards live | Generate, host, and link three public-repository dashboards; replace the two `TODO` blocks in `README.md` (lines 10 and 95) | A choice of three repositories and static hosting |

Criterion 1 is the one that matters most: it is the only open item that could hide a genuine product defect rather than an unfinished errand, and the `LC_UUID` bug is proof that this platform can break silently. Verify it before tagging anything.

---

## Implementation Status

**Last updated:** 2026-09-05 · **Branch:** `main`

The pipeline is implemented end to end and the test suite is green. `commitography ./repo -o out/` produces a working single-file dashboard today. The dashboard and the Wrapped page have been audited in a real browser, and the performance requirement has been measured against a 171,000-commit repository.

| | Count |
|---|---|
| ✅ Done and verified | 28 of 32 tasks |
| ⚠️ Built, not yet verified | 2 tasks — 0.4, 7.2 |
| ❌ Not started | 1 task — 7.3 |
| 🚫 Removed from scope | 1 task — 0.5 (continuous integration) |

### Milestone status

| Task | Status | Notes |
|---|---|---|
| 0.1 Repository structure | ✅ | Tree as specified, plus `internal/cli/`, `internal/gitcmd/`, `Dockerfile`, `.gitattributes` |
| 0.2 Module and dependencies | ✅ | Exactly the three permitted dependencies |
| 0.3 Version package | ✅ | `--version` verified with `-ldflags` injection |
| 0.4 Makefile | ⚠️ | All six targets written; verified by running each target's commands directly, since `make` is not installed on the development machine. `-s -w` is now dropped on Darwin (see below) |
| 0.5 Continuous integration | 🚫 | **Removed on request.** The workflow ran once on 2026-09-05: ubuntu passed, macOS failed on a real product bug (see below), windows did not finish. Verification on Linux and macOS is now manual |
| 1.1 Core data model | ✅ | Round-trip test preserves timezone offsets |
| 1.2 Preflight | ✅ | Shallow, empty and non-repository paths all exit 2 with the exact messages |
| 1.3 git log reader | ✅ | Streaming, control-character separators, C-quoted paths, offsets preserved. Large histories are now read by several concurrent `git log` processes (departure 8) and a subject containing the record separator no longer loses its commit's file changes (departure 9) |
| 1.4 History writer | ✅ | Atomic write, compact JSON, schema-version rejection |
| 1.5 Test fixtures | ✅ | All nine, plus a `coupling/` fixture; deterministic across runs |
| 1.6 Collection test | ✅ | Reconciles against `git rev-list`, `git log --numstat` and `git shortlog` |
| 2.1 Configuration | ✅ | Resolution order, list merging, `exclude_paths: []`, unknown-key warnings |
| 3.1 Identity resolution | ✅ | Reconciles with `git shortlog -sn --all` on the mailmap fixture |
| 3.2 Path exclusion | ✅ | Defaults, `.gitattributes`, negation, case-folding on Windows. Defaults now also carry `**/` forms so nested monorepo lockfiles and dependency trees are excluded (departure 10) |
| 3.3 Merge and bulk handling | ✅ | `ExcludedMerges == 5`, bulk flagged, bots kept out of every metric |
| 4.1 Report schema | ✅ | `docs/report-schema.json` committed and validated in tests |
| 4.2 Temporal metrics | ✅ | Hand-computed expectations for every metric, including local-vs-UTC streaks |
| 4.3 Code metrics | ✅ | Deterministic blame sample; byte-identical output across runs |
| 4.4 Message metrics | ✅ | Three-plus subjects per heuristic rule, rule ordering asserted |
| 4.5 Social metrics | ✅ | Bus factor 1 and 2 cases, coupling support/confidence thresholds |
| 4.6 Notables | ✅ | Every field populated or explicitly null |
| 4.7 Per-author section | ✅ | Absent without the flag; ordered by arrival, not volume |
| 4.8 Privacy transforms | ✅ | No email-shaped string by default; no real name under `--anonymize` |
| 5.1 Frontend toolchain | ✅ | 30.6 KB against the 150 KB cap; no framework, no charting library |
| 5.2 Single-file output | ✅ | Exactly `index.html` + `report.json`; `</script>` break-out test passes |
| 5.3 Dashboard sections | ✅ | Audited in headless Chrome 152 at 360/768/1440 px: no horizontal page overflow, every interactive control reachable by Tab in DOM order, every chart carries `title`/`desc`/`role="img"`, and all text meets WCAG AA in both themes. Two defects found and fixed (see below) |
| 5.4 Repo Wrapped | ✅ | Eight cards render; arrow-key navigation moves between them; the export control produces a real 78–110 KB PNG with a valid signature, entirely offline. One layout defect found and fixed (see below) |
| 6.1 Command surface | ✅ | All 14 flags, exit codes 0/1/2 |
| 6.2 Progress reporting | ✅ | Never on stdout; `--quiet` is silent |
| 7.1 README | ✅ | All 11 required sections; screenshot and demo links are placeholders pending 7.3 |
| 7.2 Release automation | ⚠️ | `.goreleaser.yml` and `Dockerfile` configured; **never executed**, and the tap/bucket repositories do not exist. The new `{{ if ne .Os "darwin" }}` ldflag template is standard goreleaser syntax but is **not machine-validated** — goreleaser is not installed on the development machine, so `goreleaser check` has not been run |
| 7.3 Reference outputs | ❌ | Not started — needs hosting and a choice of three public repositories |

### Deliberate departures from this plan

Each is a case where following the text literally was impossible or wrong. All are marked in the code at the point of departure.

1. **`mostTouchedFiles` field collision (Task 4.3).** The table specifies both `deleted` (a deleted-line count, beside `added`) and `deleted: true` (a boolean flag for files gone from the working tree). One key cannot carry both. `deleted` keeps the line count; the flag ships as `deletedFromHead`.

2. **CI size floor (Task 0.5, step 7).** The 100 KB assertion on `index.html` conflicts with the 150 KB bundle cap in Task 5.1. The bundle builds to 30.6 KB, so a young repository cannot reach 100 KB however correct the output is — this repository's own dashboard is 34 KB. CI asserts >25 KB plus content checks (`<style>` present, payload present, no external `src="http"`), which is what the size check was actually guarding against.

3. **`noise/` fixture composition (Tasks 3.2 and 3.3).** Task 3.2 requires the 40,000-line lockfile to contribute zero lines, while Task 3.3 requires "the lockfile-dominated commit" to be flagged bulk. Since bulk detection runs *after* path exclusion, an excluded lockfile alone yields zero lines and cannot be bulk. The fixture commit therefore also carries a 12,000-line non-excluded data table, so both criteria hold at once and both are genuinely tested.

4. **Payload escaping type (Task 5.2).** `html/template` treats the contents of any `<script>` element as JavaScript regardless of its `type` attribute, so a `template.HTML` payload gets re-encoded as a string literal. The report is passed as `template.JS`; break-out safety comes from explicitly escaping `<`, `>`, `&`, U+2028 and U+2029 before insertion.

5. **Streaming mechanism (Task 1.3, rule 1).** `bufio.Reader.ReadString(0x01)` is used instead of `bufio.Scanner` with a 10 MB buffer. It satisfies the same requirement — output is never held in memory as a single string — without a per-record ceiling that a commit touching tens of thousands of files could exceed.

6. **Vite output directory (Task 5.1).** The bundle is emitted to `internal/render/assets/` rather than `web/dist/`, because `//go:embed` cannot reach outside its own package directory. The built bundle is committed so `go build ./...` succeeds on a clean checkout without Node, as Task 0.2 requires; `.gitattributes` marks it `linguist-generated`.

7. **`gofmt` scope (Task 0.4).** `make lint` runs `gofmt -l cmd internal` rather than `gofmt -l .`, because `testdata/fixtures/` contains generated repositories whose `.go` files are deliberately not valid Go.

8. **Sharded history read (Task 1.3, "a single git invocation").** Above 5,000 commits the history is read by up to 16 concurrent `git log --no-walk --stdin` processes over contiguous slices of `git rev-list --all --date-order`, reassembled in that order.

   Profiling the 171,000-commit CPython repository showed the Go process using about 6 seconds of CPU while the whole run took 130 seconds: 97% of samples were threads blocked on git's pipe, and `git log --numstat` alone accounted for 115 seconds. git computes one diff per commit on a single thread, so no amount of Go optimization could reach the 60-second requirement — the only lever is running several git processes.

   The rule being bent exists to forbid a git invocation *per commit*, which is the actual pathology; the sharded read preserves the one-diff-per-commit property and is still a single pass. Cost is one extra `rev-list` (1.1 s on CPython, no diff work). `TestShardedReadMatchesSingleStream` asserts the sharded and single-stream readers return identical commits, in identical order, on four fixtures. Repositories below the threshold, and any repository whose commits `rev-list` cannot enumerate, still take the original single-invocation path.

9. **Rejoining split records (Task 1.3, rule 2).** The plan assumes the `0x01` record separator "cannot occur in a commit subject". git permits any byte in a commit message, and CPython contains one such subject (`4e45512d`). The record splits in two: the header parses, but its numstat block lands in an orphan chunk that was counted as a parse failure and discarded, silently losing that commit's file changes and printing a warning on every run of that repository. A chunk that does not begin with an object name followed by the field separator is now treated as the tail of the previous record and rejoined, subject and numstat both.

10. **`**/` forms in the default exclusions (Task 3.2, rule 1).** Matching remains literal `doublestar.Match` as the rule requires, but every root-anchored default is now paired with a `**/` form. Anchored patterns alone meant `web/pnpm-lock.yaml` and `packages/api/node_modules/**` were analyzed as ordinary source, so exit criterion 4 held only for repositories with a single package at the root. Only the default list changed; a pattern the user writes is still anchored exactly where they write it.

### Fixed: macOS binaries did not run

The single CI run before the workflow was removed caught a real defect:

```
dyld: missing LC_UUID load command
Abort trap: 6   (exit 134)
```

`-s -w` is now dropped on Darwin in both [`Makefile`](../Makefile) (via `TARGET_GOOS`, which reads `go env GOOS` so cross-compilation is respected) and [`.goreleaser.yml`](../.goreleaser.yml) (via `{{ if ne .Os "darwin" }}`). Linux and Windows binaries stay stripped.

**What was verified, and what was not.** Cross-compiling from the Windows development machine with `CGO_ENABLED=0`, a `-s -w` darwin/arm64 binary *does* carry `LC_UUID`: with the internal linker those flags only remove DWARF and shrink the symbol table (8,827 symbols to 93). So the failure is not reproducible on the cross-compiled path, and the original claim that *every* macOS artifact was unusable was too broad — goreleaser builds with `CGO_ENABLED=0` and would have produced a binary with `LC_UUID`.

The failure belongs to the path CI actually took: `make build` on a macOS host, where cgo is on by default and the Go linker hands `-s` to Apple's `ld`, which drops the load command. That path cannot be exercised from this machine. The flags are dropped on both paths anyway — the size saving is not worth an unrunnable binary, and keeping the two build routes identical means the release artifact matches what a contributor builds locally.

**Criterion 1 therefore remains unverified on macOS.** It needs one run of the built binary on real macOS hardware.

Linux passed the same CI run end to end, including `make lint`, `make fixtures`, `make test`, `make build` and the smoke test. Windows was still running when the workflow was deleted; the Windows path is exercised continuously by the development machine itself.

### Browser audit (Tasks 5.3 and 5.4)

Audited in headless Chrome 152 against dashboards generated from six fixtures — `basic` with `--per-author`, plus `shallow --allow-shallow` for the warning banner, `noise` for bulk commits, `coupling`, `merges` and the project's own repository for the low-confidence flag — so that every section and both themes were actually exercised. Contrast was measured on rendered text, compositing each element's own alpha with every ancestor's `opacity` against its effective background. Style combinations no fixture produces (expected-pair coupling rows, tags, emoji chips) were injected into a real page and measured there rather than left untested.

Three defects were found and fixed:

1. **Wrapped card counters stacked.** `.wrapped-progress` is `position: absolute`, but `.wrapped-card` was not a positioned ancestor, so all eight counters resolved against the viewport and rendered on top of each other in one corner. `.wrapped-card` is now `position: relative`.
2. **Expected coupling pairs failed WCAG AA.** `.coupling-expected { opacity: 0.62 }` faded the whole row, dropping its already-muted text to 3.06:1 in light and 3.44:1 in dark against a 4.5:1 requirement. De-emphasis now comes from a transparent background and a dashed border; only the decorative meter, which is `aria-hidden`, is still dimmed. The "expected pair" tag carries the meaning as text.
3. **Misleading comment in `charts.ts`.** It claimed the data table was hidden with a class "so screen readers can still reach it", while the code uses the `hidden` attribute, which removes it from the accessibility tree. The pattern itself is correct — a disclosure with `aria-expanded` and `aria-controls`, revealed on demand and forced visible in print — so the comment was corrected rather than the code.

Verified and passing: zero network requests with the network disabled (only the `file://` document itself), no uncaught errors, no horizontal page overflow at 360/768/1440 px, all seven interactive controls reachable by Tab in DOM order, `title`/`desc`/`role="img"` on every chart, the show-data toggle revealing its table and updating `aria-expanded`, the theme toggle flipping without touching any storage API, and the Wrapped export producing valid 78–110 KB PNGs offline across four different cards.

### Known gaps worth revisiting

- **Nested `.gitattributes`.** Only the repository-root file is parsed. Walking the tree for nested ones would cost a full scan on every run.
- **Binary detection for blame.** Task 4.3 step 3 mentions `git ls-files --eol`; the implementation uses only the NUL-byte sniff, which is the more reliable of the two signals.
- **Overlapping exclusion reasons.** A merge commit authored by a bot is counted under both `merges` and `bots`, so the two can sum to more than `total`. `total` is computed independently and is authoritative.
- **Progress threshold.** Task 6.2 specifies progress "for runs exceeding 2 seconds"; the implementation emits it for every non-quiet run.
- **Path filtering is now the largest Go-side cost.** Doubling the default pattern list to add the `**/` forms moved the CPython run from 29.2 s to 34.5 s. Results are memoized per distinct path, so the cost scales with distinct paths rather than commits. Well inside budget, but if the list grows further, matching literal basenames with a map before falling back to globbing would remove most of it.
- **The sharded reader is not race-verified.** It is safe by construction — each goroutine writes only its own result slot, and warnings are buffered per shard and replayed after the join, because `OnWarning` belongs to the caller and is not required to be concurrency-safe. But `-race` needs cgo and no C compiler is installed on the development machine, so it was stress-tested (`-count=5` across four fixtures and four shard counts) rather than verified with the detector. Worth one `-race` run wherever a C toolchain is available.

### Exit criteria

| # | Criterion | Status |
|---|---|---|
| 1 | Single-file dashboard on Linux, macOS and Windows | ⚠️ Verified on Windows and Linux. The `LC_UUID` link-flag fix is applied and all twelve release targets cross-compile, but **no macOS binary has been executed** — see above |
| 2 | Counts and line totals reconcile with independent `git` commands | ✅ |
| 3 | Identity resolution reconciles with `git shortlog -sn --all` | ✅ |
| 4 | Default exclusions keep lockfiles and vendored code out | ✅ Now including nested monorepo paths (departure 10) |
| 5 | Shallow clones detected and refused with the specified message | ✅ |
| 6 | `report.json` validates against the committed JSON Schema | ✅ |
| 7 | No plaintext emails; `--anonymize` leaks no real names | ✅ |
| 8 | 100,000 commits in under 60s with `--no-blame` | ✅ **99,754 commits in 12.9 s**; the full 171,014-commit CPython history in 34.5 s (was 164.4 s) |
| 9 | Works offline and meets the Task 5.3 accessibility requirements | ✅ Audited in Chrome 152; two defects found and fixed |
| 10 | `--wrapped` produces a shareable page with client-side PNG export | ✅ Export produces valid PNGs offline; one layout defect found and fixed |
| 11 | Release artifacts published, installable from two package managers | ❌ |
| 12 | Three public reference dashboards live and linked | ❌ |

Criteria 1–7 gate any public announcement. Six of the seven are met outright; criterion 1 needs one run of the built binary on real macOS hardware, which no machine available to this project can currently provide.

**Performance measurements** were taken on the Windows development machine (16 cores) against a full clone of `python/cpython` at 171,014 commits, with `--no-blame`, timing the whole command including rendering:

| Run | Commits | Before | After |
|---|---|---|---|
| Full history | 171,014 | 164.4 s | 34.5 s |
| `--until=2016-01-01` | 99,754 | — | 12.9 s |

The sharded and single-stream readers were diffed field by field over the full CPython report: the only differences were `generatedAt`, `toolVersion`, and the file changes recovered by departure 9 (`+3/-2` in `Lib/functools.py`, and the two warnings that no longer occur).

### After closure

Development on Phase 1 has stopped. The three open criteria are tracked in the release checklist at the top of this document, in the order they should be tackled: macOS verification first, then the release, then the reference dashboards.

One item is worth doing whenever the opportunity appears rather than being scheduled: a single `-race` run of `./internal/collect` wherever a C toolchain exists, to confirm the sharded reader by detector rather than by argument.

Without CI, every cross-platform claim in this document rests on a manual check. Criterion 1 in particular can regress silently — and the `LC_UUID` bug is exactly the kind of regression that only appears on the platform nobody can run. If Phase 2 reintroduces automation, that check is the first thing worth automating.

---

## Conventions Used In This Document

- **MUST / MUST NOT / SHOULD** carry RFC 2119 meaning.
- All file paths are relative to the repository root.
- "The analyzed repository" means the git repository being measured. "The project repository" means the Commitography source tree.
- Go version: **1.22 or later**. The `go.mod` file MUST declare `go 1.22`.
- Module path: `github.com/<owner>/commitography`. The owner segment is filled in at scaffolding time and MUST be consistent across all imports.
- All exported types and functions MUST have doc comments.
- No third-party dependency may be added without being listed in this document. The permitted dependency list is fixed in Task 0.2.

---

## Milestone 0 — Scaffolding

### Task 0.1 — Repository structure

Create the following directory tree. Empty directories MUST contain a `.gitkeep` file.

```
commitography/
├── cmd/
│   └── commitography/
│       └── main.go
├── internal/
│   ├── collect/
│   ├── model/
│   ├── identity/
│   ├── filter/
│   ├── config/
│   ├── aggregate/
│   ├── render/
│   └── version/
├── web/
│   ├── src/
│   ├── index.html
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── testdata/
│   └── fixtures/
├── docs/
│   ├── project-overview.md
│   ├── phase-1-detailed.md
│   ├── phase-2.md
│   └── phase-3.md
├── .github/
│   └── workflows/
│       └── ci.yml
├── .commitography.yml
├── .gitignore
├── .goreleaser.yml
├── LICENSE
├── Makefile
├── README.md
├── go.mod
└── go.sum
```

**Acceptance criteria**
- The tree exists exactly as specified.
- `LICENSE` contains the MIT license text.
- `.gitignore` excludes at minimum: `/out/`, `/dist/`, `web/node_modules/`, `web/dist/`, `*.test`, `commitography` (the built binary).

---

### Task 0.2 — Go module and dependency policy

Initialize the Go module. The following are the **only** permitted third-party Go dependencies for Phase 1:

| Dependency | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI command and flag parsing |
| `gopkg.in/yaml.v3` | Configuration file parsing |
| `github.com/bmatcuk/doublestar/v4` | Glob matching with `**` support |

Everything else MUST use the Go standard library. In particular, JSON encoding uses `encoding/json`, and HTML templating uses `html/template`.

**Acceptance criteria**
- `go build ./...` succeeds with no error.
- `go.mod` lists exactly the three dependencies above (plus their transitive requirements).

---

### Task 0.3 — Version package

Create `internal/version/version.go` exposing:

```go
package version

// Version is the semantic version, injected at build time.
var Version = "dev"

// Commit is the git commit hash of the build, injected at build time.
var Commit = "unknown"

// BuildDate is the RFC 3339 build timestamp, injected at build time.
var BuildDate = "unknown"

// String returns a single-line human-readable version string.
func String() string
```

`String()` MUST return exactly: `commitography <Version> (commit <Commit>, built <BuildDate>)`.

**Acceptance criteria**
- `commitography --version` prints the output of `version.String()` and exits with code 0.

---

### Task 0.4 — Makefile

Create a `Makefile` with these targets and no others:

| Target | Behaviour |
|---|---|
| `build` | Builds the frontend (`make web`) then compiles the binary to `./commitography` with version variables injected via `-ldflags` |
| `web` | Runs `npm ci && npm run build` inside `web/` |
| `test` | Runs `go test ./...` |
| `fixtures` | Runs `testdata/build-fixtures.sh` |
| `lint` | Runs `go vet ./...` and `gofmt -l .`, failing if `gofmt -l .` produces any output |
| `clean` | Removes `./commitography`, `./out`, `web/dist`, `dist/` |

**Acceptance criteria**
- `make build` produces a working binary from a clean checkout.
- `make lint` exits non-zero when a file is not gofmt-formatted.

---

### Task 0.5 — Continuous integration

Create `.github/workflows/ci.yml` that on every push and pull request:

1. Checks out with `fetch-depth: 0`.
2. Sets up Go 1.22 and Node 20.
3. Runs `make lint`.
4. Runs `make fixtures`.
5. Runs `make test`.
6. Runs `make build`.
7. Runs the built binary against the project's own repository and asserts `out/index.html` exists and is larger than 100 KB.

The workflow MUST run on `ubuntu-latest`, `macos-latest`, and `windows-latest`.

**Acceptance criteria**
- CI passes on all three operating systems.

---

## Milestone 1 — Collect

### Task 1.1 — Core data model

Create `internal/model/model.go` with these types. Field names, JSON tags, and types are normative.

```go
package model

import "time"

// FileChange is a single file's line delta within one commit.
type FileChange struct {
    Path     string `json:"path"`
    Added    int    `json:"added"`
    Deleted  int    `json:"deleted"`
    IsBinary bool   `json:"isBinary"`
}

// Commit is one normalized commit record.
type Commit struct {
    Hash                     string       `json:"hash"`
    AuthorName               string       `json:"authorName"`
    AuthorEmail              string       `json:"authorEmail"`
    AuthorDate               time.Time    `json:"authorDate"`
    AuthorTZOffsetMinutes    int          `json:"authorTzOffsetMinutes"`
    CommitterDate            time.Time    `json:"committerDate"`
    CommitterTZOffsetMinutes int          `json:"committerTzOffsetMinutes"`
    Parents                  []string     `json:"parents"`
    IsMerge                  bool         `json:"isMerge"`
    Subject                  string       `json:"subject"`
    Files                    []FileChange `json:"files"`

    // Populated during filtering, not during collection.
    IdentityID string `json:"identityId,omitempty"`
    IsBulk     bool   `json:"isBulk,omitempty"`
    Excluded   bool   `json:"excluded,omitempty"`
}

// RepositoryInfo describes the analyzed repository.
type RepositoryInfo struct {
    Path          string `json:"path"`
    Name          string `json:"name"`
    HeadCommit    string `json:"headCommit"`
    DefaultBranch string `json:"defaultBranch"`
    IsShallow     bool   `json:"isShallow"`
    HasGrafts     bool   `json:"hasGrafts"`
}

// History is the complete collect-stage artifact.
type History struct {
    SchemaVersion int            `json:"schemaVersion"`
    GeneratedAt   time.Time      `json:"generatedAt"`
    ToolVersion   string         `json:"toolVersion"`
    Repository    RepositoryInfo `json:"repository"`
    Commits       []Commit       `json:"commits"`
}
```

`SchemaVersion` for Phase 1 is `1`.

**Normative rules**
- `AuthorDate` and `CommitterDate` MUST retain their original timezone offset. They MUST NOT be normalized to UTC.
- `IsMerge` is `true` if and only if `len(Parents) > 1`.
- `Files` is empty for merge commits and for commits with no file changes.

**Acceptance criteria**
- Marshaling a `History` value and unmarshaling it back produces an identical value, including timezone offsets.

---

### Task 1.2 — Repository validation and preflight

Create `internal/collect/preflight.go`.

```go
// Preflight validates that the given path is analyzable and returns repository metadata.
func Preflight(repoPath string) (model.RepositoryInfo, error)
```

The function MUST perform these checks in this order and fail fast:

1. **git availability.** Run `git --version`. If the command is not found, return an error with the message: `git executable not found in PATH; commitography requires git to be installed`.
2. **git version.** Parse the version. If it is older than `2.22.0`, return an error naming the detected version and the minimum required version.
3. **Path is a repository.** Run `git -C <path> rev-parse --git-dir`. On failure return: `<path> is not a git repository`.
4. **Shallow check.** Run `git -C <path> rev-parse --is-shallow-repository`. If the output is `true`, set `IsShallow = true`.
5. **Graft check.** Determine whether `<git-dir>/shallow` or `<git-dir>/info/grafts` exists; set `HasGrafts` accordingly.
6. **Empty repository check.** Run `git -C <path> rev-parse HEAD`. If it fails, return: `repository has no commits`.
7. **Head and default branch.** Populate `HeadCommit` from the previous step. Determine `DefaultBranch` by reading `git -C <path> symbolic-ref --short HEAD`; if that fails (detached HEAD), set it to the empty string.
8. **Name.** Set `Name` to the base name of the absolute repository path, with a trailing `.git` suffix removed if present.

**Shallow repository handling (normative)**

If `IsShallow` is true, the CLI MUST refuse to proceed and exit with code `2` and this exact message on stderr:

```
Error: this repository is a shallow clone, so its history is incomplete and all statistics would be wrong.

Fix it with:
  git fetch --unshallow

In CI, configure a full clone:
  GitHub Actions      -> actions/checkout with fetch-depth: 0
  GitLab CI           -> GIT_DEPTH: 0
  Bitbucket Pipelines -> clone: depth: full

To analyze anyway and accept incorrect results, pass --allow-shallow.
```

When `--allow-shallow` is passed, analysis proceeds and the resulting dashboard MUST display a persistent warning banner.

**Acceptance criteria**
- A shallow fixture repository triggers the exact error above and exit code 2.
- A repository with zero commits produces the empty-repository error and exit code 2.
- A non-repository path produces the not-a-repository error and exit code 2.

---

### Task 1.3 — Single-pass git log reader

Create `internal/collect/gitlog.go`.

```go
// Options controls how history is read.
type Options struct {
    RepoPath   string
    UseMailmap bool
    Since      string // passed through to git log --since, empty means no bound
    Until      string // passed through to git log --until, empty means no bound
}

// Collect reads the full history in a single git invocation.
func Collect(opts Options) (*model.History, error)
```

**Normative git invocation**

```
git -C <RepoPath> log --all --numstat --no-renames --date-order
    [--use-mailmap]
    [--since=<Since>] [--until=<Until>]
    --pretty=format:%x01%H%x1f%aN%x1f%aE%x1f%aI%x1f%cI%x1f%P%x1f%s
```

- `--use-mailmap` is included only when `Options.UseMailmap` is true.
- `%x01` (ASCII 0x01) is the record separator. `%x1f` (ASCII 0x1F) is the field separator.
- These control characters MUST be used. Commas, pipes, and other printable delimiters MUST NOT be used, because they occur in commit subjects.

**Normative parsing rules**

1. Read stdout as a stream. The implementation MUST NOT buffer the entire output in memory as a single string. Use `bufio.Scanner` with a custom split function on `0x01`, or an equivalent streaming approach, with a buffer capacity of at least 10 MB per record.
2. The first field of each record, up to the first `0x1f`, is the commit hash. There are exactly 7 fields per record header, in this order: hash, author name, author email, author date (ISO 8601 strict), committer date (ISO 8601 strict), parent hashes, subject.
3. `%P` yields parent hashes separated by single spaces. An empty value means a root commit.
4. The subject field is followed by a newline and then zero or more numstat lines until the next `0x01` or end of stream.
5. Each numstat line is `<added>\t<deleted>\t<path>`. When `<added>` and `<deleted>` are both the literal `-`, the file is binary: set `IsBinary = true`, `Added = 0`, `Deleted = 0`.
6. Blank lines between the subject and the numstat block MUST be skipped.
7. Parse dates with `time.Parse(time.RFC3339, ...)`. Extract the offset via `t.Zone()` and store it in minutes as a signed integer.
8. A record whose header cannot be parsed MUST NOT abort the run. It MUST be counted and reported as a parse warning at the end of collection. If more than 1% of records fail to parse, `Collect` MUST return an error.

**Path handling**

`git log` quotes paths containing non-ASCII or special characters when `core.quotePath` is enabled. The implementation MUST run `git -C <path> config core.quotePath false` equivalence by passing `-c core.quotePath=false` on the command line, so paths are emitted raw and UTF-8. Paths MUST be stored exactly as git emits them, with forward slashes on all platforms.

**Acceptance criteria**
- On the fixture repository from Task 1.6, `len(History.Commits)` equals the output of `git rev-list --all --count`.
- Sum of commits per identity matches `git shortlog -sn --all` exactly, after mailmap is applied.
- A commit containing a file whose path includes a space, a Unicode character, and a quotation mark is parsed with the path intact.
- A binary file change is recorded with `IsBinary = true` and zero line counts.

---

### Task 1.4 — History artifact writer

Create `internal/collect/writer.go`.

```go
// WriteHistory serializes a History to the given path as JSON.
func WriteHistory(h *model.History, path string) error

// ReadHistory loads a previously written History artifact.
func ReadHistory(path string) (*model.History, error)
```

- Output MUST be written atomically: write to `<path>.tmp` then rename.
- Output MUST NOT be indented (compact JSON) to keep artifact size manageable.
- `ReadHistory` MUST reject a file whose `schemaVersion` differs from the current version, with the message: `history artifact schema version <n> is not supported by this version of commitography`.

**Acceptance criteria**
- Round-trip of a 10,000-commit history preserves all fields.
- Reading a file with `schemaVersion: 999` fails with the specified message.

---

### Task 1.5 — Test fixtures

Create `testdata/build-fixtures.sh`, a POSIX shell script that constructs deterministic git repositories under `testdata/fixtures/`. All commits MUST use fixed timestamps via `GIT_AUTHOR_DATE` and `GIT_COMMITTER_DATE` so that output is byte-reproducible.

Required fixtures:

| Fixture | Contents |
|---|---|
| `basic/` | 50 commits, 3 authors, 2 of whom share an identity across two emails, spread across 6 months, mixed hours and weekdays, at least 3 timezone offsets |
| `mailmap/` | Same as basic plus a `.mailmap` file mapping the duplicate identity |
| `merges/` | Contains at least 5 merge commits and 5 squash-style single-parent commits |
| `binary/` | Contains binary files and files with unusual path characters |
| `noise/` | Contains a `package-lock.json` with a 40,000-line commit, a `vendor/` tree, and a `dist/` bundle |
| `bots/` | Contains commits authored by `dependabot[bot]` and `renovate[bot]` |
| `shallow/` | A shallow clone of `basic/` created with `--depth 5` |
| `empty/` | An initialized repository with zero commits |
| `single/` | A repository with exactly one commit |

The script MUST be idempotent: running it twice produces identical repositories.

**Acceptance criteria**
- `make fixtures` completes on Linux and macOS.
- Running the script twice yields identical commit hashes.

---

### Task 1.6 — Collection correctness test

Create `internal/collect/collect_test.go`.

Required assertions against the `basic/` fixture:

1. Commit count matches `git rev-list --all --count`.
2. Total added lines matches the sum computed independently by a separate `git log --numstat` invocation in the test.
3. Every commit's `AuthorTZOffsetMinutes` matches the offset in the raw `%aI` string.
4. The root commit has `len(Parents) == 0`.
5. Merge commits in the `merges/` fixture have `IsMerge == true` and `len(Files) == 0`.
6. The `single/` fixture produces exactly one commit and does not panic.

**Acceptance criteria**
- All assertions pass. No test may depend on network access.

---

## Milestone 2 — Configuration

### Task 2.1 — Configuration schema and loading

Create `internal/config/config.go`.

```go
type Identity struct {
    Name   string   `yaml:"name"`
    Emails []string `yaml:"emails"`
}

type Config struct {
    Identities            []Identity `yaml:"identities"`
    ExcludeAuthors        []string   `yaml:"exclude_authors"`
    ExcludePaths          []string   `yaml:"exclude_paths"`
    OutlierThresholdLines int        `yaml:"outlier_threshold_lines"`
    CountMerges           bool       `yaml:"count_merges"`
    DateSource            string     `yaml:"date_source"`
    UseMailmap            bool       `yaml:"use_mailmap"`
    Anonymize             bool       `yaml:"anonymize"`
    HashEmails            bool       `yaml:"hash_emails"`
    OutputDir             string     `yaml:"output_dir"`
    Theme                 string     `yaml:"theme"`
}

// Default returns the built-in configuration.
func Default() Config

// Load resolves configuration from defaults, file, and flag overrides.
func Load(explicitPath string, repoPath string) (Config, error)
```

**Normative defaults**

```go
OutlierThresholdLines: 10000
CountMerges:           false
DateSource:            "author"
UseMailmap:            true
Anonymize:             false
HashEmails:            true
OutputDir:             "./out"
Theme:                 "default"
```

**Default exclude_authors** (merged with, not replaced by, user values):

```
dependabot[bot]
renovate[bot]
github-actions[bot]
gitlab-bot
imgbot[bot]
allcontributors[bot]
snyk-bot
greenkeeper[bot]
semantic-release-bot
```

In addition, any author name matching the regular expression `\[bot\]$` or any author email ending in `@users.noreply.github.com` **and** whose name matches `\[bot\]$` MUST be excluded.

**Default exclude_paths** (merged with, not replaced by, user values):

```
**/*.lock
package-lock.json
yarn.lock
pnpm-lock.yaml
Gemfile.lock
composer.lock
go.sum
Cargo.lock
poetry.lock
vendor/**
node_modules/**
third_party/**
Pods/**
dist/**
build/**
out/**
target/**
**/*.min.js
**/*.min.css
**/*.map
**/*.generated.*
**/*.pb.go
**/*_pb2.py
**/migrations/**
**/*.snap
```

**Resolution order (normative, later wins)**

1. Built-in defaults.
2. `.commitography.yml` in the analyzed repository root, if present.
3. File given by `--config`, if present. This replaces step 2 rather than merging with it.
4. Command-line flags.

For `ExcludeAuthors` and `ExcludePaths`, user-supplied values are **appended to** the defaults. To suppress defaults entirely, the user sets the key to an explicit empty list (`exclude_paths: []`), which MUST be distinguishable from the key being absent.

**Validation**

- `DateSource` MUST be `author` or `committer`; anything else is an error naming the offending value.
- `OutlierThresholdLines` MUST be greater than 0.
- `Theme` MUST be `default` for Phase 1; other values are an error.
- Unknown YAML keys MUST produce a warning on stderr but MUST NOT fail the run.

**Acceptance criteria**
- Loading with no config file yields exactly `Default()` with default lists populated.
- A config with `exclude_paths: []` results in zero path exclusions.
- A config with `date_source: invalid` fails with a descriptive error and exit code 2.

---

## Milestone 3 — Identity and Filtering

### Task 3.1 — Identity resolution

Create `internal/identity/identity.go`.

```go
type Identity struct {
    ID          string   // canonical key
    DisplayName string
    Emails      []string
    IsBot       bool
}

type Resolver struct{ /* ... */ }

// NewResolver builds a resolver from config and the observed commit set.
func NewResolver(cfg config.Config, commits []model.Commit) *Resolver

// Resolve returns the canonical identity ID for a commit's author.
func (r *Resolver) Resolve(name, email string) string

// Identities returns all resolved identities, sorted by descending commit count.
func (r *Resolver) Identities() []Identity
```

**Normative resolution algorithm**

1. Git's `--use-mailmap` has already been applied at collection time when `UseMailmap` is true. The resolver operates on the resulting names and emails.
2. Normalize every email by trimming whitespace and lowercasing.
3. Build a map from normalized email to canonical identity ID using `config.Identities`. For each configured identity, the canonical ID is the **first** email in its `emails` list, lowercased. Every email in the list maps to that ID.
4. For any email not covered by configuration, the canonical ID is the normalized email itself.
5. `DisplayName` for a configured identity is its `name` field. For an unconfigured identity it is the author name of that identity's **most recent** commit.
6. `IsBot` is true when the identity matches any entry in `ExcludeAuthors` (compared against both display name and any of its emails, case-insensitively) or matches the bot regular expressions from Task 2.1.

**Acceptance criteria**
- On the `mailmap/` fixture, the number of resolved identities matches `git shortlog -sn --all` line count with mailmap applied.
- On the `basic/` fixture with a config merging two emails, the merged identity's commit count equals the sum of the two source counts.
- On the `bots/` fixture, both bot identities have `IsBot == true`.

---

### Task 3.2 — Path exclusion

Create `internal/filter/paths.go`.

```go
type PathFilter struct{ /* ... */ }

// NewPathFilter compiles the exclusion patterns and reads .gitattributes.
func NewPathFilter(cfg config.Config, repoPath string) (*PathFilter, error)

// Excluded reports whether a path should be omitted from line-based metrics.
func (f *PathFilter) Excluded(path string) bool
```

**Normative rules**

1. Patterns are matched with `doublestar.Match` against the forward-slash path as emitted by git.
2. `.gitattributes` in the analyzed repository MUST be parsed. Any pattern carrying the `linguist-generated` attribute (with value `true` or with no value) is added to the exclusion set. Negated forms (`-linguist-generated`) MUST remove a path from exclusion.
3. Matching MUST be case-insensitive on Windows and case-sensitive elsewhere, mirroring git's own default behaviour.
4. Excluded files still count toward the commit's existence. They are removed only from line-count, file-touch, coupling, and churn metrics.

**Acceptance criteria**
- On the `noise/` fixture, the 40,000-line lockfile commit contributes zero added lines to aggregate totals.
- A `.gitattributes` entry `api/*.go linguist-generated=true` causes those files to be excluded.

---

### Task 3.3 — Merge and bulk-commit handling

Create `internal/filter/commits.go`.

```go
// Apply annotates commits with identity, exclusion, and bulk flags.
func Apply(commits []model.Commit, cfg config.Config, r *identity.Resolver, pf *PathFilter) Result

type Result struct {
    Commits        []model.Commit
    TotalCommits   int
    ExcludedMerges int
    ExcludedBots   int
    BulkCommits    []BulkCommit
}

type BulkCommit struct {
    Hash         string
    Subject      string
    Date         time.Time
    LinesChanged int
}
```

**Normative rules**

1. **Merges.** When `CountMerges` is false, every commit with `IsMerge == true` is marked `Excluded = true`. The count is reported as `ExcludedMerges` and surfaced in the dashboard as a standalone statistic.
2. **Bots.** Commits whose resolved identity has `IsBot == true` are marked `Excluded = true` and counted in `ExcludedBots`.
3. **Bulk commits.** After path exclusion is applied, compute `linesChanged = sum(added + deleted)` over non-excluded, non-binary files. If `linesChanged > OutlierThresholdLines`, mark the commit `IsBulk = true`. Bulk commits are excluded from all line-based and coupling metrics but **remain included** in commit-count and temporal metrics, and are listed separately in the dashboard as notable events.
4. **File count guard for coupling.** A commit touching more than 50 non-excluded files MUST be skipped by the coupling metric specifically, independent of the bulk flag. This threshold is a compile-time constant named `couplingMaxFilesPerCommit`.

**Acceptance criteria**
- On the `merges/` fixture with defaults, `ExcludedMerges == 5`.
- On the `noise/` fixture, the lockfile-dominated commit has `IsBulk == true`.
- On the `bots/` fixture, no bot commit appears in any aggregate metric.

---

## Milestone 4 — Aggregation

All metrics operate on the filtered commit set produced by Task 3.3 unless a task states otherwise. "Local time" always means the author's local time reconstructed from the stored timezone offset.

### Task 4.1 — Report schema

Create `internal/aggregate/report.go` defining the complete `Report` type serialized to `report.json`.

```go
type Report struct {
    SchemaVersion int              `json:"schemaVersion"`
    GeneratedAt   time.Time        `json:"generatedAt"`
    ToolVersion   string           `json:"toolVersion"`
    Repository    RepositorySummary `json:"repository"`
    Temporal      TemporalMetrics  `json:"temporal"`
    Code          CodeMetrics      `json:"code"`
    Messages      MessageMetrics   `json:"messages"`
    Social        SocialMetrics    `json:"social"`
    Notables      Notables         `json:"notables"`
    PerAuthor     *PerAuthor       `json:"perAuthor,omitempty"`
    Warnings      []string         `json:"warnings"`
}
```

`SchemaVersion` for Phase 1 is `1`. Once released, fields MUST NOT be removed or repurposed within a schema version.

`RepositorySummary` MUST include: name, first commit date, last commit date, age in days, total commits analyzed, total commits excluded with a per-reason breakdown, contributor count, total files currently tracked, and total lines currently tracked.

**Acceptance criteria**
- `report.json` validates against a JSON Schema file committed at `docs/report-schema.json`.

---

### Task 4.2 — Temporal metrics

Create `internal/aggregate/temporal.go`.

| Metric | JSON field | Exact definition |
|---|---|---|
| Hour histogram | `hourHistogram` | Array of exactly 24 integers. Index *i* is the count of commits whose author-local hour equals *i*. |
| Weekday histogram | `weekdayHistogram` | Array of exactly 7 integers. Index 0 is Monday, index 6 is Sunday, using author-local date. |
| Hour-by-weekday grid | `hourWeekdayGrid` | 7×24 integer matrix, `[weekday][hour]`, same conventions as above. |
| Brave deploys | `braveDeploys` | Count of commits where author-local weekday is Friday and author-local hour is >= 17. |
| Night owl ratio | `nightOwlRatio` | Fraction of commits with author-local hour >= 22 or < 6, as a float in [0,1] rounded to 4 decimal places. |
| Busiest day | `busiestDay` | The author-local calendar date with the highest commit count. Object with `date` (YYYY-MM-DD) and `count`. Ties broken by earliest date. |
| Longest streak | `longestStreak` | The maximum number of consecutive author-local calendar dates each having >= 1 commit. Object with `days`, `startDate`, `endDate`. |
| Longest silence | `longestSilence` | The maximum gap in whole days between two chronologically adjacent commits by author date. Object with `days`, `startDate`, `endDate`. |
| Commits per month | `commitsPerMonth` | Array of objects `{month: "YYYY-MM", count: n}`, contiguous from first to last month with zero-filled gaps. |
| First / last commit | `firstCommit`, `lastCommit` | RFC 3339 timestamps preserving original offsets. |

**Normative edge cases**
- A repository spanning a single day yields `longestStreak.days == 1` and `longestSilence.days == 0`.
- Streak computation MUST use author-local dates, not UTC dates, so a commit at 23:00 UTC+03 counts toward that local day.

**Acceptance criteria**
- Unit tests using a synthetic commit list with hand-computed expected values for every metric above.
- `sum(hourHistogram) == sum(weekdayHistogram) == analyzed commit count`.

---

### Task 4.3 — Code metrics

Create `internal/aggregate/code.go`.

| Metric | JSON field | Exact definition |
|---|---|---|
| Most touched files | `mostTouchedFiles` | Top 25 non-excluded paths by number of commits touching them. Objects with `path`, `commits`, `added`, `deleted`. Files deleted from the working tree are still counted and flagged `deleted: true`. |
| Largest commit | `largestCommit` | The non-bulk commit with the greatest `added + deleted` over non-excluded files. |
| Average commit size | `averageCommitSize` | Mean of `added + deleted` over non-excluded, non-bulk, non-merge commits, rounded to 1 decimal place. |
| Median commit size | `medianCommitSize` | Median of the same population. |
| Line totals | `totalAdded`, `totalDeleted` | Sums over non-excluded, non-bulk files. |
| Oldest untouched file | `oldestUntouchedFile` | Among files currently present in HEAD, the one whose most recent modifying commit is oldest. Object with `path` and `lastModified`. |
| File type distribution | `fileTypeDistribution` | Top 15 file extensions by number of file-changes, as `{extension, changes, added, deleted}`. Files with no extension are grouped under `(none)`. |
| Code age | `codeAge` | Array of `{year, lines}` derived from the blame sample described below. |
| Surviving ratio | `survivingFromFirstYear` | Fraction of sampled lines whose last-modifying commit falls in the repository's first calendar year of activity. |

**Blame sampling (normative)**

Blame is expensive and MUST be sampled deterministically:

1. List files in HEAD with `git ls-tree -r --name-only HEAD`.
2. Remove paths matching the path filter.
3. Remove files detected as binary by `git ls-files --eol` reporting no text attribute, or whose content contains a NUL byte in the first 8000 bytes.
4. Sort remaining paths lexicographically for determinism.
5. If the count exceeds 300, select 300 paths by taking every *n*-th path where `n = floor(count / 300)`, starting at index 0. This must be reproducible across runs.
6. For each selected path, run `git blame --line-porcelain -w -M HEAD -- <path>` and aggregate line counts by the author-date year of the blamed commit.
7. If `--no-blame` is passed, skip this task entirely and set `codeAge` to an empty array and `survivingFromFirstYear` to `null`. The dashboard MUST hide the corresponding sections rather than showing zeros.

The report MUST record `codeAgeSampledFiles` and `codeAgeTotalFiles` so the UI can state the sampling ratio.

**Acceptance criteria**
- Running twice on the same repository produces byte-identical `codeAge` output.
- `--no-blame` reduces runtime on the `basic/` fixture and produces `codeAge: []`.

---

### Task 4.4 — Message metrics

Create `internal/aggregate/messages.go`.

**Conventional Commit detection (normative)**

A subject is a Conventional Commit if it matches:

```
^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([^)]*\))?(!)?: .+
```

The type is the first capture group, lowercased.

**Heuristic fallback (normative)**

When a subject does not match, classify it by testing these rules **in order** and taking the first match. All matching is case-insensitive against the subject.

| Order | Pattern | Category |
|---|---|---|
| 1 | `^revert\b` or `^revert "` | `revert` |
| 2 | `\b(merge branch|merge pull request|merge remote-tracking)\b` | `merge` |
| 3 | `\b(fix|fixes|fixed|bugfix|hotfix|patch|resolve[sd]?|correct)\b` | `fix` |
| 4 | `\b(add|adds|added|implement|introduce|create|new feature|feature)\b` | `feat` |
| 5 | `\b(refactor|rename|restructure|cleanup|clean up|simplify|extract)\b` | `refactor` |
| 6 | `\b(test|tests|testing|spec|specs)\b` | `test` |
| 7 | `\b(doc|docs|documentation|readme|comment)\b` | `docs` |
| 8 | `\b(bump|upgrade|update dependenc|dependency|deps)\b` | `chore` |
| 9 | `\b(style|format|formatting|lint|prettier|gofmt)\b` | `style` |
| 10 | `\b(ci|pipeline|workflow|build|release|deploy)\b` | `ci` |
| 11 | (no match) | `other` |

**Confidence reporting**

`conventionalRatio` is the fraction of analyzed commits matching the strict Conventional Commit pattern. The dashboard MUST display this value and MUST label the classification section as low confidence when it is below `0.30`.

**Other message metrics**

| Metric | JSON field | Exact definition |
|---|---|---|
| Type distribution | `typeDistribution` | Map from category to count, covering all commits. |
| Short messages | `shortMessages` | Count of subjects whose trimmed length is <= 5 characters, plus subjects exactly matching (case-insensitively) any of: `wip`, `fix`, `fixes`, `update`, `updates`, `changes`, `stuff`, `oops`, `asdf`, `.`, `..`, `...`. |
| Longest subject | `longestSubject` | Object with `hash`, `length`, `subject` truncated to 200 characters for display. |
| Average subject length | `averageSubjectLength` | Mean character length, 1 decimal place. |
| Emoji usage | `emojiCommits`, `topEmoji` | Count of subjects containing at least one code point in the Unicode ranges U+1F300–U+1FAFF, U+2600–U+27BF, U+FE0F, U+2190–U+21FF combined with U+FE0F. `topEmoji` is the 10 most frequent emoji with counts. |
| Reverts | `revertCount` | Count of subjects matching `^revert\b` case-insensitively. |
| Typo fixes | `typoFixCount` | Count of subjects matching `\btypos?\b` case-insensitively. |
| Word cloud | `topWords` | Top 40 words after lowercasing, stripping punctuation, and removing a committed stopword list at `internal/aggregate/stopwords.go`. Words shorter than 3 characters are dropped. |

**Acceptance criteria**
- Table-driven unit tests covering at least three subjects per heuristic rule, including at least one case that must fall through to a later rule.
- A subject matching both `fix` and `add` keywords is classified as `fix`, per rule ordering.

---

### Task 4.5 — Social metrics

Create `internal/aggregate/social.go`.

**Bus factor (normative)**

For a given path scope *S* (a directory, or the whole repository):

1. Count, per identity, the number of non-excluded, non-bulk commits that touched at least one non-excluded file under *S*.
2. Sort identities by that count, descending. Break ties by identity ID ascending, for determinism.
3. The bus factor is the smallest *k* such that the top *k* identities account for at least 50% of the total count.

Compute this for the repository as a whole and for every directory at depth 1 and depth 2 from the repository root that contains at least 10 commits' worth of activity. Report as `busFactor` (repository) and `directoryBusFactor` (array of `{path, busFactor, contributors, commits}`).

**Change coupling (normative)**

1. Consider only non-excluded, non-bulk commits touching between 2 and `couplingMaxFilesPerCommit` (50) non-excluded files.
2. For each such commit, form all unordered file pairs.
3. For a pair (A, B): `support` is the number of commits containing both; `confidence` is `support / min(changes(A), changes(B))` where `changes(X)` counts commits touching X within the same filtered population.
4. Report pairs with `support >= 5` **and** `confidence >= 0.5`, sorted by `support` descending then `confidence` descending, capped at 50 pairs.
5. Pairs where both files share the same basename without extension (for example `Foo.ts` and `Foo.test.ts`) MUST still be reported but flagged `expected: true` so the UI can de-emphasize them.

Memory guard: if the pair map would exceed 5,000,000 entries, the implementation MUST progressively drop pairs with `support == 1` and record a warning in `Report.Warnings`.

**Churn (normative)**

A file is a churn hotspot if it received at least 5 non-excluded, non-bulk commits within any rolling 30-day window (measured on author dates). Report the top 25 hotspots as `{path, maxCommitsInWindow, windowStart, totalCommits, added, deleted}`, sorted by `maxCommitsInWindow` descending.

**Knowledge concentration (normative, repository level only)**

For each depth-1 directory, report the share of commits attributable to its single largest contributor as a float in [0,1], without naming that contributor unless `--per-author` is set. Field: `knowledgeConcentration`.

**Acceptance criteria**
- Bus factor of a synthetic repository where one identity authored 80% of commits is exactly 1.
- Bus factor of a synthetic repository with four identities at 25% each is 2.
- Coupling detection finds a deliberately coupled file pair in a synthetic fixture and excludes a pair appearing in only 4 commits.

---

### Task 4.6 — Notables

Create `internal/aggregate/notables.go`. This section holds the "fun fact" content.

Required fields:

| Field | Definition |
|---|---|
| `bulkCommits` | The bulk commits identified in Task 3.3, capped at 10, most recent first. |
| `latestNightCommit` | The commit with the author-local time closest to 03:00, restricted to hours 00:00–05:59. |
| `earliestMorningCommit` | The commit with the earliest author-local time in the range 04:00–07:59. |
| `weekendRatio` | Fraction of commits on author-local Saturday or Sunday. |
| `holidayCommits` | Count of commits on December 25 and January 1 by author-local date. |
| `firstCommitSubject` | Subject of the root commit, or of the earliest commit if multiple roots exist. |
| `mergeCount` | Total merge commits present in the repository, reported regardless of `count_merges`. |
| `timezoneSpread` | Distinct timezone offsets observed, with commit counts per offset. |

**Acceptance criteria**
- Every notable is either populated or explicitly `null`; the renderer must never display a placeholder for missing data.

---

### Task 4.7 — Per-author section

Create `internal/aggregate/perauthor.go`. This section is populated **only** when `--per-author` is passed; otherwise `Report.PerAuthor` is `nil` and the key is absent from the JSON.

Per identity: commit count, added lines, deleted lines, files touched, active hour histogram (24 buckets), first commit date, last commit date, active day span.

The output MUST be sorted by first commit date ascending, **not** by commit count, so the presentation does not read as a ranking.

**Acceptance criteria**
- Without the flag, `report.json` contains no `perAuthor` key.
- With the flag, the sum of per-identity commit counts equals the analyzed commit total.

---

### Task 4.8 — Privacy transforms

Create `internal/aggregate/privacy.go`.

```go
// ApplyPrivacy rewrites identity-bearing fields according to configuration.
func ApplyPrivacy(r *Report, cfg config.Config)
```

**Normative rules**

1. When `HashEmails` is true (the default), every email in the report is replaced by the first 16 hexadecimal characters of the SHA-256 digest of the lowercased, trimmed email. The raw email MUST NOT appear anywhere in the output.
2. When `Anonymize` is true, display names are replaced with stable pseudonyms. Pseudonyms are assigned by sorting identities by first commit date ascending and labeling them `Contributor A`, `Contributor B`, …, continuing to `Contributor AA` after `Contributor Z`. Emails are removed entirely rather than hashed.
3. Privacy transforms are applied **after** all aggregation and **before** serialization, so no metric depends on the transformed values.
4. When `Anonymize` is true, `Notables.firstCommitSubject`, `longestSubject`, and `topWords` MUST still be included, since commit message content is not identity data. This is stated explicitly so implementers do not over-redact.

**Acceptance criteria**
- With defaults, grepping `report.json` for `@` finds no email-shaped string.
- With `--anonymize`, no original author name appears anywhere in `out/`.

---

## Milestone 5 — Rendering

### Task 5.1 — Frontend toolchain

Set up `web/` with Vite and TypeScript. No UI framework and no charting library are permitted in Phase 1. Visualizations are produced as SVG built by hand-written TypeScript.

Constraints:

- Output MUST be a single JS bundle and a single CSS bundle.
- Total bundle size MUST be under 150 KB uncompressed.
- No web fonts loaded from the network. System font stack only.
- No runtime network requests of any kind. The page MUST function with the network disabled.

**Acceptance criteria**
- `npm run build` emits to `web/dist/`.
- Opening the built page with devtools offline mode enabled shows zero failed requests.

---

### Task 5.2 — Single-file HTML output

Create `internal/render/render.go`.

```go
// Render writes the complete dashboard to the output directory.
func Render(r *aggregate.Report, outputDir string) error
```

**Normative rules**

1. The frontend bundle is embedded into the Go binary with `//go:embed`.
2. `Render` MUST produce **exactly one file**: `<outputDir>/index.html`. CSS, JS, and the report JSON are all inlined into that file. There are no sibling assets.
3. The report is inlined as `<script type="application/json" id="commitography-data">…</script>`. Its content MUST be escaped so that a commit subject containing `</script>` cannot break out of the tag. Specifically, `<` MUST be written as `\u003c` in the embedded JSON.
4. `Render` MUST also write `<outputDir>/report.json` as a separate machine-readable artifact. This is the only exception to rule 2 and is not referenced by the HTML.
5. The output directory is created if absent. Existing `index.html` and `report.json` are overwritten without prompting. No other file is ever deleted.

**Acceptance criteria**
- `out/index.html` opens correctly after being moved to an unrelated directory.
- A fixture containing a commit subject with `</script>` renders without breaking the page.

---

### Task 5.3 — Dashboard sections

The dashboard is a single scrolling page with the following sections, in this order. Every section that lacks data MUST be omitted entirely rather than rendered empty.

1. **Header.** Repository name, analysis date, commit count, contributor count, date range, tool version.
2. **Warning banner.** Rendered only when `Report.Warnings` is non-empty or the repository was shallow.
3. **Pulse.** Commits-per-month area chart across the full history.
4. **Chronotype.** 24-hour radial histogram plus the 7×24 grid heatmap. Includes the night-owl ratio and brave-deploy count as callouts.
5. **Rhythm.** Weekday distribution, longest streak, longest silence, busiest day.
6. **Code age.** Stacked area of lines by year, with the sampling ratio stated in the section subheading.
7. **Hotspots.** Most touched files, churn hotspots.
8. **Coupling.** Force-free layout: an ordered list of file pairs with a support/confidence bar, expected pairs de-emphasized.
9. **Bus factor.** Repository figure plus a directory table.
10. **Messages.** Type distribution, conventional ratio with confidence label, word cloud, emoji, short-message and typo counters.
11. **Notables.** Card grid of fun facts.
12. **Contributors.** Rendered only when `perAuthor` is present.
13. **Footer.** Link to the project, license, and a one-line statement that no data left the machine.

**Accessibility requirements**
- Every SVG visualization MUST have a `<title>` and a `<desc>`.
- Every visualization MUST be accompanied by an accessible data table, visually hidden by default and revealed by a per-section "show data" toggle.
- Colour MUST NOT be the sole carrier of meaning in any chart.
- Contrast ratios MUST meet WCAG AA.

**Theming**
- Light and dark themes, following `prefers-color-scheme` with a manual toggle whose state is held in memory only. No storage APIs are used.

**Acceptance criteria**
- The page renders correctly at 360 px, 768 px, and 1440 px widths.
- Keyboard navigation reaches every interactive control in a logical order.

---

### Task 5.4 — Repo Wrapped mode

Triggered by `--wrapped <year>`. Produces `<outputDir>/wrapped-<year>.html`, again a single self-contained file.

**Normative rules**

1. The analysis is restricted to commits whose author-local date falls within the given calendar year.
2. The presentation is a vertical sequence of full-viewport cards, one statistic per card, navigable by scroll, arrow keys, and swipe.
3. Required cards, in order: total commits with year-over-year delta; chronotype summary sentence; busiest day; longest streak; most touched file; top commit message type; a short-message or typo counter; a closing summary card.
4. Every card MUST have an "export as image" control that renders that card to a PNG via canvas, entirely client-side.
5. If the requested year contains fewer than 10 commits, the command MUST exit with code 2 and the message: `not enough commits in <year> to generate a wrapped report (found <n>, need at least 10)`.
6. Year-over-year delta is omitted from the first card when the preceding year has zero commits.

**Acceptance criteria**
- `--wrapped 2026` on a fixture with 2026 activity produces a working single-file page.
- PNG export produces a non-empty image with no network request.

---

## Milestone 6 — CLI

### Task 6.1 — Command surface

Create `cmd/commitography/main.go` using Cobra.

```
commitography [path] [flags]
```

- `path` is optional and defaults to `.`.

| Flag | Short | Type | Default | Behaviour |
|---|---|---|---|---|
| `--output` | `-o` | string | `./out` | Output directory |
| `--config` | `-c` | string | none | Explicit config file path; replaces repository-local config |
| `--wrapped` | | int | 0 | Generate the year-in-review page for the given year |
| `--per-author` | | bool | false | Include the per-contributor section |
| `--anonymize` | | bool | false | Replace names with pseudonyms and drop emails |
| `--since` | | string | none | Lower date bound, passed to git |
| `--until` | | string | none | Upper date bound, passed to git |
| `--json` | | bool | false | Write only `report.json`, skip HTML rendering |
| `--no-blame` | | bool | false | Skip blame-derived metrics |
| `--allow-shallow` | | bool | false | Proceed despite a shallow clone |
| `--count-merges` | | bool | false | Include merge commits in analysis |
| `--quiet` | `-q` | bool | false | Suppress progress output |
| `--verbose` | `-v` | bool | false | Emit debug logging to stderr |
| `--version` | | bool | false | Print version and exit |

**Normative rules**
- `--quiet` and `--verbose` together is an error.
- Flags override configuration file values.
- Progress output goes to **stderr**, never stdout, so `--json` output can be piped safely when combined with a future stdout mode.

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Unexpected internal error |
| 2 | Usage, configuration, or repository validation error |
| 3 | Analysis completed but produced warnings that were escalated by `--strict` (reserved, not implemented in Phase 1) |

**Acceptance criteria**
- Every flag above is present in `--help` output with the described default.
- Invalid flag combinations exit with code 2 and a message naming the conflict.

---

### Task 6.2 — Progress reporting

For runs exceeding 2 seconds, emit progress to stderr in this form, updating in place when stderr is a TTY and line-by-line when it is not:

```
Reading history ......... 128,401 commits
Resolving identities .... 47 contributors
Filtering ............... 3,204 excluded
Computing metrics ....... temporal, code, messages, social
Sampling blame .......... 300 of 12,884 files
Rendering ............... out/index.html
Done in 41.3s
```

Suppressed entirely by `--quiet`.

**Acceptance criteria**
- Progress never appears on stdout.
- `--quiet` produces zero stderr output on a successful run.

---

## Milestone 7 — Documentation and Release

### Task 7.1 — README

The README MUST contain, in this order:

1. One-sentence description and a screenshot of the dashboard.
2. A prominent callout describing the shallow-clone pitfall with the fix for GitHub Actions, GitLab CI, and Bitbucket Pipelines.
3. Installation: Homebrew, Scoop, `go install`, Docker, and direct binary download.
4. Quick start: the two-line usage example.
5. A live demo link.
6. A short section explaining why per-author metrics are opt-in.
7. Configuration reference.
8. Full flag reference.
9. A metric definitions section, or a link to one, so no displayed number is unexplained.
10. Privacy statement: no telemetry, no network calls, read-only access to the repository.
11. License.

---

### Task 7.2 — Release automation

Configure `.goreleaser.yml` to build for:

| OS | Architectures |
|---|---|
| linux | amd64, arm64 |
| darwin | amd64, arm64 |
| windows | amd64, arm64 |

Requirements:
- Binaries are statically linked with `CGO_ENABLED=0`.
- Version, commit, and build date are injected into `internal/version`.
- Checksums file is published with every release.
- A Docker image is published with `git` installed and the binary as entrypoint.
- Homebrew tap and Scoop manifest are updated automatically.

**Acceptance criteria**
- A tagged release produces all twelve binaries plus checksums and a Docker image.
- The Docker image runs successfully against a mounted repository.

---

### Task 7.3 — Reference outputs

Generate and publish dashboards for at least three well-known public repositories of differing size and age. Host them as static pages linked from the README.

Each reference run MUST be reproducible: the exact command and the analyzed commit hash are recorded alongside the output.

**Acceptance criteria**
- Three live demo links resolve and render.
- Each demo page states the analyzed commit and the tool version used.

---

## Phase 1 Exit Criteria

Phase 1 is complete when **all** of the following hold:

1. `commitography ./repo -o out/` produces a working single-file dashboard on Linux, macOS, and Windows.
2. Commit counts and line totals reconcile exactly with independent `git` commands on all fixtures.
3. Identity resolution reconciles with `git shortlog -sn --all` on the mailmap fixture.
4. Default exclusions prevent lockfiles and vendored code from appearing in any line-based metric.
5. Shallow clones are detected and refused with the specified message.
6. `report.json` validates against the committed JSON Schema.
7. Default output contains no plaintext email addresses; `--anonymize` output contains no real names.
8. A 100,000-commit repository completes analysis in under 60 seconds with `--no-blame` on commodity hardware.
9. The dashboard functions with the network disabled and passes the accessibility requirements in Task 5.3.
10. `--wrapped <year>` produces a working shareable page with client-side PNG export.
11. Cross-platform release artifacts are published and installable via at least two package managers.
12. Three public reference dashboards are live and linked from the README.

Nothing is announced publicly before criteria 1 through 7 are met. A tool that displays an incorrect number on first impression does not get a second chance.
