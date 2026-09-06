# M5 — GitHub Action, Pages and Dogfood

**Depends on:** M1 (the Action downloads a published release asset) and M4.
**Closes:** Phase 1 exit criterion 12; Phase 2 exit criteria 10 and 11.

Everything in this milestone runs under `.github/`. None of it is the continuous integration that was declined — see [`phase-2-detailed-work-packages.md`](../phase-2-detailed-work-packages.md) §3 for the distinction, which is normative. No workflow created here runs `go test`, `go vet` or `make build`, and none of them gates a push or a pull request.

| Package | Status |
|---|---|
| WP-5.1 Composite action definition | ☐ |
| WP-5.2 Verified binary download | ☐ |
| WP-5.3 Pages publishing template | ☐ |
| WP-5.4 Private-repository artifact template | ☐ |
| WP-5.5 Incremental cache in the templates | ☐ |
| WP-5.6 Dogfood: publish this project's own dashboard | ☐ |
| WP-5.7 Two further reference dashboards and the README links | ☐ |

---

## WP-5.1 — Composite action definition

Create `action.yml` **at the repository root**. GitHub resolves `sinanganiz/commitography@<ref>` to an action definition at the root and nowhere else; a file under `templates/` or `.github/` will not be found.

### Inputs

Deliberately few. Every Commitography flag is reachable through `args`, so the Action does not have to mirror a flag set that will keep growing, and a user who knows the CLI already knows the Action.

| Input | Required | Default | Meaning |
|---|---|---|---|
| `version` | no | the release this action shipped with | Release tag of the binary to download |
| `path` | no | `.` | Repository to analyze |
| `output` | no | `commitography-out` | Output directory |
| `cache` | no | `~/.cache/commitography` | Cache directory, passed as `--cache` |
| `summary` | no | `true` | Append the Markdown digest to the job summary |
| `args` | no | empty | Extra arguments appended verbatim |

### Outputs

| Output | Meaning |
|---|---|
| `index` | Path to the generated `index.html` |
| `report` | Path to the generated `report.json` |

### Normative rules

1. `using: composite`. Decision C1: a Docker action adds an image pull to every run, and a JavaScript action adds Node and a committed bundle.
2. The default for `version` MUST be a concrete tag, never `latest`. An action that silently changes the binary it runs is an action whose output cannot be reproduced. Updating that default is a step in the release procedure and MUST be listed as such in [`m7-docs.md`](m7-docs.md).
3. The Action MUST NOT check out anything. Checkout is the caller's job, and the caller is the only one who can set `fetch-depth: 0`.
4. The Action MUST NOT set `--quiet`. A CI log with no progress is a CI log nobody can debug.
5. `summary: true` MUST append `--summary` output to `$GITHUB_STEP_SUMMARY`. When the variable is unset — the Action running outside GitHub Actions, which happens in local runners — it MUST be skipped silently rather than failing.
6. The Action MUST fail the step when the binary exits non-zero, including exit 3 from `--strict`. Swallowing an exit code would make `--strict` unusable through the Action, which is where it is most wanted.
7. `args` MUST be passed through without shell re-interpretation of quotes beyond one documented level, and the documentation MUST state which level that is.

### Acceptance criteria

- `action.yml` parses as a valid composite action.
- A workflow using it with no inputs produces a dashboard.
- `args: --strict` on a repository with warnings fails the step.
- With `GITHUB_STEP_SUMMARY` unset, the step succeeds and skips the summary.

---

## WP-5.2 — Verified binary download

Implemented as steps inside `action.yml`, or as a script under `.github/scripts/` that `action.yml` calls.

### Normative sequence

1. Map `runner.os` and `runner.arch` onto the operating system and architecture strings goreleaser used in the release archive names.
2. Download the archive and the checksum file from the release identified by `version`.
3. Compute the archive's SHA-256 and compare it against its line in the checksum file. **A mismatch MUST fail the step**, with a message naming both values. It MUST NOT warn and continue.
4. Extract, place the binary on `PATH` for subsequent steps, and confirm `commitography --version` reports the requested version.

### The archive names must be read, not guessed

`.goreleaser.yml` builds names from `{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}`, and goreleaser's `.Version` differs from `.Tag` in whether the leading `v` survives. `format_overrides` also gives Windows a `.zip` where every other target gets `.tar.gz`.

**This package MUST derive the exact names from the release that [WP-1.3](m1-release.md) actually produced, and MUST NOT infer them from the template.** Getting this wrong produces a 404 at the moment a user first tries the Action, which is the worst possible place for it.

### Caching the download

The downloaded binary SHOULD be cached by `version` and platform, so a scheduled job does not re-download an identical file every run. The cache key MUST include the version, and a cache hit MUST still verify the checksum: a cache is not a trust boundary.

### Acceptance criteria

- The correct archive downloads on `ubuntu-latest`, `windows-latest` and `macos-latest`.
- A deliberately corrupted archive fails the step with both checksums named.
- `commitography --version` after extraction matches the requested version.
- A second run in the same job reuses the cached download and still verifies it.

---

## WP-5.3 — Pages publishing template

Create `templates/github-actions/dashboard.yml`. This is the file [`phase-2.md`](../phase-2.md) deliverable 1 describes, and the file exit criterion 10 measures: copying it into a repository, and nothing else, must produce a published dashboard.

### Normative contents

1. Triggers: a weekly `schedule` and `workflow_dispatch`. It MUST NOT trigger on `push`. A dashboard regenerated on every commit is a Pages deployment queue, not a dashboard.
2. `actions/checkout` with **`fetch-depth: 0`**, carrying an inline comment stating that removing it silently produces wrong numbers. This is the single most common cause of incorrect output and the reason `phase-2.md` §3 leads with it.
3. The Commitography action, pinned to a tag.
4. `actions/upload-pages-artifact` and `actions/deploy-pages`.
5. `permissions` set at the smallest scope that works: `contents: read`, `pages: write`, `id-token: write`, declared at the job rather than the workflow where possible.
6. `concurrency` with a group name and `cancel-in-progress: false`, so two schedules overlapping never race a deployment.
7. Every third-party action pinned to a full-length commit SHA with the human-readable version in a trailing comment. A template that teaches unpinned actions teaches a supply-chain hazard to everyone who copies it.

### Privacy defaults

[`phase-2.md`](../phase-2.md) §3 requires the shipped templates to make the privacy decision explicit at the point where it is made. The template MUST therefore carry, as comments immediately above the run step:

- That emails are hashed by default and that this is what makes the output safe to publish.
- That `--per-author` names individuals and that publishing to Pages makes that world-readable.
- That `--anonymize` is the recommended setting for a public dashboard of a repository whose contributors have not been asked.

These are comments, not settings. The template's default is the tool's default, and the decision stays the user's.

### Acceptance criteria

- Copying the file into a fresh public repository with Pages enabled, changing nothing, produces a published dashboard.
- Removing `fetch-depth: 0` makes the run fail with the shallow-clone error rather than publishing wrong numbers.
- Every third-party action is SHA-pinned.

---

## WP-5.4 — Private-repository artifact template

Create `templates/github-actions/dashboard-artifact.yml`. Decision C5: GitHub Pages is unavailable on private repositories outside paid plans, and much of this product's audience is private and self-hosted.

### Normative contents

Identical to WP-5.3 except that the Pages steps are replaced by `actions/upload-artifact`, and the header comment states plainly:

- That this variant exists because Pages is unavailable on private repositories on the free plan.
- That the artifact is downloaded and opened locally.
- That the dashboard is a single self-contained file, so downloading and opening it works with no server and no network.
- What the artifact retention period is and that it is configurable.

### Acceptance criteria

- Copying the file into a private repository produces a downloadable artifact containing `index.html` and `report.json`.
- The downloaded `index.html` opens correctly from `file://` with the network disabled.

---

## WP-5.5 — Incremental cache in the templates

Both templates gain an `actions/cache` step around the run.

### Normative rules

1. The cache key MUST be unique per run and the restore key a stable prefix — for example a key ending in the run's commit SHA with a restore prefix that omits it. `actions/cache` never updates an entry whose key already exists, so a fixed key would store the first run's cache forever and every later run would restore a cache that only grows staler. This is the single most common way an incremental cache in GitHub Actions silently stops working.
2. The key MUST include the Commitography version. A `FormatVersion` bump invalidates the cache on load anyway, but keeping stale content out of storage is cheaper than downloading it to discard it.
3. The cached path MUST be the directory passed as the Action's `cache` input, and the two MUST be defined once and referenced, not written out twice.
4. A comment MUST state that deleting the cache is always safe and only costs runtime — the contract [`phase-2.md`](../phase-2.md) §3 sets and [WP-2.3](m2-cache.md) implements.
5. A comment MUST warn against sharing one cache path across a job matrix, since concurrent writers can leave a mismatched set that the next run discards. [WP-2.3](m2-cache.md) explains why that is safe but wasteful.

### Acceptance criteria

- A second scheduled run restores the cache, reads only new commits, and says so in its log.
- Deleting the repository's caches makes the next run a full read and changes nothing else about the output.
- Three consecutive runs each restore the previous run's cache, verified in the logs rather than assumed.

---

## WP-5.6 — Dogfood: publish this project's own dashboard

Create `.github/workflows/dashboard.yml` in the project repository, **as a copy of the template from WP-5.3 with only the values a copier would change**. If the copy needs an edit the template does not anticipate, the template is wrong and MUST be fixed rather than the copy adjusted.

### Normative rules

1. It MUST use the published Action by tag, exactly as an external user would. Referencing the local `./` action would test a code path no user takes.
2. It MUST NOT run `go test`, `go vet`, `make build` or any other verification. See [`phase-2-detailed-work-packages.md`](../phase-2-detailed-work-packages.md) §3. If it ever needs to, that is the declined CI and needs a new decision.
3. It MUST run with `--per-author`. This repository's contributors are the project's own, the decision is theirs to make, and the section is worth showing on a reference dashboard.
4. Any divergence from the template MUST be recorded as a comment in the template explaining why a copier might need the same change.

### Acceptance criteria

- The workflow runs on schedule and on manual dispatch, and publishes.
- The published dashboard is reachable and renders with no console errors.
- A diff between `.github/workflows/dashboard.yml` and `templates/github-actions/dashboard.yml` shows only repository-specific values.

---

## WP-5.7 — Two further reference dashboards and the README links

Phase 1 exit criterion 12 requires three live public reference dashboards, linked from the README. WP-5.6 produces the first.

### Choosing the two repositories

The choice MUST be recorded here when made. It MUST satisfy:

1. Public, with full history available from a normal clone.
2. Different in shape from this project and from each other — for example one very large and long-lived, one mid-sized and active. The dashboards exist to show what the tool does across shapes, and three small young repositories demonstrate nothing.
3. Large enough to populate the sections that need volume: coupling requires pairs at support ≥ 5 and confidence ≥ 0.5, and the code-age chart needs more than one calendar year to say anything. This project's own repository is 11 commits spanning three days, so its dashboard will show almost nothing on either — which is precisely why the other two have to carry the demonstration.

### Privacy, for repositories whose contributors were not asked

**Third-party reference dashboards MUST NOT use `--per-author`.** The tool's stated position is that per-contributor breakdowns are opt-in and the opt-in belongs to the people being described. Publishing a named per-contributor breakdown of a project that never asked for one would contradict the position the README argues for, on a page whose purpose is to demonstrate the product.

Repository-level metrics are the product, and they are what these dashboards should show. Emails remain hashed by default, which is what makes publishing them acceptable at all.

### Publishing layout

All three dashboards MUST be published from the single Pages site this project already deploys, at distinct paths, so that Pages is configured once. The workflow checks out each repository with `fetch-depth: 0` into a separate directory, runs the Action against each, and assembles one artifact.

A small index page MUST link the three, with one line each stating which repository it describes and when it was generated.

### README

Both `TODO` blocks in `README.md` — at lines 10 and 95 — MUST be replaced:

- Line 10: a screenshot of a generated dashboard. It SHOULD come from a dashboard with enough data to look like the product rather than an empty template.
- Line 95: the "Live demos" section, linking all three dashboards.

### Acceptance criteria

- Three dashboards are live at stable URLs and regenerate on the same schedule.
- Neither third-party dashboard contains a per-contributor section.
- No `TODO` remains in `README.md`.
- Phase 1 exit criterion 12 is marked met in [`phase-1-detailed.md`](../phase-1-detailed.md).
