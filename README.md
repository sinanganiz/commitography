# Commitography

Turn any git repository's history into a beautiful, self-contained static dashboard with a single command — no API tokens, no server, no data leaving your machine.

```bash
commitography ./repo -o out/
open out/index.html
```

<!-- TODO: replace with a screenshot of the generated dashboard once a reference
     run is published. See "Live demos" below. -->

---

## ⚠️ Shallow clones produce wrong numbers

This is the single most common cause of incorrect output, so it is the first thing in this README rather than a footnote.

CI systems shallow-clone by default. A shallow clone has a truncated history, which makes **every** statistic wrong. Commitography detects this and refuses to run rather than showing you a confident, incorrect dashboard.

Locally:

```bash
git fetch --unshallow
```

In CI:

| System | Required setting |
|---|---|
| GitHub Actions | `actions/checkout` with `fetch-depth: 0` |
| GitLab CI | `GIT_DEPTH: 0` |
| Bitbucket Pipelines | `clone: depth: full` |
| Azure Pipelines | `fetchDepth: 0` |
| Jenkins | Disable shallow clone in the Git SCM step |

If you understand the consequences and want the numbers anyway, pass `--allow-shallow`. The dashboard will carry a permanent warning banner.

---

## Installation

The only runtime dependency is `git` itself.

**Homebrew**

```bash
brew install sinanganiz/tap/commitography
```

**Scoop**

```bash
scoop bucket add sinanganiz https://github.com/sinanganiz/scoop-bucket
scoop install commitography
```

**go install**

```bash
go install github.com/sinanganiz/commitography/cmd/commitography@latest
```

**Docker**

```bash
docker run --rm -v "$PWD:/repo" ghcr.io/sinanganiz/commitography:latest
```

**Direct download**

Binaries for Linux, macOS and Windows on amd64 and arm64 are attached to every [release](https://github.com/sinanganiz/commitography/releases), along with a SHA-256 checksums file.

---

## Quick start

```bash
commitography ./repo -o out/
open out/index.html
```

That is the whole thing. No configuration is required for a correct first run.

A year in review, as a shareable page of full-screen cards:

```bash
commitography ./repo --wrapped 2026
```

### Planned local web dashboard

The approved Phase 1.5 plan adds a local web runner for users who prefer a
dashboard while an analysis is running:

```bash
commitography serve --open
```

The server listens on `127.0.0.1:8080` by default. It accepts a repository path
inside the current working directory, or inside a path supplied with one or
more `--allowed-root` flags:

```bash
commitography serve --open --allowed-root /work
```

When implemented, the browser will receive progress updates, let you cancel
the active analysis and open the completed report in the same application. A
normal browser cannot
open an arbitrary host filesystem picker, so native usage enters the path as
text. Docker usage enters the path visible inside the container, not the host
path:

```bash
docker run --rm \
  --publish 127.0.0.1:8080:8080 \
  --mount type=bind,source="$PWD",target=/repos,readonly \
  ghcr.io/sinanganiz/commitography:latest \
  serve --listen 0.0.0.0:8080 --allowed-root /repos
```

The existing Docker default remains the batch CLI mode. The local web runner is
single-user, keeps the ten most recent jobs in memory and does not provide
remote repository access or a persistent server database. See
[`docs/phase-1.5.md`](docs/phase-1.5.md) for the scope and
[`docs/phase-1.5-detailed.md`](docs/phase-1.5-detailed.md) for the work
packages.

---

## Live demos

<!-- TODO: publish reference dashboards for three public repositories of
     differing size and age, and link them here. Each page must state the
     analyzed commit and the tool version used. -->

Not yet published. See [issue tracker](https://github.com/sinanganiz/commitography/issues) for progress.

---

## Why per-author metrics are opt-in

Commitography is explicitly **not** a developer productivity measurement tool.

Per-developer ranking tools are widely and correctly criticized as surveillance tooling, and individual-level measurement carries real legal exposure under GDPR and comparable regimes when applied to employees. Repository-level metrics such as bus factor, change coupling and code churn are also simply more actionable for engineering leadership, without pitting contributors against one another.

Per-contributor figures are available behind `--per-author`. When enabled they are ordered by **first commit date, not by commit count**, because a list ordered by volume reads as a leaderboard however it is labelled.

Emails are hashed by default. `--anonymize` replaces names with stable pseudonyms and drops addresses entirely, which is the recommended setting for publishing a dashboard anywhere public.

---

## Configuration

Configuration is optional. When present, `.commitography.yml` is read from the analyzed repository root, or supplied with `--config`.

```yaml
identities:
  - name: Jane Doe
    emails:
      - jane@example.com          # the first email is the canonical identity
      - jane.doe@corp.example.com

exclude_authors:
  - "release-robot"

exclude_paths:
  - "generated/**"

outlier_threshold_lines: 10000

count_merges: false
date_source: author        # author | committer
use_mailmap: true

anonymize: false
hash_emails: true

output_dir: ./out
theme: default
```

### Resolution order

Later wins:

1. Built-in defaults.
2. `.commitography.yml` in the analyzed repository root.
3. The file given by `--config`, which **replaces** the repository-local file rather than merging with it.
4. Command-line flags.

`exclude_authors` and `exclude_paths` are **appended to** the built-in lists. To suppress the built-ins entirely, set the key to an explicit empty list:

```yaml
exclude_paths: []
```

Unknown keys produce a warning on stderr but never fail the run.

### Defaults worth knowing

- **Bots** such as `dependabot[bot]` and `renovate[bot]` are excluded, along with any author whose name ends in `[bot]`.
- **Generated and vendored paths** — lockfiles, `vendor/`, `node_modules/`, `dist/`, minified bundles, protobuf output, migrations — are excluded from line-based metrics. Without this, machine-written code dominates every number.
- **`.gitattributes`** entries carrying `linguist-generated` are excluded too. `-linguist-generated` puts a path back in.
- Path patterns are matched literally with `**` globbing, not with gitignore's "a bare name matches at any depth" rule. The built-in defaults ship both forms — `pnpm-lock.yaml` and `**/pnpm-lock.yaml`, `node_modules/**` and `**/node_modules/**` — so a monorepo's nested lockfiles and dependency trees are excluded without configuration. A pattern **you** add is anchored where you write it: use `**/` yourself if you mean any depth.

---

## Flags

```
commitography [path] [flags]
```

`path` is optional and defaults to `.`.

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

Progress output goes to **stderr**, never stdout.

### Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Unexpected internal error |
| 2 | Usage, configuration, or repository validation error |
| 3 | Reserved for `--strict`; not implemented |

---

## Metric definitions

No number on the dashboard is unexplained. All times are the **author's own local time**, reconstructed from the timezone offset git recorded, because hour-of-day analysis in UTC says nothing about people.

### Analysis population

| Term | Meaning |
|---|---|
| **Analyzed commits** | Everything except merge commits (unless `--count-merges`) and commits by excluded authors. |
| **Bulk commit** | A commit changing more than `outlier_threshold_lines` (10,000) lines *after* path exclusion. Left out of line, coupling and churn metrics; still counted in commit and temporal metrics; listed separately as a notable event. |
| **Excluded path** | A path matching the exclusion patterns or marked `linguist-generated`. Such files still count toward a commit's existence, only not toward its size. |

### Temporal

| Metric | Definition |
|---|---|
| Hour histogram | 24 buckets; index *i* counts commits whose author-local hour is *i*. |
| Weekday histogram | 7 buckets, index 0 is Monday. |
| Brave deploys | Commits on a local Friday at or after 17:00. |
| Night owl ratio | Share of commits at local hour ≥ 22 or < 6. |
| Busiest day | The local calendar date with the most commits; ties break on the earliest date. |
| Longest streak | The longest run of consecutive local dates each carrying at least one commit. |
| Longest silence | The widest gap in whole days between chronologically adjacent commits. |

### Code

| Metric | Definition |
|---|---|
| Most modified files | Top 25 non-excluded paths by number of commits touching them. Files since removed from HEAD stay in the ranking, flagged. |
| Average / median commit size | Over non-excluded, non-bulk, non-merge commits. |
| Code age | Which year each surviving line was last written in, from `git blame` over a **deterministic sample of at most 300 text files**. The dashboard states the sampling ratio. `--no-blame` skips this entirely. |
| Oldest untouched file | Among files present in HEAD, the one whose most recent modifying commit is oldest. |

### Social

| Metric | Definition |
|---|---|
| **Bus factor** | The smallest number of contributors accounting for at least 50% of commits in a scope. Computed for the repository and for directories at depth 1 and 2 with at least 10 commits' activity. |
| **Change coupling** | File pairs appearing together in at least 5 commits with confidence ≥ 0.5, where confidence is `support / min(changes(A), changes(B))`. Commits touching more than 50 files are skipped. Pairs sharing a basename, such as `Foo.ts` and `Foo.test.ts`, are reported but flagged `expected`. |
| **Churn** | Files receiving at least 5 commits inside any rolling 30-day window. |
| **Knowledge concentration** | Per top-level directory, the share held by its single largest contributor. The contributor is not named unless `--per-author` is set. |

### Messages

A subject is a **Conventional Commit** when it matches `^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\(...\))?!?: .+`. Everything else is classified by an ordered list of keyword rules, first match winning.

The dashboard reports the **conventional ratio** and labels the section low confidence below 0.30, because keyword classification of free-form messages is a guess and should be presented as one.

---

## Output artifacts

| File | Contents |
|---|---|
| `out/index.html` | The dashboard. One file: CSS, JavaScript and the report JSON are all inlined. It opens correctly from anywhere, with no sibling files and no network. |
| `out/report.json` | The same numbers as a documented, machine-readable artifact. Validated against [`docs/report-schema.json`](docs/report-schema.json). Not referenced by the HTML. |
| `out/wrapped-<year>.html` | The year-in-review page, when `--wrapped` is used. |

Within a schema version, `report.json` fields are never removed or repurposed, so external tools can depend on the shape.

---

## Privacy

- **No telemetry.** The tool never phones home, at any point, for any reason.
- **No network calls.** Not during analysis, and not from the generated page — it loads no fonts, scripts or images from anywhere.
- **Read-only.** Commitography never creates commits, branches or tags, and never pushes.
- **Emails are hashed by default** to the first 16 hex characters of a SHA-256 digest. Plaintext addresses do not appear in the output.
- **Nothing is uploaded.** The tool reads a repository and writes files. Where those files go afterwards is entirely your decision.

---

## Development

```bash
make fixtures   # build the deterministic test repositories
make lint       # go vet + gofmt
make test       # go test ./...
make build      # build the frontend, then the binary with version info
make clean
```

The frontend lives in `web/` (TypeScript and hand-written SVG, no framework and no charting library) and is embedded into the binary with `//go:embed`.

Third-party Go dependencies are deliberately limited to three: `cobra`, `yaml.v3` and `doublestar`. Everything else is the standard library.

---

## License

MIT. See [LICENSE](LICENSE).
