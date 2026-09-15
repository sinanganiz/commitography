# Repository audit

Produced by WP-0001. It records what exists in the tracked tree and which
accepted record governs it. It states no opinion about implementation quality
and proposes no fix.

**Scope.** Every file listed by `git ls-files`, excluding the paths WP-0001
clause 1 declares current by construction: `docs/decisions/` (61 files),
`docs/metrics.md`, `docs/conventions.md`, `AGENTS.md` and
`internal/pipeline/interpret/taxonomy/` (3 files). 204 tracked files minus 67
excluded leaves 137 rows in section 1.

**Classification.** `keep`: no record requires the file to change. `change`:
a record requires its content or its location to change; a move required by
ADR-0040 counts as a change. `delete`: a record removes the file's subject
entirely. `undecided`: the records do not determine the outcome; the missing
information is stated.

---

## 1. File inventory

| path | classification | governing ADRs | what must change |
|---|---|---|---|
| `.commitography.yml` | change | ADR-0026, ADR-0033 | `output_dir` is an operational setting inside the analysis-plane repository file; `hash_emails: false` would put raw addresses in the report. |
| `.gitattributes` | change | ADR-0040 | Line 14 names `internal/render/assets/**`, a package path the layout does not contain. |
| `.gitignore` | change | ADR-0040 | Comment on lines 12–14 names `internal/render/assets/`, a package path the layout does not contain. |
| `.goreleaser.yml` | change | ADR-0034, ADR-0031, ADR-0049 | Descriptions say "static dashboard"; archives ship `docs/report-schema.json`; no SBOM or signing; `BuildDate` injected. |
| `CHANGELOG.md` | change | ADR-0034, ADR-0004 | Describes the static page and the in-memory local dashboard as current behaviour. |
| `CLAUDE.md` | keep | — | |
| `Dockerfile` | change | ADR-0034, ADR-0046, ADR-0047, ADR-0029 | Default command writes a dashboard into `/repo/out` inside the analysed repository; system-level `safe.directory` relies on system git config; no mode switch. |
| `LICENSE` | keep | ADR-0002 | |
| `Makefile` | change | ADR-0049, ADR-0057, ADR-0040 | `BUILD_DATE` from the clock makes builds non-reproducible; no gate targets; targets name `internal/dockersmoke` and `internal/perfcheck`. |
| `PROMPTS.md` | keep | — | |
| `README.md` | change | ADR-0034, ADR-0020, ADR-0009, ADR-0011, ADR-0004, ADR-0031 | Documents static HTML output, `--no-blame`, `--wrapped` HTML, `--json`, opt-in per-author framing, phase documents and dead links. |
| `cmd/commitography/main.go` | change | ADR-0034, ADR-0020, ADR-0009, ADR-0041 | Flags `--wrapped`, `--json`, `--no-blame`, `--per-author`; help text describes a static HTML dashboard. |
| `cmd/commitography/serve.go` | change | ADR-0029, ADR-0010, ADR-0042 | No mode switch; non-loopback bind has no operator passphrase; job manager constructed implicitly inside the server package. |
| `cmd/commitography/serve_test.go` | keep | ADR-0029 | |
| `docs/project-overview.md` | change | ADR-0004, ADR-0034, ADR-0020, ADR-0011, ADR-0022 | Describes phases with dates, static HTML output, a three-stage pipeline, blame sampling and "no server, no database". |
| `docs/report-schema.json` | delete | ADR-0031, ADR-0032, ADR-0021 | Describes the superseded report document; WP-0002 clause 4 moves it to `docs/legacy/report-schema-v0.json`. |
| `docs/work/0000-template.md` | keep | ADR-0004 | |
| `docs/work/0001-repository-audit.md` | keep | — | |
| `docs/work/0002-neutralise-contradicting-documents.md` | keep | — | |
| `docs/work/0003-enforcement-skeleton.md` | keep | — | |
| `docs/work/0004-fixtures-and-golden-harness.md` | keep | — | |
| `docs/work/0005-package-layout-migration.md` | keep | — | |
| `docs/work/INDEX.md` | keep | ADR-0004 | |
| `go.mod` | change | ADR-0035, ADR-0049 | No pure Go SQLite driver is present; direct dependencies are not matched against an allow list. |
| `go.sum` | change | ADR-0035, ADR-0049 | Follows `go.mod`. |
| `internal/aggregate/code.go` | change | ADR-0020, ADR-0024, ADR-0040, ADR-0047, ADR-0053 | Runs sampled `git blame` and reads the working tree inside aggregation; content splits across `commit-size`, `files`, `ownership`, `hotspot`; limits are code constants. |
| `internal/aggregate/messages.go` | change | ADR-0024, ADR-0031, ADR-0032, ADR-0040 | Becomes the `messages` family with inputs, namespace, version and status; confidence is a boolean, not a `degraded` status; stores subject text. |
| `internal/aggregate/messages_test.go` | change | ADR-0040, ADR-0024 | Moves with the family; keyword rule order it asserts differs from `docs/metrics.md` section 4. |
| `internal/aggregate/notables.go` | change | ADR-0024, ADR-0032 | `notables` is not a catalogue family; bulk commits belong to `commit-size`, weekend ratio to `temporal`; other fields have no catalogue namespace. |
| `internal/aggregate/perauthor.go` | change | ADR-0009, ADR-0024, ADR-0028, ADR-0033, ADR-0018 | Opt-in `perAuthor` section outside any family namespace; carries raw or hashed email lists; person scope must be a lens over families. |
| `internal/aggregate/privacy.go` | change | ADR-0033 | Digest applied only when `hash_emails` is true and only in `perAuthor`; report must always carry display name plus digest and never a raw address. |
| `internal/aggregate/privacy_test.go` | change | ADR-0033, ADR-0040 | Asserts hashing is a configurable default. |
| `internal/aggregate/report.go` | change | ADR-0021, ADR-0026, ADR-0031, ADR-0032, ADR-0040, ADR-0042 | Report type has one integer schema version, no family status, no resolved configuration, no metadata section; `time.Now()` call; type and build logic share one package. |
| `internal/aggregate/schema_test.go` | change | ADR-0031, ADR-0020 | Validates against `docs/report-schema.json`; runs with `NoBlame`. |
| `internal/aggregate/social.go` | change | ADR-0024, ADR-0020, ADR-0053, ADR-0040 | Bus factor and knowledge concentration by commit counts move to line-based `ownership`; coupling to `coupling`; churn to `hotspot`; limits are code constants. |
| `internal/aggregate/social_test.go` | change | ADR-0024, ADR-0040 | Asserts commit-count bus factor and the `--per-author` naming rule. |
| `internal/aggregate/stats.go` | change | ADR-0040 | Shared rounding helpers move to `core`. |
| `internal/aggregate/stopwords.go` | undecided | ADR-0032, ADR-0024 | The word list serves `topWords`; no record decides whether a word metric exists. `docs/metrics.md` section 4 lists none, but that document is not a record. Missing: a decision on the word metric. |
| `internal/aggregate/temporal.go` | change | ADR-0024, ADR-0031, ADR-0032, ADR-0040 | Becomes the `temporal` family with inputs, namespace, version and status. |
| `internal/aggregate/temporal_test.go` | change | ADR-0040 | Moves with the family. |
| `internal/analysis/analysis.go` | change | ADR-0020, ADR-0034, ADR-0040 | Stage identifiers do not match the five stages; options carry `NoBlame`, `PerAuthor`, `Year`; package has no place in the layout. |
| `internal/analysis/consistency_test.go` | change | ADR-0040 | Moves with the package. |
| `internal/analysis/progress_test.go` | change | ADR-0020, ADR-0040 | Asserts progress windows keyed to the current stage names. |
| `internal/analysis/run.go` | change | ADR-0020, ADR-0008, ADR-0034, ADR-0042, ADR-0040 | Orchestrates preflight→collect→identity→filter→aggregate without replay or interpret; filters the report by Wrapped year. |
| `internal/analysis/run_test.go` | change | ADR-0020, ADR-0040 | Uses `NoBlame`. |
| `internal/cli/progress.go` | change | ADR-0042, ADR-0040 | Reads the process clock and `os.Stderr` directly; package has no place in the layout. |
| `internal/cli/report_test.go` | change | ADR-0040, ADR-0041 | Moves with the package. |
| `internal/cli/run.go` | change | ADR-0034, ADR-0041, ADR-0040 | Writes `index.html`, `wrapped-<year>.html` or `report.json`; exit code chosen by type switch at the call site; `ExitStrictWarn = 3`. |
| `internal/cli/run_test.go` | change | ADR-0034, ADR-0009, ADR-0040 | Asserts dashboard, Wrapped page, `--json` and opt-in per-author behaviour. |
| `internal/collect/collect_test.go` | change | ADR-0040 | Moves to `internal/pipeline/collect`. |
| `internal/collect/git.go` | delete | ADR-0047 | Re-exports `gitcmd` returning `*exec.Cmd`; only the git package may import process execution. |
| `internal/collect/gitlog.go` | change | ADR-0047, ADR-0044, ADR-0042, ADR-0017, ADR-0040 | Record format uses `\x01`/`\x1f` with newline-split numstat; bare `go`; `time.Now()`; no `--` separator; output not independently cached. |
| `internal/collect/gitlog_test.go` | change | ADR-0047, ADR-0040 | Asserts the `\x01` record format. |
| `internal/collect/preflight.go` | change | ADR-0047, ADR-0041, ADR-0034, ADR-0040 | Imports `os/exec`; error messages carry the repository path; shallow override records no degraded status. |
| `internal/collect/preflight_test.go` | change | ADR-0040 | Moves with the package. |
| `internal/collect/writer.go` | change | ADR-0017, ADR-0035, ADR-0040 | Uncompressed JSON history artifact with no production caller; persistence of commit records belongs to storage. |
| `internal/collect/writer_test.go` | change | ADR-0035, ADR-0040 | Asserts uncompressed compact JSON. |
| `internal/config/config.go` | change | ADR-0026, ADR-0042, ADR-0033, ADR-0053, ADR-0040 | Mixes `output_dir`/`theme` with analysis keys; no recency window or cardinality limits; mutable package variable `Warn`; `hash_emails` toggle. |
| `internal/config/config_test.go` | change | ADR-0026, ADR-0040 | Moves to `core`; asserts `theme` validation. |
| `internal/container/container.go` | change | ADR-0040, ADR-0042 | No destination in the layout; package-level `markers` and direct filesystem access. |
| `internal/dockersmoke/doc.go` | change | ADR-0040 | Package has no place in the layout. |
| `internal/dockersmoke/smoke_test.go` | change | ADR-0034, ADR-0029, ADR-0040 | Asserts the image CLI writes `index.html`; posts `noBlame`. |
| `internal/filter/commits.go` | change | ADR-0040, ADR-0024 | Shared definitions (analysed commit, bulk commit) belong in `core`; `CouplingMaxFilesPerCommit` is consumed by coupling. |
| `internal/filter/filter_test.go` | change | ADR-0040 | Moves with the package. |
| `internal/filter/paths.go` | change | ADR-0020, ADR-0042, ADR-0046, ADR-0040 | Reads `.gitattributes` from the working tree outside replay; package-level `caseInsensitiveFS`. |
| `internal/gitcmd/gitcmd.go` | change | ADR-0047, ADR-0044, ADR-0040 | No environment sanitisation, mandatory flags, output size limit or process-group termination; `Lines` splits on newline; moves to `internal/git`. |
| `internal/gitcmd/gitcmd_test.go` | change | ADR-0040 | Moves to `internal/git`. |
| `internal/identity/identity.go` | change | ADR-0033, ADR-0051, ADR-0010, ADR-0040 | Canonical ID is the raw normalized email; no integer index or digest; no candidate proposals; moves to `core`. |
| `internal/identity/identity_test.go` | change | ADR-0033, ADR-0040 | Asserts email-keyed IDs. |
| `internal/jobs/jobs.go` | change | ADR-0027, ADR-0044, ADR-0042, ADR-0040 | One global active slot, in-memory records, bare `go`, `time.Now` default; no job classes, per-repository persistent lock or persisted job record. |
| `internal/jobs/jobs_test.go` | change | ADR-0027, ADR-0040 | Asserts one active job globally and ten retained in memory. |
| `internal/model/model.go` | change | ADR-0040, ADR-0033, ADR-0017, ADR-0042 | `RepositoryInfo.Path` is absolute; no content-derived repository identity; `GeneratedAt` on the history artifact. |
| `internal/perfcheck/doc.go` | change | ADR-0040, ADR-0050 | Package has no place in the layout. |
| `internal/perfcheck/perf_test.go` | change | ADR-0050, ADR-0054, ADR-0019, ADR-0034, ADR-0040 | Absolute duration bounds in code; runs `--no-blame`; not a CI gate. |
| `internal/render/assets/app.css` | change | ADR-0036, ADR-0038, ADR-0049, ADR-0040 | Built from the current styles; regenerated from rewritten source and verified against it. |
| `internal/render/assets/app.js` | change | ADR-0036, ADR-0038, ADR-0049, ADR-0040 | Contains MUI and emotion; single IIFE; regenerated and verified against source. |
| `internal/render/bundle_test.go` | change | ADR-0038, ADR-0040 | Allow-lists `https://mui.com/production-error/`. |
| `internal/render/render.go` | change | ADR-0034, ADR-0036, ADR-0040 | Writes self-contained `index.html` and `wrapped-<year>.html`; only `report.json` writing and asset embedding survive, in layout locations. |
| `internal/render/render_test.go` | change | ADR-0034, ADR-0040 | Asserts HTML page output, inlining and Wrapped pages. |
| `internal/server/api.go` | change | ADR-0043, ADR-0029, ADR-0033, ADR-0041, ADR-0020 | Polling instead of SSE; `DELETE` for state change; capabilities not mode-derived; `repoPath` in status; free-form error codes; `noBlame` option. |
| `internal/server/api_test.go` | change | ADR-0043, ADR-0020 | Asserts `DELETE` route and `noBlame`/`perAuthor` pass-through. |
| `internal/server/handler.go` | change | ADR-0036, ADR-0041, ADR-0010, ADR-0029 | Static shell with no metadata injection; `panic` in `NewApp`/`newSessionToken`; no recovery layer; host allow list limited to loopback names. |
| `internal/server/handler_test.go` | keep | ADR-0036 | |
| `internal/server/host_test.go` | change | ADR-0029, ADR-0005 | Asserts only loopback host names are served; public mode is served under a public host name. |
| `internal/server/listener.go` | change | ADR-0047, ADR-0044, ADR-0029, ADR-0042 | Imports `os/exec` to open a browser; bare `go`; non-loopback warning without passphrase. |
| `internal/server/listener_test.go` | change | ADR-0029 | Asserts warnings for non-loopback listeners without passphrase. |
| `internal/server/matrix_test.go` | change | ADR-0043, ADR-0027 | Asserts the current endpoint set and one-active-job conflict. |
| `internal/server/path.go` | change | ADR-0029, ADR-0046 | Local-path analysis is reachable with no mode check; containment does not cover working-tree symlinks or clone targets. |
| `internal/server/path_test.go` | keep | ADR-0046 | |
| `internal/server/path_windows_test.go` | keep | ADR-0046 | |
| `internal/server/resolve_other.go` | keep | ADR-0046 | |
| `internal/server/resolve_windows.go` | change | ADR-0042 | Package-level `procGetFinalPathNameByHandleW` variable. |
| `internal/server/security_test.go` | change | ADR-0043 | Method matrix includes `DELETE`. |
| `internal/server/securitymatrix_test.go` | change | ADR-0043, ADR-0031 | Reads `docs/report-schema.json`; asserts `DELETE` as state-changing. |
| `internal/version/version.go` | change | ADR-0042, ADR-0040, ADR-0049 | Package-level variables set by `-ldflags -X`; package has no place in the layout. |
| `testdata/build-fixtures.sh` | change | ADR-0019 | Fixture set differs from WP-0004; `binary` fixture adds a commit only where the filesystem accepts a quote, so hashes differ by platform. |
| `testdata/fixtures/.gitkeep` | keep | — | |
| `web/e2e/a11y.mjs` | change | ADR-0038 | Selectors depend on `.Mui-*` classes. |
| `web/e2e/audit.mjs` | change | ADR-0034, ADR-0009, ADR-0020 | Generates static pages with `--wrapped` and `--per-author` and audits them offline; posts `noBlame`. |
| `web/e2e/cdp.mjs` | keep | — | |
| `web/e2e/repositories.mjs` | keep | — | |
| `web/index.html` | change | ADR-0034, ADR-0036 | Comment describes a single inlined file assembled by `internal/render`. |
| `web/package-lock.json` | change | ADR-0038, ADR-0049 | Locks `@mui/*` and `@emotion/*` trees. |
| `web/package.json` | change | ADR-0038, ADR-0037, ADR-0049 | Depends on `@mui/material`, `@emotion/react`, `@emotion/styled`; no headless primitive or layout-mathematics library. |
| `web/src/api/client.ts` | change | ADR-0043, ADR-0029, ADR-0020 | Polling contract, `noBlame` option, hard-coded capability fields; header cites `docs/phase-1.5/m0-contracts.md`. |
| `web/src/app/AppShell.tsx` | change | ADR-0038, ADR-0028 | Built on MUI; navigation is analyze/recent/job, not registry/repository/Wrapped. |
| `web/src/app/JobReport.tsx` | change | ADR-0038, ADR-0034 | Built on MUI; comment refers to the static CLI page. |
| `web/src/app/JobView.test.tsx` | change | ADR-0043, ADR-0038 | Asserts polling behaviour. |
| `web/src/app/JobView.tsx` | change | ADR-0038, ADR-0043 | Built on MUI; progress by polling. |
| `web/src/app/RecentJobs.tsx` | change | ADR-0038, ADR-0011 | Built on MUI; lists ten in-memory jobs instead of a persistent registry. |
| `web/src/app/RepositoryForm.test.tsx` | change | ADR-0009, ADR-0016 | Asserts per-contributor warning and path-only input. |
| `web/src/app/RepositoryForm.tsx` | change | ADR-0038, ADR-0020, ADR-0009, ADR-0016, ADR-0029 | Built on MUI; "Skip blame" and "per-author" options; local path only. |
| `web/src/app/a11y.ts` | change | ADR-0038 | Raw pixel values written for MUI `sx`. |
| `web/src/app/format.ts` | change | ADR-0020 | Stage labels mirror the current stage identifiers. |
| `web/src/app/logic.test.ts` | change | ADR-0020, ADR-0041 | Asserts current stage labels and error codes. |
| `web/src/app/outcomes.ts` | change | ADR-0038 | Imports MUI `AlertColor` and `ChipProps` types. |
| `web/src/app/routes.ts` | change | ADR-0028, ADR-0010 | Hash routes for jobs; no repository, lens or identity-selection state. |
| `web/src/app/startErrors.ts` | change | ADR-0041 | Maps free-form server codes rather than the enumerated reason set. |
| `web/src/app/useJobStatus.ts` | change | ADR-0043 | Polls instead of consuming server-sent events. |
| `web/src/main.test.tsx` | change | ADR-0034 | Asserts static-page bootstrap from `data-mode`. |
| `web/src/main.tsx` | change | ADR-0034, ADR-0053 | Boots static `dashboard`/`wrapped` modes; Wrapped is not code-split. |
| `web/src/report/Dashboard.test.tsx` | change | ADR-0032, ADR-0034, ADR-0009 | Asserts sections without data are omitted; static page landmarks; opt-in per-author. |
| `web/src/report/Dashboard.tsx` | change | ADR-0032, ADR-0028, ADR-0034 | Omits empty sections; static-page theme toggle. |
| `web/src/report/Figure.tsx` | keep | ADR-0032, ADR-0037 | |
| `web/src/report/Wrapped.tsx` | change | ADR-0015, ADR-0023, ADR-0038, ADR-0053 | Repository subject only; exports by redrawing text on a canvas; raw colour fallbacks; bundled with the dashboard. |
| `web/src/report/charts.tsx` | keep | ADR-0037 | |
| `web/src/report/embedded.ts` | delete | ADR-0034 | Reads a report inlined into a static page. |
| `web/src/report/format.ts` | keep | — | |
| `web/src/report/heading.tsx` | change | ADR-0034 | Heading base distinguishes a static page from the application. |
| `web/src/report/parts.tsx` | keep | — | |
| `web/src/report/sections.tsx` | change | ADR-0024, ADR-0031, ADR-0032, ADR-0009 | Renders the current report shape; no skipped or degraded states; per-author section. |
| `web/src/styles.css` | change | ADR-0038, ADR-0034 | Raw colour and pixel values; `.cg-static` scoping; styles shared with MUI-owned page. |
| `web/src/test/sampleReport.ts` | change | ADR-0031 | Fixture of the current report shape. |
| `web/src/types.ts` | change | ADR-0031, ADR-0021, ADR-0034 | Hand-mirrors the current report; `Mode` type for static pages. |
| `web/src/ui/theme.ts` | delete | ADR-0038 | MUI `createTheme` configuration. |
| `web/tsconfig.json` | keep | — | |
| `web/vite.config.ts` | change | ADR-0036, ADR-0053, ADR-0049, ADR-0040 | Single IIFE with code splitting disabled for inlining; `outDir` is `../internal/render/assets`. |
| `web/vitest.config.ts` | keep | — | |

---

## 2. Stale references

Searches run with `git grep` over tracked files outside `docs/decisions/`,
`docs/work/`, and the other WP-0001 exclusions. `web/package-lock.json` and the
minified `internal/render/assets/*` were excluded from the text search as
machine-written; both are classified in section 1. Matches that only
coincidentally contain a search term (for example `fmt.Sprintf` for "sprint",
the CSS custom property `--wrapped-bg`, or the Wrapped PNG file name) are not
listed. Contiguous lines carrying the same reference are listed as a range.

### Static HTML output (ADR-0034)

| file | line | the reference | what makes it stale |
|---|---|---|---|
| `.goreleaser.yml` | 119 | "self-contained static dashboard" | CLI emits `report.json` only (ADR-0034 clause 1–2). |
| `.goreleaser.yml` | 133 | "self-contained static dashboard" | ADR-0034 clause 1–2. |
| `CHANGELOG.md` | 14–15 | "The static dashboard … The static page stays a single self-contained file." | ADR-0034 clause 2. |
| `README.md` | 3 | "self-contained dashboard" | ADR-0034 clause 2. |
| `README.md` | 18 | "The result is one HTML file" | ADR-0034 clause 2. |
| `README.md` | 66 | "`index.html` inlines its styles, scripts and data" | ADR-0034 clause 2. |
| `README.md` | 172–174 | `open out/index.html` (three OS rows) | ADR-0034 clause 1. |
| `README.md` | 318 | "CLI mode, the default, writes a static dashboard" | ADR-0034 clause 2. |
| `README.md` | 391 | "Open `out/index.html`" | ADR-0034 clause 1. |
| `README.md` | 474 | "`wrapped-<year>.html` … instead of `index.html`" | ADR-0034 clause 1–2. |
| `README.md` | 609 | `out/index.html` output row | ADR-0034 clause 1. |
| `cmd/commitography/main.go` | 1–2 | "turns … history into a self-contained static dashboard" | ADR-0034 clause 2. |
| `cmd/commitography/main.go` | 30 | `Short: "… into a static dashboard"` | ADR-0034 clause 2. |
| `cmd/commitography/main.go` | 32 | "self-contained HTML dashboard" in `Long` | ADR-0034 clause 2. |
| `docs/project-overview.md` | 4 | "self-contained static dashboard" | ADR-0034 clause 2. |
| `docs/project-overview.md` | 10 | "produces a static HTML dashboard" | ADR-0034 clause 2. |
| `docs/project-overview.md` | 41 | "Output: Self-contained static HTML" | ADR-0034 clause 2. |
| `docs/project-overview.md` | 58 | "self-contained HTML dashboard" goal | ADR-0034 clause 2. |
| `docs/project-overview.md` | 96 | "render → out/index.html (static site with JSON embedded)" | ADR-0034 clause 2; ADR-0020 clause 7. |
| `docs/project-overview.md` | 134 | `open out/index.html` | ADR-0034 clause 1. |
| `internal/cli/run.go` | 109 | `render.RenderWrapped(...)` | ADR-0034 clause 2. |
| `internal/cli/run.go` | 126 | `render.IndexFile` progress path | ADR-0034 clause 1. |
| `internal/cli/run.go` | 130 | `render.IndexFile` done path | ADR-0034 clause 1. |
| `internal/cli/run_test.go` | 72 | `render.IndexFile` | ADR-0034 clause 1. |
| `internal/cli/run_test.go` | 75 | "index.html was not written" | ADR-0034 clause 1. |
| `internal/cli/run_test.go` | 78 | "the bundle does not appear to be inlined" | ADR-0034 clause 2. |
| `internal/cli/run_test.go` | 310–311 | "--wrapped should not also write index.html" | ADR-0034 clause 1–2. |
| `internal/dockersmoke/smoke_test.go` | 151 | `"out", "index.html"` | ADR-0034 clause 1. |
| `internal/dockersmoke/smoke_test.go` | 153 | "out/index.html is missing or is not the dashboard" | ADR-0034 clause 1. |
| `internal/dockersmoke/smoke_test.go` | 179 | `[]string{"index.html", "report.json"}` | ADR-0034 clause 1. |
| `internal/render/render.go` | 1 | "turns a report into the self-contained HTML dashboard" | ADR-0034 clause 2. |
| `internal/render/render.go` | 28 | "IndexFile and ReportFile are the only files Render ever writes." | ADR-0034 clause 1. |
| `internal/render/render.go` | 30 | `IndexFile = "index.html"` | ADR-0034 clause 1. |
| `internal/render/render.go` | 34–36 | "CSS, JS and the report are inlined" | ADR-0034 clause 2. |
| `internal/render/render.go` | 70 | "It produces exactly one page, index.html, with everything inlined" | ADR-0034 clause 2. |
| `internal/render/render.go` | 88 | `writeFile(filepath.Join(outputDir, IndexFile), page)` | ADR-0034 clause 1. |
| `internal/render/render.go` | 91 | "writes the year-in-review page, again as one self-contained file" | ADR-0034 clause 2. |
| `internal/render/render.go` | 93 | `func RenderWrapped(...)` | ADR-0034 clause 2. |
| `internal/render/render_test.go` | 62 | "want exactly index.html and report.json" | ADR-0034 clause 1. |
| `internal/render/render_test.go` | 64 | `[]string{IndexFile, ReportFile}` | ADR-0034 clause 1. |
| `internal/render/render_test.go` | 76 | `filepath.Join(dir, IndexFile)` | ADR-0034 clause 1. |
| `internal/render/render_test.go` | 110 | `filepath.Join(dir, IndexFile)` | ADR-0034 clause 1. |
| `internal/render/render_test.go` | 141 | `filepath.Join(src, IndexFile)` | ADR-0034 clause 1. |
| `internal/render/render_test.go` | 179 | `IndexFile+".tmp"` | ADR-0034 clause 1. |
| `internal/render/render_test.go` | 184 | `func TestRenderWrapped` | ADR-0034 clause 2. |
| `internal/render/render_test.go` | 188–189 | `RenderWrapped(...)` | ADR-0034 clause 2. |
| `internal/render/render_test.go` | 206–207 | `RenderWrapped(...)` | ADR-0034 clause 2. |
| `web/e2e/audit.mjs` | 3–4 | "the static pages opened offline" | ADR-0034 clause 2. |
| `web/e2e/audit.mjs` | 278 | `step('static pages offline', …)` | ADR-0034 clause 2. |
| `web/e2e/audit.mjs` | 281–283 | `index.html`, "the static dashboard" | ADR-0034 clause 1–2. |
| `web/e2e/audit.mjs` | 288 | "the static pages make no network request" | ADR-0034 clause 2. |
| `web/index.html` | 11–12 | "internal/render, which inlines … into one self-contained file" | ADR-0034 clause 2. |
| `web/src/main.tsx` | 15 | "The Go renderer marks every static page with its mode" | ADR-0034 clause 2. |
| `web/src/main.test.tsx` | 34 | `describe('static bootstrap', …)` | ADR-0034 clause 2. |
| `web/src/main.test.tsx` | 76 | `describe('static and server-fetched reports', …)` | ADR-0034 clause 2. |
| `web/src/app/JobReport.tsx` | 118 | "same dashboard components as the static CLI page" | ADR-0034 clause 2. |
| `web/src/report/Dashboard.test.tsx` | 54 | "uses page landmarks and an h1 on the static page" | ADR-0034 clause 2. |
| `web/src/report/Dashboard.tsx` | 19 | "The static page follows the system." | ADR-0034 clause 2. |
| `web/src/report/Dashboard.tsx` | 46 | "The static CLI page and the local application render this same component" | ADR-0034 clause 2. |
| `web/src/report/Dashboard.tsx` | 99 | "Switches the static page theme." | ADR-0034 clause 2. |
| `web/src/report/embedded.ts` | 5 | "Reads the report the Go renderer inlined into a static page." | ADR-0034 clause 2. |
| `web/src/report/heading.tsx` | 5 | "The static page starts at h1" | ADR-0034 clause 2. |
| `web/src/styles.css` | 4–5 | "the static CLI page, whose <html> carries .cg-static" | ADR-0034 clause 2. |
| `web/vite.config.ts` | 3–6 | "inlined into a single HTML file … code splitting … disabled for that reason" | ADR-0034 clause 2; ADR-0053 clause 5. |

### `--no-blame` and blame-derived metrics (ADR-0020)

| file | line | the reference | what makes it stale |
|---|---|---|---|
| `CHANGELOG.md` | 60 | "skipping blame helps on large repositories" | No flag to skip blame exists (ADR-0020). |
| `README.md` | 213 | "Analyze a very large repository quickly by skipping `git blame`" | ADR-0020 clause 4. |
| `README.md` | 216 | `commitography ./monorepo --no-blame` | ADR-0020. |
| `README.md` | 271 | "Advanced options mirror the CLI flags: skip blame, per-author, …" | ADR-0020; ADR-0009 clause 1. |
| `README.md` | 392 | `/repo -o /out --no-blame --anonymize` | ADR-0020. |
| `README.md` | 476 | `--no-blame` flag row | ADR-0020. |
| `README.md` | 650 | "Code age … from `git blame` over a deterministic sample … Skipped with `--no-blame`" | ADR-0020 clause 3–4; ownership is replay-derived over all lines. |
| `README.md` | 703 | "`git blame` is by far the most expensive step" | ADR-0020 clause 4. |
| `README.md` | 705 | "about half a second with `--no-blame`" | ADR-0020. |
| `README.md` | 707 | "Use **`--no-blame`**" | ADR-0020. |
| `README.md` | 730 | "Use `--no-blame`" troubleshooting row | ADR-0020. |
| `cmd/commitography/main.go` | 70 | `f.BoolVar(&opts.NoBlame, "no-blame", …)` | ADR-0020. |
| `docs/project-overview.md` | 219 | "Surviving line ratio … (sampled blame)" | ADR-0020 clause 3–4. |
| `docs/project-overview.md` | 291 | "`--no-blame` fast mode, incremental cache in Phase 2" | ADR-0020; ADR-0004. |
| `docs/report-schema.json` | 238 | "Null when blame was skipped with --no-blame" | ADR-0020. |
| `internal/aggregate/code.go` | 22–25 | `blameSampleSize` "Blame is the most expensive operation … sampled" | ADR-0020 clause 4. |
| `internal/aggregate/code.go` | 33 | "whether blame would say anything meaningful" | ADR-0020 clause 4. |
| `internal/aggregate/code.go` | 93 | "SurvivingFromFirstYear is null when blame was skipped" | ADR-0020. |
| `internal/aggregate/code.go` | 189 | `if in.NoBlame {` | ADR-0020. |
| `internal/aggregate/code.go` | 193–203 | blame sample, `"blame"` progress stage, `blameYears` call | ADR-0020 clause 4. |
| `internal/aggregate/code.go` | 316 | "files blame cannot say anything useful about" | ADR-0020 clause 4. |
| `internal/aggregate/code.go` | 366–381 | `blameYears`, `git blame --line-porcelain`, "blame failed for %s" | ADR-0020 clause 4. |
| `internal/aggregate/report.go` | 74–76 | "NoBlame skips the sampled-blame metrics" | ADR-0020. |
| `internal/aggregate/schema_test.go` | 75 | `NoBlame: true` | ADR-0020. |
| `internal/aggregate/schema_test.go` | 103 | `{"with-blame", … in.NoBlame = false}` | ADR-0020. |
| `internal/analysis/analysis.go` | 26 | `NoBlame bool` | ADR-0020. |
| `internal/analysis/run.go` | 122 | `NoBlame: opts.NoBlame` | ADR-0020. |
| `internal/analysis/run.go` | 141 | `stage == "blame"` | ADR-0020 clause 4. |
| `internal/analysis/run_test.go` | 30 | `NoBlame: true` | ADR-0020. |
| `internal/analysis/run_test.go` | 50 | `NoBlame: true` | ADR-0020. |
| `internal/analysis/run_test.go` | 70 | `NoBlame: true` | ADR-0020. |
| `internal/analysis/run_test.go` | 110 | `NoBlame: true` | ADR-0020. |
| `internal/cli/run.go` | 53 | `NoBlame bool` | ADR-0020. |
| `internal/cli/run.go` | 87 | `NoBlame: opts.NoBlame` | ADR-0020. |
| `internal/cli/run_test.go` | 35 | "a quiet run with blame skipped" | ADR-0020. |
| `internal/cli/run_test.go` | 42 | `NoBlame: true` | ADR-0020. |
| `internal/cli/run_test.go` | 103 | `NoBlame: true` | ADR-0020. |
| `internal/dockersmoke/smoke_test.go` | 536 | `"noBlame": false` | ADR-0020. |
| `internal/jobs/jobs.go` | 349 | "such as blame diagnostics" | ADR-0020 clause 4. |
| `internal/jobs/jobs_test.go` | 241 | `"blame: sampled"` | ADR-0020 clause 4. |
| `internal/jobs/jobs_test.go` | 270 | `"blame: sampled"` | ADR-0020 clause 4. |
| `internal/perfcheck/perf_test.go` | 85 | "so blame has real history to walk" | ADR-0020 clause 4. |
| `internal/perfcheck/perf_test.go` | 134 | `timeAnalysis(…, noBlame bool)` | ADR-0020. |
| `internal/perfcheck/perf_test.go` | 137 | `NoBlame: noBlame` | ADR-0020. |
| `internal/perfcheck/perf_test.go` | 157–159 | "with blame … --no-blame" timings | ADR-0020. |
| `internal/perfcheck/perf_test.go` | 240 | `start(repo string, noBlame bool)` | ADR-0020. |
| `internal/perfcheck/perf_test.go` | 248 | `"noBlame": noBlame` | ADR-0020. |
| `internal/perfcheck/perf_test.go` | 325–334 | `noBlame` stage targets; "waiting out blame on 15,000 commits" | ADR-0020. |
| `internal/perfcheck/perf_test.go` | 459–468 | `--no-blame` native CLI runs | ADR-0020. |
| `internal/perfcheck/perf_test.go` | 539–548 | `--no-blame` Docker CLI runs | ADR-0020. |
| `internal/server/api.go` | 59 | `NoBlame bool \`json:"noBlame"\`` | ADR-0020. |
| `internal/server/api.go` | 158 | `NoBlame: request.Options.NoBlame` | ADR-0020. |
| `internal/server/api_test.go` | 82 | `"options":{"noBlame":true}` | ADR-0020. |
| `internal/server/api_test.go` | 157 | `"noBlame":true` | ADR-0020. |
| `internal/server/api_test.go` | 172 | `!opts.NoBlame` | ADR-0020. |
| `internal/server/securitymatrix_test.go` | 224 | `"options":{"noBlame":true}` | ADR-0020. |
| `web/e2e/audit.mjs` | 141 | `noBlame: false` | ADR-0020. |
| `web/e2e/audit.mjs` | 223 | "Blame stays on to keep those stages long enough." | ADR-0020. |
| `web/src/api/client.ts` | 59 | `noBlame: boolean` | ADR-0020. |
| `web/src/app/JobView.tsx` | 64 | "Blame can run after the …" | ADR-0020 clause 4. |
| `web/src/app/RepositoryForm.tsx` | 31 | `noBlame: false` | ADR-0020. |
| `web/src/app/RepositoryForm.tsx` | 42 | `'noBlame'` option key | ADR-0020. |
| `web/src/app/RepositoryForm.tsx` | 46–47 | `key: 'noBlame'`, `label: 'Skip blame'` | ADR-0020. |

### Phase names, plan references and dated statements (ADR-0004)

| file | line | the reference | what makes it stale |
|---|---|---|---|
| `README.md` | 715 | `docs/phase-1.5/m6-verification.md` | Phase document; not tracked (ADR-0004). |
| `README.md` | 776 | "Vision, principles, architecture and roadmap" | Roadmap is a schedule (ADR-0004). |
| `README.md` | 779 | `docs/phase-1.5.md` "scope, limitations and exit criteria" | Phase document; not tracked. |
| `README.md` | 780 | `docs/phase-2.md` "Planned CI/CD integration" | Phase document; not tracked. |
| `README.md` | 781 | `docs/phase-3.md` "Planned server mode" | Phase document; not tracked; server mode is decided (ADR-0011). |
| `docs/project-overview.md` | 109 | "See departure 8 in `phase-1-detailed.md`" | Phase document; not tracked. |
| `docs/project-overview.md` | 117–119 | "Phase 1.5 … Phase 2 … Phase 3 may use persistent storage … and accounts" | ADR-0004; ADR-0010 clause 1; ADR-0035. |
| `docs/project-overview.md` | 124 | "## 6. Phases" | ADR-0004. |
| `docs/project-overview.md` | 130 | "### Phase 1 — Local CLI (MVP)" | ADR-0004. |
| `docs/project-overview.md` | 137 | "No server, no database … the entirety of the initial release … `phase-1-detailed.md`" | ADR-0004; ADR-0011; ADR-0035. |
| `docs/project-overview.md` | 139 | "Closed for development on 2026-09-05. Nine of twelve exit criteria …" | Dated statement and exit criteria (ADR-0004). |
| `docs/project-overview.md` | 141 | "### Phase 1.5 — Local Web Dashboard and Runner" | ADR-0004. |
| `docs/project-overview.md` | 147 | "Phase 1.5 wraps the Phase 1 analysis engine" | ADR-0004. |
| `docs/project-overview.md` | 154–157 | "Implemented on `main` on 2026-09-13 … exit criterion … `phase-1.5.md` … `phase-1.5-detailed.md`" | Dated statement; phase documents (ADR-0004). |
| `docs/project-overview.md` | 159 | "### Phase 2 — CI/CD integration" | ADR-0004. |
| `docs/project-overview.md` | 161 | "Scope described in `phase-2.md`" | ADR-0004. |
| `docs/project-overview.md` | 163 | "### Phase 3 — Server mode" | ADR-0004. |
| `docs/project-overview.md` | 165 | "Scope described in `phase-3.md`" | ADR-0004. |
| `docs/project-overview.md` | 191 | "Phase 1 custom SVG, Phase 1.5 React/MUI shell" | ADR-0004; ADR-0038 clause 1. |
| `docs/project-overview.md` | 193 | "Optional Phase 2 cache; not required by Phase 1.5" | ADR-0004; ADR-0017. |
| `internal/analysis/analysis.go` | 49 | "the Phase 1.5 job status contract" | ADR-0004. |
| `internal/analysis/analysis.go` | 64 | "added by the orchestration work package" | Refers to a prior plan; `Run` exists. |
| `internal/analysis/run.go` | 19 | "It keeps the Phase 1 pipeline order" | ADR-0004; ADR-0020 clause 1. |
| `internal/cli/run.go` | 24 | "reserved for --strict, not implemented in Phase 1" | ADR-0004; ADR-0034 clause 5 defines codes 0, 1, 2. |
| `internal/collect/gitlog.go` | 96 | "departure from the letter of Task 1.3" | Prior plan task (ADR-0004). |
| `internal/config/config.go` | 68 | "as Task 3.2 requires" | Prior plan task (ADR-0004). |
| `internal/config/config.go` | 72 | "exit criterion 4 held only for single-package repositories" | Prior plan exit criterion (ADR-0004). |
| `internal/filter/filter_test.go` | 140 | "exit criterion 4 held only for repositories" | Prior plan exit criterion (ADR-0004). |
| `internal/server/handler.go` | 33–35 | "return 404 until their handlers are registered by the later server milestones" | Milestone reference (ADR-0004); handlers are registered. |
| `web/src/api/client.ts` | 1 | "Mirrors the Phase 1.5 local API contract in docs/phase-1.5/m0-contracts.md" | Phase document; not tracked (ADR-0004); API shape is ADR-0043. |

### `wrapped-<year>.html` as CLI output (ADR-0034, ADR-0008)

| file | line | the reference | what makes it stale |
|---|---|---|---|
| `README.md` | 76 | "`--wrapped` produces a shareable card-based page" | ADR-0034 clause 2; ADR-0008 clause 3. |
| `README.md` | 201 | "Create a year-in-review page … (`out/wrapped-2025.html`)" | ADR-0034 clause 2. |
| `README.md` | 204 | `commitography ./api --wrapped 2025` | ADR-0034 clause 2. |
| `README.md` | 474 | `--wrapped` flag row, `wrapped-<year>.html` | ADR-0034 clause 2. |
| `README.md` | 611 | `out/wrapped-<year>.html` output row | ADR-0034 clause 1. |
| `cmd/commitography/main.go` | 64 | `f.IntVar(&opts.Wrapped, "wrapped", …)` | ADR-0034 clause 2. |
| `docs/project-overview.md` | 242 | `commitography ./repo --wrapped 2026` | ADR-0034 clause 2. |
| `internal/cli/run.go` | 106–107 | `render.WrappedFileName(opts.Wrapped)` | ADR-0034 clause 1–2. |
| `internal/cli/run_test.go` | 305 | `render.WrappedFileName(2026)` | ADR-0034 clause 2. |
| `internal/cli/run_test.go` | 311 | "--wrapped should not also write index.html" | ADR-0034 clause 2. |
| `internal/render/render.go` | 113 | `WrappedFileName(year)` | ADR-0034 clause 2. |
| `internal/render/render.go` | 116–118 | `func WrappedFileName` → `"wrapped-%d.html"` | ADR-0034 clause 2. |
| `internal/render/render_test.go` | 191 | `WrappedFileName(2026)` | ADR-0034 clause 2. |
| `internal/render/render_test.go` | 209 | `WrappedFileName(2026)` | ADR-0034 clause 2. |
| `web/e2e/audit.mjs` | 111 | `'--wrapped', '2025'` CLI invocation | ADR-0034 clause 2. |
| `web/e2e/audit.mjs` | 284 | `wrapped-2025.html` | ADR-0034 clause 2. |

### `report-schema.json` (ADR-0031, ADR-0021)

| file | line | the reference | what makes it stale |
|---|---|---|---|
| `.goreleaser.yml` | 54 | `docs/report-schema.json` in release archives | Schema describes the superseded document (ADR-0031). |
| `README.md` | 75 | "follows a documented, versioned JSON schema" link | ADR-0031. |
| `README.md` | 610 | "validated against `docs/report-schema.json`" | ADR-0031. |
| `README.md` | 778 | `docs/report-schema.json` documentation row | ADR-0031. |
| `docs/report-schema.json` | 3 | `$id` self-reference | ADR-0031. |
| `internal/aggregate/schema_test.go` | 28 | `filepath.Join("..", "..", "docs", "report-schema.json")` | ADR-0031; WP-0002 clause 4 moves the file. |
| `internal/aggregate/schema_test.go` | 124 | "report does not validate against docs/report-schema.json" | ADR-0031. |
| `internal/server/securitymatrix_test.go` | 268 | `filepath.Join(testRepoPath(t), "docs", "report-schema.json")` | ADR-0031; WP-0002 clause 4 moves the file. |

### `project-overview.md`

| file | line | the reference | what makes it stale |
|---|---|---|---|
| `README.md` | 776 | `docs/project-overview.md` "Vision, principles, architecture and roadmap" | The document describes phases and removed capabilities (ADR-0004, ADR-0034); binding source is `docs/decisions/INDEX.md` (ADR-0001). |

### Removed component library (ADR-0038)

| file | line | the reference | what makes it stale |
|---|---|---|---|
| `README.md` | 762 | "React and MUI application" | ADR-0038 clause 1. |
| `docs/project-overview.md` | 191 | "React/MUI shell" | ADR-0038 clause 1. |
| `internal/render/bundle_test.go` | 11 | "documentation pages React and MUI link from" | ADR-0038 clause 1. |
| `internal/render/bundle_test.go` | 17 | `"https://mui.com/production-error/"` | ADR-0038 clause 1. |
| `web/e2e/a11y.mjs` | 80 | `.Mui-disabled` selector | ADR-0038 clause 1. |
| `web/e2e/a11y.mjs` | 88 | "such as MUI's outline notch" | ADR-0038 clause 1. |
| `web/e2e/a11y.mjs` | 119 | `.Mui-focusVisible, .Mui-focused` selectors | ADR-0038 clause 1. |
| `web/package.json` | 24–26 | `@emotion/react`, `@emotion/styled`, `@mui/material` | ADR-0038 clause 1. |
| `web/src/app/AppShell.tsx` | 3–17 | imports from `@mui/material` (15 lines) | ADR-0038 clause 1. |
| `web/src/app/AppShell.tsx` | 160–161 | `<ThemeProvider>`, `<CssBaseline />` | ADR-0038 clause 1. |
| `web/src/app/AppShell.tsx` | 271 | `</ThemeProvider>` | ADR-0038 clause 1. |
| `web/src/app/JobReport.tsx` | 3–9 | imports from `@mui/material` | ADR-0038 clause 1. |
| `web/src/app/JobView.tsx` | 3–15 | imports from `@mui/material` | ADR-0038 clause 1. |
| `web/src/app/RecentJobs.tsx` | 3–11 | imports from `@mui/material` | ADR-0038 clause 1. |
| `web/src/app/RepositoryForm.tsx` | 3–13 | imports from `@mui/material` | ADR-0038 clause 1. |
| `web/src/app/a11y.ts` | 3 | "MUI's sx reads 1 as 100%" | ADR-0038 clause 1. |
| `web/src/app/outcomes.ts` | 1–2 | `AlertColor`, `ChipProps` from `@mui/material` | ADR-0038 clause 1. |
| `web/src/styles.css` | 5 | "the local application, where MUI owns the page" | ADR-0038 clause 1. |
| `web/src/ui/theme.ts` | 1–2 | `createTheme`, `PaletteMode` from `@mui/material/styles` | ADR-0038 clause 1. |
| `web/src/ui/theme.ts` | 30 | `return createTheme({` | ADR-0038 clause 1. |
| `web/src/ui/theme.ts` | 52 | `MuiCssBaseline` override | ADR-0038 clause 1. |

### Other removed or contradicted concepts found by the same searches

| file | line | the reference | what makes it stale |
|---|---|---|---|
| `CHANGELOG.md` | 32 | `docs/docker.md` link | Target not tracked. |
| `CHANGELOG.md` | 54–57 | "Jobs and reports live in memory … no accounts, remote cloning, scheduling or persistent storage" | ADR-0011, ADR-0016, ADR-0017, ADR-0035. |
| `README.md` | 4 | "no server" | ADR-0011. |
| `README.md` | 21–22 | "per-contributor figures are strictly opt-in" | ADR-0009 clause 1. |
| `README.md` | 207–210 | "Produce only the JSON report" `--json` | ADR-0034 clause 1 makes it the only output. |
| `README.md` | 225–228 | "Include the opt-in per-contributor section" `--per-author` | ADR-0009 clause 1; ADR-0028 clause 3. |
| `README.md` | 300–302 | "Memory only. Jobs and reports are kept in memory" | ADR-0011 clause 1; ADR-0017 clause 5. |
| `README.md` | 310–311 | "no remote cloning, no account and no scheduling" | ADR-0016 clause 1; ADR-0011 clause 2. |
| `README.md` | 456 | `docs/docker.md` link | Target not tracked. |
| `README.md` | 475 | `--json` flag row | ADR-0034 clause 1. |
| `README.md` | 477 | `--per-author` flag row | ADR-0009 clause 1. |
| `README.md` | 660 | "The contributor is named only with `--per-author`" | ADR-0009 clause 1. |
| `README.md` | 687–697 | "Why per-author metrics are opt-in … with `--per-author`" | ADR-0009 clause 1. |
| `README.md` | 777 | `docs/docker.md` documentation row | Target not tracked. |
| `cmd/commitography/main.go` | 38–39 | "per-contributor breakdowns are opt-in behind --per-author" | ADR-0009 clause 1. |
| `cmd/commitography/main.go` | 65 | `"per-author"` flag | ADR-0009 clause 1; ADR-0028 clause 3. |
| `cmd/commitography/main.go` | 69 | `"json"` flag "skip HTML rendering" | ADR-0034 clause 1–2. |
| `docs/project-overview.md` | 52 | "Per-author breakdowns are available behind the `--per-author` flag" | ADR-0009 clause 1. |
| `docs/project-overview.md` | 116 | "No database server." under Storage | Storage is embedded SQLite (ADR-0035); statement omits it. |
| `docs/report-schema.json` | 365 | "Present only when --per-author was passed" | ADR-0009 clause 1. |
| `docs/report-schema.json` | 469 | "Present only when --per-author was passed" | ADR-0009 clause 1. |
| `internal/aggregate/perauthor.go` | 30–31 | "opt-in per-contributor section, present only when --per-author was passed" | ADR-0009 clause 1. |
| `internal/aggregate/social.go` | 68 | "named only when --per-author was requested" | ADR-0009 clause 1. |
| `internal/aggregate/social_test.go` | 211 | "must not be named without --per-author" | ADR-0009 clause 1. |
| `internal/render/render.go` | 121 | "for --json runs" | ADR-0034 clause 1. |
| `web/e2e/audit.mjs` | 110 | `'--per-author'` CLI invocation | ADR-0009 clause 1. |

---

## 3. Tests

Go test files are run by `go test ./...` except the two build-tagged packages
noted. Web component tests run under `vitest`; the browser audit runs with
`npm run e2e`.

| test file | what it asserts | does its subject survive the decision set | governing ADRs |
|---|---|---|---|
| `cmd/commitography/serve_test.go` | `serve` flags default to `127.0.0.1:8080`, no browser, no allowed roots. | yes | ADR-0029 |
| `internal/aggregate/messages_test.go` | Conventional commit detection, heuristic rule order, confidence threshold 0.30, short/revert/typo counters, emoji detection, subject truncation, stopword filtering, average length, empty input. | partly — conventional pattern and 0.30 threshold survive as `degraded` status; rule order, stored subject text and word cloud are not in `docs/metrics.md` | ADR-0024, ADR-0032 |
| `internal/aggregate/privacy_test.go` | Emails hashed by default, hash stable and case-insensitive, anonymize replaces names and drops emails, pseudonyms in arrival order, message content kept, alphabetic labels, nil per-author section. | partly — anonymisation survives; hashing as a configurable default does not | ADR-0033 |
| `internal/aggregate/schema_test.go` | Reports built from fixtures (default, with blame, per-author, anonymized) validate against `docs/report-schema.json`; a malformed report is rejected. | no — schema and report document are replaced | ADR-0031, ADR-0021 |
| `internal/aggregate/social_test.go` | Bus factor by commit counts, coupling support/confidence/expected flag, wide-commit skip, churn window, directory bus factor and knowledge concentration, activity threshold, stem extraction. | partly — coupling rules survive; bus factor and concentration become line-based; churn moves to `hotspot` | ADR-0024, ADR-0020 |
| `internal/aggregate/temporal_test.go` | Author-local histograms, busiest-day tie break, streak and silence, single-day edge case, local-date streaks, zero-filled months, empty input, offsets preserved. | yes | ADR-0024 |
| `internal/analysis/consistency_test.go` | `repositoryChanged` detects HEAD, branch and shallow/graft changes. | undecided — no record addresses end-of-run revalidation or a `stale` outcome | ADR-0020 |
| `internal/analysis/progress_test.go` | Event emitter yields estimated fractions per stage window. | partly — progress survives (ADR-0043); stage names change | ADR-0020, ADR-0043 |
| `internal/analysis/run_test.go` | Cancelled context does nothing; cancellation during collection and at the next checkpoint; concurrent runs keep warnings apart. | partly — cancellation survives; runs use `NoBlame` | ADR-0044, ADR-0020 |
| `internal/cli/report_test.go` | Mount hints for missing repository or allowed root printed only inside containers. | partly — user errors with remedies survive; message format governed by reason codes | ADR-0041 |
| `internal/cli/run_test.go` | Dashboard written; CLI and service reports equal; no plaintext email; anonymize; `--json`; opt-in per-author; shallow refused and allowed; exit code 2; quiet/verbose conflict; Wrapped thin-year refusal and page; bots excluded. | partly — CLI/server equivalence, no raw email, shallow exit 2 and bot exclusion survive; HTML, Wrapped page, `--json` and opt-in per-author do not | ADR-0034, ADR-0021, ADR-0033, ADR-0009 |
| `internal/collect/collect_test.go` | Commit count matches rev-list; added lines match git; author offsets preserved; root has no parents; merges carry no files; single commit; unusual paths; mailmap reconciles with shortlog. | yes | ADR-0020, ADR-0047 |
| `internal/collect/gitlog_test.go` | Parser rejoins subjects containing `\x01`; headerless stream reported; progress; record-start detection; sharded read equals single stream; shard count. | partly — sharding survives (ADR-0052); the `\x01` newline format is replaced by NUL-delimited output | ADR-0047, ADR-0052 |
| `internal/collect/preflight_test.go` | Preflight on basic repository, shallow detection, shallow message, empty repository, non-repository, git version parsing and comparison. | yes | ADR-0034 |
| `internal/collect/writer_test.go` | History artifact round trip, compact encoding, foreign schema version rejected. | partly — stored commit records survive (ADR-0017); uncompressed JSON file does not (ADR-0035 clause 4) | ADR-0017, ADR-0035 |
| `internal/config/config_test.go` | Defaults, list append, empty list clears, scalar overrides, explicit file replaces repository file, invalid date source, invalid threshold and theme, unknown key warns, call-local warning sink, bot identity pattern. | partly — resolution order and unknown-key warning survive; `theme` is not an analysis setting | ADR-0026 |
| `internal/dockersmoke/smoke_test.go` (tag `dockersmoke`) | Image CLI contract; default command writes the dashboard; read-only repositories as any user; server mode equals native server; no writes to mounts; foreign hosts refused; stop signal via tini; missing-mount hints. | partly — one image, read-only mounts and signal handling survive; dashboard output and server shape do not | ADR-0005, ADR-0034, ADR-0046 |
| `internal/filter/filter_test.go` | Lockfile contributes no lines; linguist-generated excluded; negation re-includes; default patterns at any depth and in nested monorepos; empty list disables; merges excluded by default and kept on request; bulk commit flagged; bots excluded; input not mutated. | yes | ADR-0026, ADR-0019 |
| `internal/gitcmd/gitcmd_test.go` | A cancelled context terminates the git process. | yes | ADR-0044 |
| `internal/identity/identity_test.go` | Resolver reconciles with shortlog on mailmap fixture; configured identity merges emails; bots flagged; display name from latest commit; ordering by commit count; unknown addresses stable. | partly — resolution survives; email-keyed IDs do not | ADR-0033, ADR-0051 |
| `internal/jobs/jobs_test.go` | One active job and bounded terminal history; eviction; worker run and cancellation; monotonic progress; only succeeded jobs expose reports; cancel all; warnings merged and bounded; cancellation beats late result; terminal states immutable; ten newest kept; stale failure hides errors. | partly — cancellation survives; one global slot and in-memory history do not | ADR-0027, ADR-0044 |
| `internal/perfcheck/perf_test.go` (tag `perfcheck`) | Analysis duration with and without blame; server responsiveness during analysis; cancellation latency; retained memory bounded by job limit; native startup and CLI; Docker startup and mount overhead. | partly — responsiveness and memory measurement survive (ADR-0050 clause 3); absolute duration bounds and blame variants do not | ADR-0050, ADR-0054, ADR-0020 |
| `internal/render/bundle_test.go` | Embedded bundle contains no unreviewed external URL and no remote-loading construct. | yes | ADR-0022, ADR-0036 |
| `internal/render/render_test.go` | Render writes only `index.html` and `report.json`; page self-contained; `</script>` cannot break out; page portable; unrelated files untouched; overwrite; Wrapped page and delta; `report.json` valid and indented. | partly — `report.json` writing survives; every HTML assertion does not | ADR-0034 |
| `internal/server/api_test.go` | Capabilities and job list; versioned lifecycle routes; job creation rejects unknown fields; status, report and delete; options passed to analysis; progress fields; cancel transitions and rejection; session bootstrap and origin protection. | partly — versioned paths, session and origin checks survive; polling status, `DELETE` and option set do not | ADR-0043, ADR-0020 |
| `internal/server/handler_test.go` | Handler serves the shell and embedded assets; reserves `/api/`; assets cannot be listed. | yes | ADR-0036 |
| `internal/server/host_test.go` | Only loopback host names accepted; rebinding page cannot start jobs; explicit listen address allowed. | partly — rebinding protection is not addressed by a record; loopback-only names cannot serve public mode | ADR-0029, ADR-0045 |
| `internal/server/listener_test.go` | Serve binds and prints URL; browser failure is a warning; announcement of listener reachability; shutdown hook called. | partly — non-loopback warning survives; passphrase requirement absent | ADR-0029 |
| `internal/server/matrix_test.go` | Endpoint method/status matrix; non-repository folder rejected; history evicts oldest finished job. | partly | ADR-0043, ADR-0027 |
| `internal/server/path_test.go` | Prefix sibling rejected; outside root rejected; inside root accepted; missing and empty roots reported; symlink escape rejected. | yes | ADR-0046 |
| `internal/server/path_windows_test.go` | Junction escape rejected; case and separator variants contained; parent traversal rejected; extended and UNC paths contained. | yes | ADR-0046 |
| `internal/server/security_test.go` | Security headers present; path errors do not echo filesystem paths; method and unknown-job matrix. | partly — headers and no-path errors survive (ADR-0033, ADR-0041); method matrix changes | ADR-0033, ADR-0041, ADR-0043 |
| `internal/server/securitymatrix_test.go` | Route traversal reads no files; state-changing routes need session and same origin; headers on every response class; session cookie process-scoped and strict; responses carry only documented data (schema file); default listener loopback. | partly — session, origin, traversal and loopback checks survive; schema file and `DELETE` do not | ADR-0043, ADR-0029, ADR-0031 |
| `web/src/app/JobView.test.tsx` | Estimated percentage and stage labels; indeterminate stage; polling stops on success and shows report; cancel without reload; rejected versus failed; stale job loads no report; retry reads status only; unknown job explained. | partly — progress, cancellation and terminal-state wording survive; polling does not | ADR-0043 |
| `web/src/app/RepositoryForm.test.tsx` | Empty path refused; inverted date range refused; contributor-naming warning; single request per start; blocked while active; path error guidance; allow-shallow offer; started job reported. | partly — validation survives; per-author warning and single active job do not | ADR-0009, ADR-0027 |
| `web/src/app/logic.test.ts` | Start-error guidance by code; outcome wording per terminal state; progress formatting; route round trip and unsafe hash fallback. | partly — codes, stages and routes change | ADR-0041, ADR-0028 |
| `web/src/main.test.tsx` | Static bootstrap renders dashboard and Wrapped from embedded data; missing data explained; application starts without mode; static and fetched reports render the same text. | no — static pages are removed | ADR-0034 |
| `web/src/report/Dashboard.test.tsx` | Sections in reading order; sections without data omitted; every chart named with a data table; landmarks on static page; nested heading when embedded; warnings shown and per-author opt-in. | partly — accessible chart tables survive (ADR-0032 clause 7); omitting empty sections contradicts ADR-0032 clause 6 | ADR-0032, ADR-0034, ADR-0009 |
| `web/e2e/audit.mjs` (with `a11y.mjs`, `cdp.mjs`, `repositories.mjs`) | Headless Chrome drives job flows, recent jobs, keyboard, responsive and contrast checks, and opens the static dashboard and Wrapped pages offline asserting no network request. | partly — application audit survives; static page steps do not | ADR-0034, ADR-0038 |

---

## 4. Dependencies

No dependency allow list exists in the repository (ADR-0049 clause 2 and 3;
WP-0003 clause 6 creates it from this audit). "undecided" in the permitted
column below means: no clause of ADR-0022 forbids the dependency, and allow-list
membership has not been recorded.

### Go

| dependency | direct or indirect | what uses it | still permitted under ADR-0022 and ADR-0049 | note |
|---|---|---|---|---|
| `github.com/bmatcuk/doublestar/v4` v4.10.0 | direct | `internal/filter/paths.go` (exclusion patterns) | undecided | Pure Go; no external service. |
| `github.com/spf13/cobra` v1.10.2 | direct | `cmd/commitography/main.go`, `serve.go` | undecided | Pure Go; no external service. |
| `gopkg.in/yaml.v3` v3.0.1 | direct | `internal/config/config.go` | undecided | Pure Go; no external service. |
| `github.com/inconshreveable/mousetrap` v1.1.0 | indirect | cobra (Windows start detection) | undecided | Pure Go. |
| `github.com/spf13/pflag` v1.0.9 | indirect | cobra (flag parsing) | undecided | Pure Go. |
| pure Go SQLite driver | absent | — | — | ADR-0035 clause 1–2 requires one; none is present. |

`go.sum` additionally carries `go.mod`-only hashes for
`github.com/cpuguy83/go-md2man/v2`, `github.com/russross/blackfriday/v2`,
`gopkg.in/check.v1` and `go.yaml.in/yaml/v3`, which are not in the build list.

### Frontend

| dependency | direct or indirect | what uses it | still permitted under ADR-0022 and ADR-0049 | note |
|---|---|---|---|---|
| `react` ^19.3.0 | direct (runtime) | all of `web/src` | undecided | Named as the renderer by ADR-0037 clause 3. |
| `react-dom` ^19.3.0 | direct (runtime) | `web/src/main.tsx` | undecided | Same. |
| `@mui/material` ^9.4.0 | direct (runtime) | `web/src/app/*.tsx`, `outcomes.ts`, `ui/theme.ts` | no | ADR-0038 clause 1 requires removal of the opinionated component library. |
| `@emotion/react` ^11.14.0 | direct (runtime) | peer of `@mui/material`; no direct import in `web/src` | no | Present only for MUI (ADR-0038 clause 1). |
| `@emotion/styled` ^11.14.1 | direct (runtime) | peer of `@mui/material`; no direct import in `web/src` | no | Present only for MUI (ADR-0038 clause 1). |
| `typescript` ~5.6.3 | direct (dev) | `npm run build`, `typecheck` | undecided | Build-time only (ADR-0036 clause 3). |
| `vite` ~5.4.11 | direct (dev) | `npm run build`, `dev` | undecided | Build-time only; build must be reproducible (ADR-0049 clause 1). |
| `vitest` ^2.1.9 | direct (dev) | `npm run test` | undecided | Test-time only. |
| `jsdom` ^25.0.1 | direct (dev) | vitest environment | undecided | Test-time only. |
| `@testing-library/dom` ^10.4.1 | direct (dev) | component tests | undecided | Test-time only. |
| `@testing-library/react` ^16.3.3 | direct (dev) | component tests | undecided | Test-time only. |
| `@types/react` ^19.3.0 | direct (dev) | type checking | undecided | Type definitions only. |
| `@types/react-dom` ^19.3.0 | direct (dev) | type checking | undecided | Type definitions only. |
| 79 runtime transitive packages | indirect | pulled by `@mui/material` and `@emotion/*` (for example `@mui/system`, `@mui/utils`, `@popperjs/core`, `@emotion/cache`, `@emotion/babel-plugin`, `@babel/*`, `react-transition-group`, `stylis`) and by `react`/`react-dom` (`scheduler`) | no for the MUI and emotion trees; undecided for `scheduler` | Counted from `web/package-lock.json` (lockfileVersion 3): 239 entries, 84 without the `dev` flag of which 5 are direct. |
| 147 development transitive packages | indirect | pulled by vite, vitest, jsdom, typescript and testing-library | undecided | 155 entries carry the `dev` flag, of which 8 are direct. |

No headless primitive library (ADR-0038 clause 4), layout-mathematics library
(ADR-0037 clause 2) or utility-class tooling (ADR-0038 clause 2) is present.

---

## 5. Current report shape

Determined by reading `internal/aggregate/report.go` and confirmed by building
the CLI into a scratch directory and running it with `--json` against a
throwaway 12-commit repository outside the checkout. `perAuthor` appeared only
when `--per-author` was added. `docs/report-schema.json` declares the same keys,
with `perAuthor` optional and `additionalProperties: false`.

| key | description |
|---|---|
| `schemaVersion` | Integer report format version; currently `1`. |
| `generatedAt` | RFC 3339 UTC timestamp read from the process clock at build time. |
| `toolVersion` | Build version string injected at link time (`dev` when not injected). |
| `repository` | Summary: name, default branch, head commit, first/last commit timestamps, age in days, total/analysed/excluded commit counts, contributor count, tracked files and lines, shallow flag. |
| `temporal` | Hour, weekday and hour×weekday histograms, Friday-evening count, night ratio, busiest day, longest streak and silence, commits per month, first/last commit timestamps. |
| `code` | Most-touched files, largest commit, mean/median commit size, total added/deleted, oldest untouched file, file-type distribution, blame-sampled code age by year, first-year survival ratio, sample and total file counts, tracked files and lines. |
| `messages` | Type distribution, conventional ratio, low-confidence flag, short-message count, longest subject (with text), mean subject length, emoji commits and top emoji, revert and typo-fix counts, top words. |
| `social` | Repository bus factor, per-directory bus factor, coupled file pairs, churn hotspots, knowledge concentration per top-level directory. |
| `notables` | Bulk commits, latest-night and earliest-morning commits, weekend ratio, holiday commits, first commit subject, merge count, timezone spread. |
| `perAuthor` | Optional; present only with `--per-author`. List of contributor summaries: identity ID, display name, emails (hashed, raw, or omitted depending on configuration), commits, lines, files, hour histogram, first/last commit, active days. |
| `warnings` | Array of free-text diagnostic strings, including the shallow-clone warning when an override was given. |

The report contains no family status, no family versions, no resolved analysis
configuration, no designated generation-metadata section and no identity digest
outside `perAuthor`.

---

## 6. Observations

### Code that appears unfinished or unreachable

- `internal/analysis/analysis.go:63-65` defines `RunFunc`; nothing references
  it. Its comment says the implementation "is added by the orchestration work
  package", while `analysis.Run` exists in `run.go`.
- `collect.WriteHistory` and `collect.ReadHistory`
  (`internal/collect/writer.go`) are called only from `writer_test.go`. No
  production path writes or reads the history artifact, so
  `model.History.SchemaVersion` is only checked in tests.
- `internal/collect/git.go`: `gitCommand` and `runGit` have no callers.
- `internal/aggregate/code.go:308`: `trackedFiles` (the non-context variant)
  has no callers.
- `internal/cli/progress.go:87`: `Progress.Debug` has no callers, so
  `--verbose` changes nothing except the conflict check with `--quiet`.
- `internal/cli/run.go:24`: `ExitStrictWarn = 3` is never returned. README
  documents it as reserved.
- `internal/cli/run.go:29`: `minWrappedCommits` is declared but unused. The
  enforced value lives in `internal/analysis/run.go:16`.
- `config.Load` and the package variable `config.Warn` have no caller outside
  package `config`; `analysis.Run` uses `LoadWithWarn`.
- `server.NewHandler` is called only from `handler_test.go`. Its comment says API
  paths "return 404 until their handlers are registered by the later server
  milestones", but `App.Handler` registers `/api/v1/`.
- `analysis.Options.CheckConsistency` is set only by the server
  (`internal/server/api.go:166`). The CLI never revalidates the repository after
  analysis.
- `config.Config.Theme` accepts only `"default"`, and nothing reads it after
  validation.
- `internal/pipeline/` contains only the taxonomy data files. No Go package
  exists under it, and no Go code references `taxonomy.yml`.
- No CI workflow files, linter configuration, dependency allow list, golden
  files or budget file are tracked.

### Hard to move under ADR-0040

- `internal/render` combines HTML page generation (`pipeline/render`
  responsibility) with the embedded frontend bundle that `internal/server`
  serves through `render.AssetFS()`. The bundle location
  `internal/render/assets` is also written into `web/vite.config.ts:12`
  (`outDir`), `.gitattributes:14`, the `.gitignore` comment and
  `internal/render/bundle_test.go`. WP-0005's allow list permits `internal/**`,
  `cmd/**` and `Makefile`, and forbids `web/**`. It does not list
  `.gitattributes` or `.gitignore`.
- `internal/aggregate/code.go` runs `git ls-tree` and `git blame` through
  `gitcmd` and reads working-tree files (`looksBinary`, `countLines`). It
  therefore spans collect, replay-equivalent and aggregate responsibilities in
  one function (`buildCode`).
- `internal/aggregate` holds the report type (a `core` responsibility), the
  build orchestration (`pipeline/aggregate`), privacy rewriting and all metric
  computations (`metrics/*`) in one package. `internal/jobs`,
  `internal/render`, `internal/analysis` and web types depend on
  `aggregate.Report`.
- Packages with no named destination in ADR-0040 clause 1:
  `internal/analysis` (orchestration used by CLI and jobs), `internal/cli`,
  `internal/container`, `internal/version`, `internal/dockersmoke`,
  `internal/perfcheck`. `internal/filter` holds shared definitions (analysed
  commit, bulk commit) and also reads `.gitattributes` from the working tree.
- `internal/cli` imports `internal/server` (for `MissingRootError`) and
  `internal/render`. `internal/server/path.go` imports `internal/collect`
  (`Preflight`) and `internal/gitcmd`.
- `internal/aggregate/social.go` reads `filter.CouplingMaxFilesPerCommit`. Once
  coupling is a family, this constant is shared derived data from another
  package (ADR-0040 clause 4).
- `os/exec` is imported in `internal/gitcmd`, `internal/collect/git.go`,
  `internal/collect/preflight.go` (`exec.LookPath`) and
  `internal/server/listener.go`, where it opens a browser, not git. ADR-0047
  clause 1 names only one package that may import process execution.
- `internal/version` exposes package-level variables because `-ldflags -X`
  (Makefile, `.goreleaser.yml`) can set only package-level string variables.
  ADR-0042 clause 2 forbids package-level variables, and ADR-0049 clause 6
  requires explicit version injection.
- Bare `go` statements: `internal/collect/gitlog.go:274`,
  `internal/jobs/jobs.go:177`, `internal/server/listener.go:79`.
- Direct process clock reads: `internal/aggregate/report.go:145`,
  `internal/collect/gitlog.go:121`, `internal/cli/progress.go:35`,
  `internal/jobs/jobs.go:113` (default).
- Package-level variables: `aggregate/messages.go` (`conventionalRe`,
  `heuristicRules`, `lowEffortSubjects`), `aggregate/stopwords.go` (`stopwords`,
  filled in `init`), `collect/preflight.go` (`ErrGitNotFound`, `gitVersionRe`),
  `config/config.go` (`Warn`, `DefaultExcludeAuthors`, `DefaultExcludePaths`,
  `botNameRe`, `knownKeys`), `container/container.go` (`markers`),
  `filter/paths.go` (`caseInsensitiveFS`), `render/render.go` (`pageTemplate`,
  `scriptSafeEscapes`), `server/resolve_windows.go`
  (`procGetFinalPathNameByHandleW`), `version/version.go` (three).
- `panic` calls: `internal/server/handler.go:53` (`NewApp` on a
  `canonicalRoots` error, which includes `os.Getwd` failure),
  `internal/server/handler.go:188` (session token randomness), and
  `internal/collect/preflight.go:161` (invalid version constant). No HTTP
  recovery layer exists.

### Places where parts of the codebase already disagree

- The report shape is defined four times, by hand: Go structs in
  `internal/aggregate`, `docs/report-schema.json`, `web/src/types.ts` and
  `web/src/test/sampleReport.ts`. `internal/server/api.go:97` hard-codes
  `ReportSchemaVersion: 1` instead of reading `aggregate.SchemaVersion`.
  `model.SchemaVersion` (history artifact) is a separate constant, also `1`.
- `Dockerfile:30` default command is `/repo -o /repo/out`, which writes inside
  the analysed repository mount. README's Docker instructions mount the
  repository `readonly` and pass `-o /out`.
- `Dockerfile:15` sets `safe.directory '*'` in git's system configuration.
  ADR-0047 clause 3 requires system configuration disabled on every invocation,
  which would ignore that setting.
- `internal/aggregate/messages.go` keyword rule order is revert, merge, fix,
  feat, refactor, test, docs, chore, style, ci. `docs/metrics.md` section 4 lists
  revert, merge, fix, feat, docs, test, refactor, style, build, ci, chore, other.
  The code keeps the patterns in code; `docs/metrics.md` places them in a rules
  file.
- Limits in code versus `docs/metrics.md` section 12: coupling pairs 50 versus
  200, file types 15 versus extensions 20, churn 25 versus 100. Rounding in
  `aggregate/stats.go` is 4 decimal places; `docs/metrics.md` section 1 states
  6 significant digits.
- Bus factor and knowledge concentration are computed from commit counts in
  `aggregate/social.go`; `docs/metrics.md` section 7 defines both over surviving
  lines.
- `docs/metrics.md` section 4 says longest message text is not stored. The
  report stores subject text in `messages.longestSubject.subject`,
  `notables.firstCommitSubject`, `notables.bulkCommits[].subject` and
  `code.largestCommit.subject`.
- `internal/server/api.go` uses `DELETE /api/v1/jobs/{id}` and `authorize`
  treats `POST` and `DELETE` as state-changing. ADR-0043 clause 4 requires
  state-changing requests to use `POST`.
- `internal/server/api.go:280` returns `repoPath` (the canonical absolute path)
  in every job status response. `internal/collect/preflight.go:53`
  (`NotRepositoryError`) and `config.LoadWithWarn` errors include file paths.
  ADR-0033 clause 3 and ADR-0041 clause 5 exclude absolute paths from API
  responses and error messages.
- `.commitography.yml` and `config.Default()` set `hash_emails: true`. With it
  set to `false` and `anonymize: false`, `perAuthor.authors[].emails` and
  `identityId` carry raw addresses.
- `web/vite.config.ts` disables code splitting so that Go can inline one file.
  ADR-0053 clause 5 requires Wrapped to be code-split.
- `web/src/app/format.ts` maps stage `code` to "Code and ownership".
  `internal/analysis/run.go:141` maps blame progress onto `StageCode`.
- `Makefile:12` injects `BUILD_DATE` from `date`, and `.goreleaser.yml:34`
  injects `{{ .Date }}`. Two builds of one commit therefore differ (ADR-0049
  acceptance criterion 3).
- `go.mod` declares `go 1.22.0`. `internal/server/resolve_windows.go:15-17`
  explains its implementation by Go 1.23 `winsymlink` behaviour. The scratch
  build for section 5 used `go1.27.0`.

### Between the work packages and the tree

- WP-0002 clause 4 moves `docs/report-schema.json`. That path is read by
  `internal/aggregate/schema_test.go:28` and
  `internal/server/securitymatrix_test.go:268`, and archived by
  `.goreleaser.yml:54`. WP-0002's definition of done requires the existing test
  suite to pass unchanged. Its allow list includes source files listed in
  section 2 of this audit, and all three appear there.
- WP-0002 clause 7 and its `Files` list name `docs/docker.md`, which is not
  tracked. WP-0002 clause 3 deletes phase documents under `docs/`; no such file
  is tracked. `README.md`, `CHANGELOG.md`, `docs/project-overview.md` and
  `web/src/api/client.ts` link to `docs/docker.md`, `docs/phase-1.5.md`,
  `docs/phase-2.md`, `docs/phase-3.md`, `docs/phase-1.5/m6-verification.md`,
  `docs/phase-1.5/m0-contracts.md`, `phase-1-detailed.md` and
  `phase-1.5-detailed.md`. None is tracked.
- `testdata/build-fixtures.sh:14` runs `rm -rf "$root"` on
  `testdata/fixtures`, deleting the tracked `testdata/fixtures/.gitkeep` on
  every run of `make fixtures`. This audit therefore did not run it.
- Existing fixtures are `basic`, `mailmap`, `merges`, `binary`, `noise`, `bots`,
  `coupling`, `shallow`, `empty`, `single`. WP-0004 clause 3 names a different
  set. The `binary` fixture commits `quoted".txt` only where the filesystem
  accepts the name (`build-fixtures.sh:206`), so its commit hashes differ
  between NTFS and POSIX filesystems (ADR-0019 clause 1).
- WP-0005 must not touch `web/**`, yet the bundle output directory is set in
  `web/vite.config.ts`. See the first item under "Hard to move".
- The shallow-clone override (`--allow-shallow`) adds a free-text warning to
  `warnings`. ADR-0034 clause 6 requires the degraded condition to be recorded
  under ADR-0032, which the report shape has no field for.
- The Wrapped year (`Input.Year`) filters the commits a report is built from,
  but the report records neither the year nor any analysis configuration
  (ADR-0021 clause 1, ADR-0026 clause 2).
