# Commitography

**Turn any Git repository's history into a self-contained dashboard that tells
its story. One command, no API tokens, no server and no data leaving your
machine.**

```bash
commitography ./my-repo -o out/
```

Commitography reads a repository's commit history and shows:
- when the repository is active;
- which files the team keeps returning to;
- where knowledge is concentrated;
- how the code has aged;
- what the commit messages say about how the team works.

The result is one HTML file that opens in any browser, offline, with nothing
next to it.

It describes a repository, not a workforce: metrics are repository-level by
default, and per-contributor figures are strictly opt-in.

<!-- TODO: add a screenshot of the generated dashboard and links to three
     reference dashboards once they are published. -->

> [!IMPORTANT]
> **Project status: pre-release.** No version has been tagged yet, so the
> Homebrew, Scoop, container registry and release downloads are not available.
> Install with [`go install`](#install-with-go) or
> [build from source](#build-from-source). Both include everything described
> here, including the local web dashboard.

---

## Contents

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Usage examples](#usage-examples)
- [Local web dashboard](#local-web-dashboard)
- [Running with Docker](#running-with-docker)
- [Command reference](#command-reference)
- [Shallow clones](#shallow-clones)
- [Configuration](#configuration)
- [Output](#output)
- [Metric definitions](#metric-definitions)
- [Privacy](#privacy)
- [Performance](#performance)
- [Troubleshooting](#troubleshooting)
- [Development](#development)
- [Documentation](#documentation)
- [License](#license)

---

## Features

- **Works with any Git host.** GitHub, GitLab, Bitbucket, Gitea, Azure DevOps
  or a self-hosted server: Commitography reads only the local clone and never
  talks to a provider API.
- **Single binary.** The frontend is embedded; the only runtime dependency is
  `git`.
- **Self-contained output.** `index.html` inlines its styles, scripts and data,
  and loads nothing from the network.
- **Statistically honest defaults.** Bots, lockfiles, vendored and generated
  files, and bulk imports are filtered. Identities are resolved through
  `.mailmap` before anything is counted, and shallow clones are refused.
- **Rich, explained metrics.** Chronotype, streaks and silences; code age,
  churn and change coupling; bus factor and knowledge concentration; Conventional
  Commit usage with an explicit confidence label.
- **Machine-readable report.** `report.json` follows a documented, versioned
  [JSON schema](docs/report-schema.json).
- **Year in review.** `--wrapped` produces a shareable card-based page for a
  single year.
- **Local web dashboard.** `commitography serve` runs the analysis from your
  browser, with live progress, cancellation and recent reports.
- **Privacy first.** No telemetry, e-mail addresses hashed by default and an
  `--anonymize` mode for publishing.

---

## Requirements

| Requirement | Needed for |
|---|---|
| `git` on `PATH` | Every analysis |
| Go 1.22 or later | Installing with `go install` or building from source |
| A full (non-shallow) clone | Correct results; see [Shallow clones](#shallow-clones) |
| Docker | Only when running in a container |

Node.js is **not** required: the built frontend is committed to the repository.

---

## Installation

### Install with Go

```bash
go install github.com/sinanganiz/commitography/cmd/commitography@latest
```

The binary is placed in `$(go env GOPATH)/bin`, which must be on your `PATH`.
Check the installation:

```bash
commitography --version
```

### Build from source

**Linux and macOS**

```bash
git clone https://github.com/sinanganiz/commitography.git
cd commitography
go build -o commitography ./cmd/commitography
./commitography --version
```

**Windows (PowerShell)**

```powershell
git clone https://github.com/sinanganiz/commitography.git
cd commitography
go build -o commitography.exe ./cmd/commitography
.\commitography.exe --version
```

To embed version information and rebuild the frontend as well, use
`make build` (requires Node.js and `make`).

### Docker

No image is published yet. Build one locally as described in
[Running with Docker](#running-with-docker).

### Planned distribution channels

The release configuration in [`.goreleaser.yml`](.goreleaser.yml) prepares
binaries for Linux, macOS and Windows on amd64 and arm64, a Homebrew tap, a
Scoop bucket and a container image on `ghcr.io`. These will become available
with the first tagged release.

---

## Quick start

1. **Get a full clone** of the repository you want to analyze.

   ```bash
   git clone https://github.com/example/project.git
   ```

   For an existing shallow clone, run `git fetch --unshallow` first.

2. **Run the analysis.**

   ```bash
   commitography ./project -o out/
   ```

   Progress is printed to stderr. The run needs no configuration.

3. **Open the dashboard.**

   | OS | Command |
   |---|---|
   | macOS | `open out/index.html` |
   | Linux | `xdg-open out/index.html` |
   | Windows | `start out\index.html` |

The same numbers are also written to `out/report.json`.

---

## Usage examples

Analyze the current directory and write to the default `./out`:

```bash
commitography
```

Choose a repository and an output directory:

```bash
commitography ~/src/api -o ~/reports/api
```

Limit the analysis to a period (any date format `git log` accepts):

```bash
commitography ./api --since 2025-01-01 --until 2025-12-31
commitography ./api --since "6 months ago"
```

Create a year-in-review page instead of the dashboard (`out/wrapped-2025.html`):

```bash
commitography ./api --wrapped 2025
```

Produce only the JSON report, for example for another tool:

```bash
commitography ./api --json -o build/metrics
```

Analyze a very large repository quickly by skipping `git blame`:

```bash
commitography ./monorepo --no-blame
```

Prepare a dashboard for public publishing, with pseudonyms instead of names:

```bash
commitography ./api --anonymize -o public/
```

Include the opt-in per-contributor section for an internal review:

```bash
commitography ./api --per-author
```

Count merge commits and use an explicit configuration file:

```bash
commitography ./api --count-merges --config ./ci/commitography.yml
```

Run silently in a script and check the exit code:

```bash
commitography ./api -q -o out/ || echo "analysis failed with exit code $?"
```

---

## Local web dashboard

`commitography serve` starts a single-user web application on your own
computer. Type a repository path, follow the analysis stage by stage, cancel
it if needed and read the finished report in the same page.

### Step by step

1. **Start the server.** It analyzes repositories below the current directory
   unless you name other folders with `--allowed-root`.

   ```bash
   commitography serve --open --allowed-root ~/src
   ```

   On Windows:

   ```powershell
   .\commitography.exe serve --open --allowed-root C:\Users\ana\src
   ```

2. **Open the dashboard.** The server prints its address,
   `http://127.0.0.1:8080`; `--open` opens it in your default browser.

3. **Enter a repository path.** A web page cannot browse your disk, so type the
   path, for example `/home/ana/src/api` or `C:\Users\ana\src\api`. Advanced
   options mirror the CLI flags: skip blame, per-author, anonymize, allow
   shallow and count merges.

4. **Follow and control the analysis.** The progress view shows the current
   stage and an estimated fraction. Cancel at any time.

5. **Read the report.** The finished dashboard opens in the same page. The ten
   most recent jobs stay available from the job list.

6. **Stop the server** with `Ctrl+C`.

### More examples

Allow several repository folders:

```bash
commitography serve --allowed-root ~/src --allowed-root /mnt/work/repos
```

Use a different port:

```bash
commitography serve --listen 127.0.0.1:9000 --open
```

### What to expect

- **One analysis at a time.** Starting a second analysis while one is running
  is refused.
- **Memory only.** Jobs and reports are kept in memory, limited to the ten most
  recent, and are gone when the server stops. Nothing is written to the
  analyzed repository.
- **Local only.** The server is a personal tool, not a network service:
  - it listens on `127.0.0.1` by default;
  - it accepts only `localhost`, `127.0.0.1` and `::1` as host names;
  - it protects state-changing requests with a session cookie and an Origin
    check.

  Binding `--listen` to another address prints a warning.
- **Local repositories only.** There is no remote cloning, no account and no
  scheduling.

---

## Running with Docker

The image contains the same binary and `git`. It runs in two modes:
- **CLI mode**, the default, writes a static dashboard.
- **Server mode** starts the local web dashboard when you pass `serve`.

### Step 1: Build the image

With `make` (Linux, macOS or Git Bash with `make` installed):

```bash
make docker-image
```

Without `make`, from the repository root:

**bash, zsh or Git Bash**

```bash
rm -rf dist/docker && mkdir -p dist/docker
CGO_ENABLED=0 GOOS=linux GOARCH="$(docker version --format '{{.Server.Arch}}')" \
  go build -trimpath -o dist/docker/commitography ./cmd/commitography
cp Dockerfile dist/docker/Dockerfile
docker build -t commitography:local dist/docker
```

**Windows PowerShell**

```powershell
Remove-Item -Recurse -Force dist\docker -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force dist\docker | Out-Null
$env:CGO_ENABLED = '0'; $env:GOOS = 'linux'; $env:GOARCH = docker version --format '{{.Server.Arch}}'
go build -trimpath -o dist\docker\commitography ./cmd/commitography
Remove-Item Env:CGO_ENABLED, Env:GOOS, Env:GOARCH
Copy-Item Dockerfile dist\docker\Dockerfile
docker build -t commitography:local dist\docker
```

Both produce the image `commitography:local`.

### Step 2a: Generate a dashboard (CLI mode)

Run these from the repository you want to analyze. The repository is mounted
read-only, and the dashboard is written to a separate `out` folder, which must
exist before it is mounted.

**Linux and macOS**

```bash
mkdir -p out
docker run --rm \
  --mount type=bind,source="$PWD",target=/repo,readonly \
  --mount type=bind,source="$PWD/out",target=/out \
  commitography:local /repo -o /out
```

**Windows PowerShell**

```powershell
New-Item -ItemType Directory -Force out | Out-Null
docker run --rm `
  --mount "type=bind,source=$PWD,target=/repo,readonly" `
  --mount "type=bind,source=$PWD\out,target=/out" `
  commitography:local /repo -o /out
```

**Windows Git Bash**

```bash
mkdir -p out
MSYS_NO_PATHCONV=1 docker run --rm \
  --mount type=bind,source="$(pwd -W)",target=/repo,readonly \
  --mount type=bind,source="$(pwd -W)/out",target=/out \
  commitography:local /repo -o /out
```

Open `out/index.html`. Any CLI flag can follow the path, for example
`/repo -o /out --no-blame --anonymize`.

**Tips for CLI mode**
- The report is named after the analyzed path, so the examples above produce a
  report named `repo`. To keep the real name, mount at a path ending in it, for
  example `target=/repos/api`, and analyze `/repos/api`.
- On Linux, add `--user "$(id -u):$(id -g)"` so the output files belong to you
  rather than root.

### Step 2b: Start the web dashboard (server mode)

Mount the folder that contains your repositories at `/repos` and publish the
port on the host's loopback interface only.

**Linux and macOS**

```bash
docker run --rm \
  --publish 127.0.0.1:8080:8080 \
  --mount type=bind,source="$PWD",target=/repos,readonly \
  commitography:local \
  serve --listen 0.0.0.0:8080 --allowed-root /repos
```

**Windows PowerShell**

```powershell
docker run --rm `
  --publish 127.0.0.1:8080:8080 `
  --mount "type=bind,source=$PWD,target=/repos,readonly" `
  commitography:local `
  serve --listen 0.0.0.0:8080 --allowed-root /repos
```

**Windows Git Bash**

```bash
MSYS_NO_PATHCONV=1 docker run --rm \
  --publish 127.0.0.1:8080:8080 \
  --mount type=bind,source="$(pwd -W)",target=/repos,readonly \
  commitography:local \
  serve --listen 0.0.0.0:8080 --allowed-root /repos
```

Then open <http://127.0.0.1:8080> and enter a **container path**:

| Repository on the host | Mounted with `source=` | Path to type in the dashboard |
|---|---|---|
| `/home/ana/src/api` | `/home/ana/src` | `/repos/api` |
| `/Users/ana/src/api` | `/Users/ana/src` | `/repos/api` |
| `C:\Users\ana\src\api` | `C:\Users\ana\src` | `/repos/api` |

If the mounted folder is itself a repository, type `/repos`. Stop the container
with `Ctrl+C`.

### Why these flags

| Flag | Reason |
|---|---|
| `--publish 127.0.0.1:8080:8080` | Only this computer can reach the dashboard. Never use `--publish 8080:8080` or `-P`: both expose the port on every network interface. |
| `--listen 0.0.0.0:8080` | Required inside the container, because published traffic arrives on the container's network interface. The host-side `127.0.0.1` binding keeps it private. |
| `readonly` | Commitography never writes to repositories. |
| `--allowed-root /repos` | Limits analysis to the mounted folder. |

[`docs/docker.md`](docs/docker.md) covers Docker Desktop file sharing,
worktrees and submodules, SELinux and troubleshooting.

---

## Command reference

### `commitography [path] [flags]`

Analyzes the repository at `path` (default: the current directory) and writes
the dashboard.

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|
| `--output` | `-o` | string | `./out` | Output directory. |
| `--config` | `-c` | string | — | Configuration file. Replaces the repository's `.commitography.yml` instead of merging with it. |
| `--since` | | string | — | Lower date bound, passed to `git` (for example `2025-01-01` or `"6 months ago"`). |
| `--until` | | string | — | Upper date bound, passed to `git`. |
| `--wrapped` | | int | — | Generate only the year-in-review page, `wrapped-<year>.html`, for the given year, instead of `index.html`. The year needs at least 10 commits. |
| `--json` | | bool | `false` | Write only `report.json`; skip HTML rendering. |
| `--no-blame` | | bool | `false` | Skip `git blame`. Much faster on large repositories; code age and the share of lines surviving from the first year are omitted. |
| `--per-author` | | bool | `false` | Include the per-contributor section. |
| `--anonymize` | | bool | `false` | Replace names with stable pseudonyms and drop e-mail addresses. |
| `--count-merges` | | bool | `false` | Include merge commits in the analysis. |
| `--allow-shallow` | | bool | `false` | Proceed on a shallow clone; the dashboard shows a permanent warning. |
| `--quiet` | `-q` | bool | `false` | Suppress progress output. |
| `--verbose` | `-v` | bool | `false` | Write debug logging to stderr. |
| `--version` | | bool | `false` | Print the version and exit. |
| `--help` | `-h` | bool | `false` | Show help. |

Progress and diagnostics go to **stderr**; stdout stays clean for scripting.

### `commitography serve [flags]`

Starts the [local web dashboard](#local-web-dashboard).

| Flag | Type | Default | Description |
|---|---|---|---|
| `--listen` | string | `127.0.0.1:8080` | HTTP listen address. A non-loopback address outside a container prints a warning. |
| `--allowed-root` | string, repeatable | current directory | Folder whose repositories may be analyzed. Paths outside every allowed root are refused. |
| `--open` | bool | `false` | Open the dashboard in the default browser once the server is listening. |

`serve` exits with `1` when it cannot listen or an allowed root does not exist,
and with `0` when stopped with `Ctrl+C` or `SIGTERM`.

### `commitography completion <shell>`

Generates a shell completion script for `bash`, `zsh`, `fish` or `powershell`.

### Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Unexpected internal error |
| `2` | Usage, configuration or repository validation error, including a refused shallow clone |
| `3` | Reserved for a future `--strict` mode; not currently returned |

---

## Shallow clones

> [!WARNING]
> A shallow clone has a truncated history, which makes **every** statistic
> wrong. Commitography detects this and refuses to run rather than showing a
> confident but incorrect dashboard.

CI systems shallow-clone by default. Fetch the full history locally with
`git fetch --unshallow`, or configure your CI:

| System | Required setting |
|---|---|
| GitHub Actions | `actions/checkout` with `fetch-depth: 0` |
| GitLab CI | `GIT_DEPTH: 0` |
| Bitbucket Pipelines | `clone: depth: full` |
| Azure Pipelines | `fetchDepth: 0` |
| Jenkins | Disable shallow clone in the Git SCM step |

If you understand the consequences, `--allow-shallow` produces the dashboard
anyway, with a permanent warning banner.

---

## Configuration

Configuration is optional. A `.commitography.yml` in the analyzed repository's
root is read automatically, or a file can be supplied with `--config`.

```yaml
identities:
  - name: Jane Doe
    emails:
      - jane@example.com          # the first address is the canonical identity
      - jane.doe@corp.example.com

exclude_authors:
  - "release-robot"

exclude_paths:
  - "generated/**"

outlier_threshold_lines: 10000   # commits larger than this are "bulk" commits

count_merges: false
date_source: author              # author | committer
use_mailmap: true

anonymize: false
hash_emails: true

output_dir: ./out
theme: default
```

### Resolution order

Later sources win:

1. Built-in defaults.
2. `.commitography.yml` in the analyzed repository root.
3. The file given with `--config`, which **replaces** the repository file.
4. Command-line flags.

`exclude_authors` and `exclude_paths` are **appended** to the built-in lists.
To disable the built-in list, set the key to an empty list:

```yaml
exclude_paths: []
```

Unknown keys produce a warning on stderr but never fail the run.

### Built-in defaults

- **Bots** such as `dependabot[bot]` and `renovate[bot]`, and any author whose
  name ends in `[bot]`, are excluded.
- **Generated and vendored paths** are excluded from line-based metrics:
  lockfiles, `vendor/`, `node_modules/`, `dist/`, minified bundles, protobuf
  output and migrations.
- **`.gitattributes`** entries with `linguist-generated` are excluded;
  `-linguist-generated` includes a path again.
- **Path patterns** use `**` globbing and are anchored where you write them,
  unlike gitignore's rule that a bare name matches at any depth. The built-in
  defaults ship both forms, such as `pnpm-lock.yaml` and `**/pnpm-lock.yaml`,
  so nested files in monorepos are covered. Add `**/` yourself when you mean
  any depth.

---

## Output

| File | Contents |
|---|---|
| `out/index.html` | The dashboard. CSS, JavaScript and report data are inlined, so it opens from anywhere with no sibling files and no network access. |
| `out/report.json` | The same numbers as a machine-readable artifact, validated against [`docs/report-schema.json`](docs/report-schema.json). |
| `out/wrapped-<year>.html` | The year-in-review page, when `--wrapped` is used. |

Within a schema version, `report.json` fields are never removed or repurposed,
so external tools can depend on its shape.

---

## Metric definitions

No number on the dashboard is unexplained. All times are the **author's local
time**, reconstructed from the timezone offset Git recorded, because
hour-of-day analysis in UTC says nothing about people.

### Analysis population

| Term | Meaning |
|---|---|
| **Analyzed commits** | All commits except merge commits (unless `--count-merges`) and commits by excluded authors. |
| **Bulk commit** | A commit changing more than `outlier_threshold_lines` (10,000) lines after path exclusion. Left out of line, coupling and churn metrics; still counted in commit and temporal metrics; listed separately as a notable event. |
| **Excluded path** | A path matching the exclusion patterns or marked `linguist-generated`. It still counts toward a commit's existence, but not toward its size. |

### Temporal

| Metric | Definition |
|---|---|
| Hour histogram | 24 buckets; bucket *i* counts commits made at author-local hour *i*. |
| Weekday histogram | 7 buckets; bucket 0 is Monday. |
| Brave deploys | Commits on a local Friday at or after 17:00. |
| Night owl ratio | Share of commits at local hour ≥ 22 or < 6. |
| Busiest day | The local date with the most commits; ties go to the earliest date. |
| Longest streak | The longest run of consecutive local dates with at least one commit. |
| Longest silence | The widest gap, in whole days, between chronologically adjacent commits. |

### Code

| Metric | Definition |
|---|---|
| Most modified files | Top 25 non-excluded paths by the number of commits touching them. Files since removed from HEAD stay in the ranking, flagged. |
| Average / median commit size | Over non-excluded, non-bulk, non-merge commits. |
| Code age | The year each surviving line was last written, from `git blame` over a **deterministic sample of at most 300 text files**. The dashboard states the sampling ratio. Skipped with `--no-blame`. |
| Oldest untouched file | Among files in HEAD, the one whose most recent modifying commit is oldest. |

### Social

| Metric | Definition |
|---|---|
| **Bus factor** | The smallest number of contributors accounting for at least 50% of commits in a scope. Computed for the repository and for directories at depth 1 and 2 with at least 10 commits of activity. |
| **Change coupling** | File pairs that appear together in at least 5 commits with confidence ≥ 0.5, where confidence is `support / min(changes(A), changes(B))`. Commits touching more than 50 files are skipped. Pairs sharing a base name, such as `Foo.ts` and `Foo.test.ts`, are reported but flagged `expected`. |
| **Churn** | Files receiving at least 5 commits inside any rolling 30-day window. |
| **Knowledge concentration** | Per top-level directory, the share held by its single largest contributor. The contributor is named only with `--per-author`. |

### Messages

A subject is a **Conventional Commit** when it matches
`^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\(...\))?!?: .+`.
Other messages are classified by an ordered list of keyword rules; the first
match wins.

The dashboard reports the **conventional ratio** and labels the section as low
confidence below 0.30, because keyword classification of free-form messages is
a guess and is presented as one.

---

## Privacy

- **No telemetry.** The tool never phones home, for any reason.
- **No network calls.** None during analysis, and none from the generated page,
  which loads no fonts, scripts or images.
- **Read-only.** Commitography never creates commits, branches or tags, and
  never pushes.
- **E-mail addresses are hashed by default** to the first 16 hex characters of
  a SHA-256 digest; plaintext addresses do not appear in the output.
- **Nothing is uploaded.** Where the generated files go is entirely your
  decision.

### Why per-author metrics are opt-in

Commitography is deliberately **not** a developer productivity tool. Ranking
individuals is widely criticized as surveillance, carries legal exposure under
GDPR and comparable regimes, and is less actionable than repository-level
signals such as bus factor, change coupling and churn.

Per-contributor figures are available with `--per-author`. They are ordered by
**first commit date, not commit count**, because a list ordered by volume reads
as a leaderboard however it is labelled. For anything published publicly, use
`--anonymize`.

---

## Performance

`git blame` is by far the most expensive step. Measured on a 12-core laptop, a
generated repository with 15,000 commits took about 1 minute 40 seconds with
blame and about half a second with `--no-blame`.

- Use **`--no-blame`** for repositories with long per-file histories, or when
  code age is not needed. All other metrics are unchanged.
- Very large working trees also cost time, because every tracked text file is
  read once to count lines.
- **Docker Desktop** bind mounts on Windows and macOS are noticeably slower than
  native disk access. Prefer the native binary for large repositories.

Detailed measurements are recorded in
[`docs/phase-1.5/m6-verification.md`](docs/phase-1.5/m6-verification.md).

---

## Troubleshooting

| Symptom | Cause and fix |
|---|---|
| `shallow clone` error | The history is truncated. Run `git fetch --unshallow`, or set a full-depth checkout in CI. |
| `git` not found | Install Git and make sure it is on `PATH`. |
| `commitography: command not found` after `go install` | Add `$(go env GOPATH)/bin` to your `PATH`. |
| The dashboard refuses a path | The path is outside every `--allowed-root`, or it is not a Git repository. In Docker, type the container path (`/repos/<name>`), never the host path. |
| Docker reports that a mounted path is not a repository | Docker Desktop creates an empty folder for a mistyped `source=`. Check the spelling and the file sharing settings. |
| Git Bash turns `/repos` into `C:/Program Files/Git/repos` | Prefix the command with `MSYS_NO_PATHCONV=1` and use `$(pwd -W)`. |
| The web dashboard is unreachable from Docker | Pass `--listen 0.0.0.0:8080` to `serve` and publish with `--publish 127.0.0.1:8080:8080`. |
| The analysis is slow | Use `--no-blame`, and run the native binary instead of Docker for large repositories. |

---

## Development

```bash
make fixtures      # build the deterministic test repositories
make lint          # go vet and gofmt
make test          # go test ./...
make build         # build the frontend, then the binary with version information
make docker-image  # build commitography:local for the local Docker daemon
make docker-smoke  # run that image against the fixtures (needs Docker and make fixtures)
make perfcheck     # measure analysis time, responsiveness, cancellation and memory
make clean
```

Frontend checks run from `web/`:

```bash
npm ci
npm run typecheck   # TypeScript
npm run test        # component tests
npm run e2e         # browser audit; needs Chrome, Go and Git
```

**Project layout**

| Path | Contents |
|---|---|
| `cmd/commitography` | Command-line entry point |
| `internal/` | Collection, aggregation, rendering, analysis service, job manager and HTTP server |
| `web/` | React and MUI application, plus hand-written SVG charts |
| `testdata/` | Fixture repository generators |
| `docs/` | Design, scope, schema and verification documents |

The built frontend bundle is committed and embedded with `//go:embed`, so
`go build` works without Node.js. Third-party Go dependencies are deliberately
limited to `cobra`, `yaml.v3` and `doublestar`.

---

## Documentation

| Document | Topic |
|---|---|
| [`docs/project-overview.md`](docs/project-overview.md) | Vision, principles, architecture and roadmap |
| [`docs/docker.md`](docs/docker.md) | Complete Docker guide |
| [`docs/report-schema.json`](docs/report-schema.json) | `report.json` schema |
| [`docs/phase-1.5.md`](docs/phase-1.5.md) | Local web dashboard scope, limitations and exit criteria |
| [`docs/phase-2.md`](docs/phase-2.md) | Planned CI/CD integration |
| [`docs/phase-3.md`](docs/phase-3.md) | Planned server mode |

---

## License

Commitography is released under the MIT License. See [LICENSE](LICENSE).
