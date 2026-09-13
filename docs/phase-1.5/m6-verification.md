# M6 - Verification and Release Documentation

**Depends on:** M1, M2, M3, M4 and M5.

M6 is the release gate. It records evidence rather than relying on source
inspection for behavior that can be exercised.

| Package | Status |
|---|---|
| WP-6.1 Analysis and CLI regression tests | Complete |
| WP-6.2 Job lifecycle and API tests | Complete |
| WP-6.3 Frontend build and component tests | Complete |
| WP-6.4 Browser end-to-end audit | Complete |
| WP-6.5 Cross-platform path verification | Blocked: macOS host unavailable; Windows and Linux complete |
| WP-6.6 Performance and resource verification | Complete |
| WP-6.7 Security verification | Complete |
| WP-6.8 Final documentation and phase closure | Complete |

---

## Clarified scope - 2026-09-13

The existing evidence was inventoried before this milestone started. The work
packages below remain normative; this section records the gaps they close and
how.

### Findings

1. `go test ./...` passes with the fixtures built. Exactly one test skips:
   `TestValidateRepositoryRejectsSymlinkEscape`, because creating a symlink on
   Windows needs a privilege the development account does not hold.
2. `internal/analysis` tests change detection and progress estimates only.
   Cancellation is tested for Git commands and for the job manager with fake
   runners, never for `analysis.Run` itself. CLI and service report parity is
   already tested.
3. API tests cover capabilities, creation, status, report, deletion,
   cancellation and origin checks, but not a second start (`409`), failed or
   stale jobs, eviction through the API, deleting an active job or reading the
   report of an unfinished job.
4. The M0 contract maps an oversized request body to `413`. The server answers
   `400 invalid_json`.
5. The web application has no automated tests. The browser audits that
   verified M4 exist only as local scripts, and they depend on a private
   repository for slow analyses.
6. Only a Windows host is available. Linux can be exercised in a Linux
   container; macOS cannot be exercised at all.

### Decisions

| ID | Decision |
|---|---|
| D1 | WP-6.1 adds tests for cancelling `analysis.Run` before and during a run, and for warning sinks that stay separate across concurrent runs. |
| D2 | WP-6.2 adds an endpoint matrix with success and failure cases for every documented endpoint, and changes an oversized body to `413 request_too_large` as the contract states. |
| D3 | WP-6.3 adds Vitest 2.1.9 with jsdom 25 and Testing Library as pinned development dependencies, run by `npm run test`, plus a Go test that inspects the embedded bundle for external URLs. |
| D4 | WP-6.4 commits the dependency-free Chrome DevTools Protocol audit under `web/e2e/`. It builds its own repositories, including a large generated one for cancellation and a stale run, so it depends on nothing outside the checkout except Git, Go and Chrome. |
| D5 | WP-6.5 adds Windows junction, drive-case and extended-path tests, runs the path tests in a Linux container for Linux evidence, and records macOS as blocked until a macOS host runs the documented commands. |
| D6 | WP-6.6 measures through a `perfcheck`-tagged Go test that uses a generated large repository, so the numbers can be reproduced; a real repository is measured additionally when one is supplied. |
| D7 | WP-6.7 adds a security test matrix and records a manual review, including every filesystem read a job performs. |
| D8 | WP-6.8 turns the phase exit criteria into a table with evidence or an explicit blocker, and does not mark the phase released. |

---

## WP-6.1 - Analysis and CLI regression tests

### Deliverables

Extend the existing Go test suite for context cancellation, service parity,
warning isolation, report equivalence and unchanged CLI behavior.

### Acceptance criteria

- `go test ./...` passes.
- `go vet ./...` passes.
- `gofmt -l cmd internal` is empty.
- Existing fixture counts, privacy assertions, schema tests and render security
  tests remain active.

### Changes

`internal/analysis/run_test.go` adds four tests: a cancelled context stops
`analysis.Run` before any work or progress event; cancelling while `git log`
streams history reports `context.Canceled` rather than the killed process's
failure; cancelling after identity resolution stops at the next checkpoint
before any metric stage; and two concurrent runs each receive only their own
configuration warnings. CLI and service report parity was already covered by
`TestCLIAndAnalysisServiceProduceTheSameReport`.

### Verification

Recorded 2026-09-13 on Windows 11 Pro 10.0.26200 with Go 1.27.0 and Git
2.55.0.windows.3, with the fixtures built.

- `gofmt -l cmd internal` printed nothing and `go vet ./...` passed.
- `go test -count=1 -v ./...` passed: 193 tests passed, none failed, one
  skipped.
- The skipped test is `TestValidateRepositoryRejectsSymlinkEscape`, which needs
  the Windows symlink privilege. WP-6.5 covers that case with junctions and a
  Linux run. No fixture, privacy, schema or render test skipped.

## WP-6.2 - Job lifecycle and API tests

### Deliverables

Use `httptest` and deterministic fake analysis runners where necessary to test
start, polling, conflict, cancel, failure, stale, delete and eviction flows.

### Acceptance criteria

- Every documented endpoint has success and failure tests.
- A second start cannot create a second active worker.
- Cancellation cannot be overwritten by success.
- The ten-job limit is deterministic.
- Polling is safe across repeated identical requests.

### Changes

- **Cancellation wins.** A runner that finished after a cancellation was
  requested turned the job into `succeeded`. The job manager now records a
  requested cancellation whatever the runner returns afterwards. With the
  previous manager, the new `TestCancellationWinsOverALateResult` failed with
  `status = succeeded, want cancelled`.
- **Oversized bodies answer `413`.** A job request over 1 MiB now answers
  `413 request_too_large`, as the M0 contract maps it, instead of
  `400 invalid_json`.
- `internal/server/matrix_test.go` walks every endpoint through its lifecycle
  with a runner the test releases, and adds a plain folder and eviction through
  the API. `internal/jobs` adds the late-result, overwrite and ten-newest
  tests.

### Verification

Recorded 2026-09-13 on the host recorded under WP-6.1. `go test -count=1 -v
./...` passed with 202 tests passing, none failing and the same single skip.

| Criterion | Evidence |
|---|---|
| Every documented endpoint has success and failure tests | `TestAPIEndpointMatrix`: capabilities `200` and `405`; list `200`, `401`, `405`; create `202`, `400` for malformed JSON, two objects, `outputDir`, `configPath` and an empty path, `401`, `403` for origin and outside root, `409`, `413`; status `200`, `401`, `404`, `405`; report `200`, `401`, `404`, `405` and `409` while running, cancelled, failed or stale; cancel `202`, `200` when repeated, `401`, `403`, `404`, `405`, `409` when succeeded; delete `204`, `401`, `403`, `404`, `409` while running. `TestAPIRejectsAFolderThatIsNotARepository` adds `invalid_repository` and a missing path. |
| A second start cannot create a second active worker | The matrix answers `409 active_job` to a second start and counts exactly one started analysis. |
| Cancellation cannot be overwritten by success | `TestCancellationWinsOverALateResult` and `TestTerminalStatesCannotBeOverwritten`, which refuses complete, fail, cancel and restart on each terminal state. |
| The ten-job limit is deterministic | `TestHistoryKeepsExactlyTheTenNewestJobs` keeps `job-12` to `job-03` of twelve jobs, newest first; `TestAPIHistoryEvictsTheOldestFinishedJob` shows the same through the API. |
| Polling is safe across repeated identical requests | Five repeated polls of a running job were equal apart from elapsed time, and five polls of a finished job were byte-identical. |

## WP-6.3 - Frontend build and component tests

### Deliverables

Add type, build and component-level tests for form validation, progress states,
terminal states, report rendering and static bootstrap.

### Acceptance criteria

- `npm.cmd run typecheck` passes.
- `npm.cmd run build` passes.
- The bundle contains no external `http` or `https` asset references.
- Static embedded data and server-fetched data render the same report sections.

### Changes

- Vitest 2.1.9, jsdom 25.0.1, `@testing-library/react` 16.3.3 and
  `@testing-library/dom` 10.4.1 are development dependencies locked in
  `web/package-lock.json`. `npm run test` runs them with `web/vitest.config.ts`;
  the library build configuration is unchanged.
- 38 tests in five files:
  - `logic.test.ts`: start-error guidance, outcome wording for each terminal
    state, formatting, and routes, including hashes that must fall back to the
    start view.
  - `RepositoryForm.test.tsx`: an empty path, an inverted date range, the
    per-author privacy warning, one request however often Start is pressed, an
    active job blocking a new start, and guidance for a rejected path and a
    shallow clone.
  - `JobView.test.tsx`: an estimated percentage that says so, an indeterminate
    stage, polling that stops at success and then shows the report,
    cancellation without a reload, a rejected request told apart from a failed
    analysis, a stale job that never loads a report, a retry that only reads
    status, and a job the server no longer has.
  - `Dashboard.test.tsx`: section order, sections omitted without data, a named
    chart and data table for every figure, static and embedded heading
    structure, warnings, and per-author data staying opt-in.
  - `main.test.tsx`: static dashboard and Wrapped bootstrap, a page without
    data, the local application for a page without a mode, and static against
    server-fetched rendering.
- `internal/render/bundle_test.go` accepts only reviewed absolute URLs in the
  embedded JavaScript and CSS (XML namespaces and the React and MUI error
  documentation) and refuses every form that loads a remote resource.

### Verification

Recorded 2026-09-13 on the host recorded under WP-6.1, with Node.js 24.18.1.

| Criterion | Evidence |
|---|---|
| `npm run typecheck` passes | Exit code `0`, with the test files included. |
| `npm run build` passes | The build succeeded and produced a bundle identical to the committed one. |
| No external asset references | `TestEmbeddedBundleReferencesNoExternalResources` passes. With `fetch("https://cdn.example.com/tracker.js")` appended to the bundle it failed twice, once for the unreviewed URL and once for the remote fetch. |
| Static and server data render the same sections | `main.test.tsx` renders one report from embedded JSON and from the report endpoint; all ten sections match in order and text. |

`npm run test` passed all 38 tests.

## WP-6.4 - Browser end-to-end audit

### Deliverables

Run the local server against fixture repositories in a real browser or a
documented headless browser environment.

### Acceptance criteria

- Start, progress, cancellation, success, failure and stale flows work.
- The report is visible after completion without a full page reload.
- Keyboard and responsive behavior meet M4 criteria.
- Browser console has no uncaught errors.
- No network request leaves the local server origin.

### Changes

`web/e2e/` holds the audit, run by `npm run e2e`. It needs Node.js 22 or newer,
Go, Git and Chrome, Chromium or Edge (or `CHROME_PATH`), and nothing else:

- `cdp.mjs` drives headless Chrome through the DevTools Protocol over Node's
  built-in WebSocket.
- `repositories.mjs` builds the repositories it analyzes: a small history, a
  15,000-commit history written with `git fast-import`, an invalid
  configuration and a repository missing an object.
- `a11y.mjs` checks horizontal overflow, Tab reachability and order, visible
  focus and WCAG AA text contrast.
- `audit.mjs` builds the binary, runs the server and the static CLI output, and
  walks every flow.

The M4 audits depended on a private repository for slow analyses; this one
does not.

### Verification

Recorded 2026-09-13 on the host recorded under WP-6.1, with Node.js 24.18.1 and
Chrome 153.0.8010.37.

The first run passed 76 checks and failed 4, all in the harness: the 15,000-commit
history was read before the stale step could change the repository, and the
lost-server step waited for a Retry button while the page correctly reported
an expired session. After changing the repository during a later stage and
waiting for the expired session instead, the second run passed all 80 checks.

| Criterion | Evidence |
|---|---|
| Start, progress, cancellation, success, failure and stale flows work | A job started from the form succeeded; a 15,000-commit analysis showed activity and an estimated percentage and was cancelled; an invalid configuration was reported as a rejected request and a missing object as a failed analysis; a repository changed during the analysis ended stale without a report. The recent jobs list described all five outcomes differently. |
| The report is visible without a full page reload | A marker set on `window` survived until the report rendered, and until cancellation completed. |
| Keyboard and responsive behavior meet M4 | The start view, a job report, the recent jobs list, the static dashboard and the static Wrapped page had no horizontal overflow at 360, 768 and 1440 pixels; every control was reachable with Tab in document order with visible focus at 360 and 1440 pixels; text met WCAG AA contrast in both themes; every chart kept a named image and data table; changing views moved focus to the new heading. |
| No uncaught errors | No exception and no console error were recorded. |
| No request leaves the origin | Every request went to the local server; the static pages, opened with the network disabled, made none. After a server restart, Retry sent no `POST` and the page reported an expired session. |

## WP-6.5 - Cross-platform path verification

### Deliverables

Verify path and listener behavior on Windows, macOS and Linux. Manual evidence
must include OS version, Go version, Git version and date.

### Acceptance criteria

- Windows drive, UNC, case and junction cases are covered.
- macOS and Linux symlink escape cases are covered.
- Native loopback binding and `--open` behavior are recorded.
- Docker path behavior is recorded separately from native behavior.

### Changes

- **Junctions are resolved by Windows.** A junction needs no privilege to
  create. Go resolves junctions in `filepath.EvalSymlinks` only under the pre-1.23
  `winsymlink` default, which this module gets from `go 1.22.0` in `go.mod`.
  Run with `GODEBUG=winsymlink=1`, the Go 1.23 default, the new junction test
  saw a junction to a repository outside the allowed root pass the root check;
  only the target's lack of commits stopped it. The path check now asks Windows
  for the final path with `GetFinalPathNameByHandleW`, so it no longer depends
  on the Go version. Other systems keep `filepath.EvalSymlinks`.
- `internal/server/path_windows_test.go` adds a junction out of the root, case
  and separator variants inside it, parent traversal, and extended-length and
  UNC spellings.

### Verification

Recorded 2026-09-13.

**Windows** — the host recorded under WP-6.1.

| Case | Result |
|---|---|
| Junction to a repository outside the root | Refused as forbidden under the module default and under `GODEBUG=winsymlink=1`. Before the change, the second run failed. |
| Drive letter case, upper and lower case, forward slashes, trailing separator, `\internal\..` | Accepted inside the root. |
| `..` out of the root with backslashes, forward slashes and different case | Refused as forbidden. |
| `\\?\` spelling of an outside repository | Refused. |
| UNC spelling `\\localhost\C$\…` of an outside repository | Refused. |
| UNC spelling of a path inside the root | Refused as outside the roots: the root is compared in the spelling it was given. This is a limitation, not an escape. |
| Symbolic link out of the root | Not run: creating a symlink needs a privilege the account lacks. Covered on Linux below. |

**Linux** — `golang:1.22` container on Docker Desktop 29.7.2: Go 1.22.12,
Git 2.39.5, kernel 6.18.33.2-microsoft-standard-WSL2 x86_64, with the checkout
mounted read-only. Every `internal/server` test passed, including
`TestValidateRepositoryRejectsSymlinkEscape`, the path, host, origin, session,
endpoint matrix and listener tests.

**macOS — blocked.** No macOS host is available. The case is covered by
`TestValidateRepositoryRejectsSymlinkEscape`, which has not run on macOS. On a
macOS host with Go and Git, run:

```bash
make fixtures
go test -count=1 -v ./internal/server
```

and record the macOS, Go and Git versions here with the result.

**Native listener and `--open`** — on the Windows host,
`commitography serve --open` with default flags printed
`Commitography listening at http://127.0.0.1:8080`, opened that address in the
default browser and printed no browser warning. `netstat` showed a single
listening socket, `127.0.0.1:8080`; connections to the host's other addresses,
192.168.1.114, 172.28.160.1 and 172.26.0.1, were refused.
`TestServeFlagsDefaultToLoopbackWithoutBrowser` checks the flag defaults.

**Docker** — recorded separately under M5: the dashboard works with container
paths such as `/repos/<name>`, a host path is refused with
`invalid_repository_path`, Git Bash needs `MSYS_NO_PATHCONV=1`, and Docker
Desktop mounts an empty folder for a mistyped source. In a container, `--open`
has no browser to launch, so the documented command does not use it.

## WP-6.6 - Performance and resource verification

### Deliverables

Measure native and Docker startup, small fixture analysis, cancellation latency,
memory behavior for ten retained reports and a large repository with and
without `--no-blame`.

### Acceptance criteria

- The server remains responsive while analysis runs.
- Cancellation latency is recorded and has an explicit upper bound for the
  tested repository.
- Memory growth from retained reports is bounded by the ten-job limit.
- Docker filesystem overhead and `--no-blame` guidance are documented.

### Changes

- `internal/perfcheck` is a measurement package behind the `perfcheck` build
  tag, run with `make perfcheck`. It generates two linear repositories with
  `git fast-import` in a temporary directory: 15,000 and 3,000 commits, each
  commit rewriting one of 400 files. Blame therefore walks real history.
  `COMMITOGRAPHY_PERF_REPO` adds a real repository to the analysis and Docker
  timings.
- The tests record numbers and fail only past generous bounds:
  - `GET` p95 above 250 ms during analysis.
  - Cancellation slower than 5 s.
  - Heap growth above 8 MiB once the ten-job limit is reached.

### Verification

Recorded 2026-09-13 with `make perfcheck` on the Windows host recorded under
WP-6.1: Intel Core i5-13500H, 12 cores and 16 threads, 31.7 GiB RAM, and Docker
Desktop 29.7.2 with the WSL2 Linux engine, kernel 6.18.33.2. Every test passed
in 538.6 s.

**Analysis duration** — `analysis.Run`, in process.

| Repository | With blame | `--no-blame` |
|---|---:|---:|
| `testdata/fixtures/basic` | 325 ms | 240 ms |
| Generated, 3,000 commits | 28.7 s | 335 ms |
| Generated, 15,000 commits | 1 m 39.6 s | 503 ms |

Blame is almost the whole cost. Collecting and aggregating 15,000 commits takes
about half a second. Blame runs on a deterministic sample of at most 300 text
files, so its cost grows with the history behind each sampled file rather than
with the number of files. Here each file has about 37 revisions in the
15,000-commit repository and 7.5 in the 3,000-commit one. Every tracked text
file is still read once to detect binaries and count lines, with or without
blame.

**Responsiveness** — during a with-blame analysis of the 15,000-commit
repository, 300 status requests and 300 page requests alternated 20 ms apart.

| Request | p50 | p95 | Max |
|---|---:|---:|---:|
| `GET /api/v1/jobs/{id}` | 0 s | 561 µs | 14.8 ms |
| `GET /` | 0 s | 683 µs | 843 µs |

A p50 of 0 s means below the clock resolution Go measures on this Windows host;
it is not an exact zero. The p95 bound is 250 ms.

**Cancellation latency** — the time from the cancel request to the `cancelled`
status, on the 15,000-commit repository with blame.

| Stage when cancelled | Latency |
|---|---:|
| `collecting` (`git log` streaming) | 11 ms |
| `identity` | 12 ms |
| `code` (line counting and blame) | 12 ms |
| `messages` | 53 ms |

For this repository the upper bound is 5 s; the worst measured value was 53 ms.
A running Git child process is killed through its context, so latency does not
grow with repository size. Pure Go stages stop at their next checkpoint.

**Retained reports** — 20 consecutive `--no-blame` jobs on the 3,000-commit
repository, with live heap measured after a forced GC.

| After jobs | Retained | Heap |
|---:|---:|---:|
| 5 | 5 | 1.9 MiB |
| 10 | 10 | 2.9 MiB |
| 15 | 10 | 2.9 MiB |
| 20 | 10 | 2.9 MiB |

Each retained report took about 189 KiB. Heap growth from job 10 to job 20 was
0 MiB, so memory stops growing at the job limit. A report's size depends on the
repository's file and author counts, not its commit count, so larger
repositories raise the plateau but do not remove it.

**Startup**

| Measurement | Result |
|---|---:|
| Native `serve`, process start to first `200` | median 28 ms, max 272 ms (5 runs; the max is the cold first start) |
| `docker run --rm` of an empty command | median 336 ms (3 runs) |
| `docker run -d … serve` to first `200` through the published port | median 236 ms, max 249 ms (3 runs) |

The `serve` start is faster than the empty `docker run` because that
measurement also includes removing the container.

**Docker filesystem overhead** — the same `-trimpath` Linux binary in the
repository Dockerfile image, with the repository bind-mounted read-only from
the Windows filesystem. Native times use the Windows binary.

| Repository | Mode | Native | Docker | Difference |
|---|---|---:|---:|---:|
| Basic fixture | with blame | 359 ms | 5.153 s | +4.8 s |
| Basic fixture | `--no-blame` | 277 ms | 2.547 s | +2.3 s |
| Generated, 15,000 commits | with blame | 1 m 41.1 s | 2 m 40.1 s | +59 s (1.58×) |
| Generated, 15,000 commits | `--no-blame` | 478 ms | 2.984 s | +2.5 s |

A Docker Desktop bind mount of a Windows path costs about 2 to 2.5 s per run
before any analysis. That is well above the 336 ms empty container start, so
most of it is Git reading the repository through the mount. Every blame call
pays that cost again, which is why blame slows down the most. A Linux host
without a VM file share was not measured.

**A real repository** — a private application repository with 2,632 commits,
14,377 tracked files and a 299 MiB working tree, timed with the native CLI.

| Mode | Time |
|---|---:|
| `--no-blame`, two runs | 11.3 s, 10.9 s |
| With blame | 19.5 s |

Here the file reads, not blame, dominate: without blame the run still reads
every tracked text file to sniff binaries and count lines. For a working tree
of this size, `--no-blame` roughly halves the run but does not bring it under a
few seconds. A first `make perfcheck` run that included this repository was
stopped while it read files at about 46 per second. The cause was not isolated,
and the recorded run above excludes the repository. `COMMITOGRAPHY_PERF_REPO`
still adds one on request.

**`--no-blame` guidance.** Blame is the only stage whose cost reaches minutes.
Use `--no-blame`, or the dashboard's blame toggle:
- for repositories with thousands of tracked files or long per-file histories;
- for runs through Docker Desktop on Windows or macOS;
- when only history, temporal and message metrics are needed.

Without blame, the report leaves out code age by year and the share of lines
surviving from the first year. Bus factor, knowledge concentration and every
other metric come from commit history and are unchanged. `docs/docker.md` and
section 12 of `phase-1.5.md` carry the same guidance. `docs/docker.md` used to
say line ownership and knowledge concentration were left out; this package
corrects that.

## WP-6.7 - Security verification

### Deliverables

Run tests and a manual review for path traversal, symlink escapes, CSRF-like
cross-origin requests, raw data exposure, cache headers and public binding.

### Acceptance criteria

- No unauthenticated state-changing request succeeds.
- No API endpoint reads arbitrary files.
- No raw history or cache file is exposed.
- The default listener is loopback-only.
- Response headers match the security contract.

### Changes

`internal/server/securitymatrix_test.go` gathers the security matrix:

- `TestRouteTraversalReadsNoFiles` sends twelve traversal spellings through
  the page, asset and API routes: `..`, `%2e%2e`, `%2f`, `%5c`, doubled
  slashes, and traversal past a real asset name. Each test follows up to three
  redirects and requires that no response is `200` or contains `go.mod`. A job
  whose `repoPath` names a file is refused with `400`.
- `TestStateChangingRoutesRequireSessionAndSameOrigin` covers job creation,
  cancellation and deletion. Each is sent without a session, without a session
  from another site, from another site, from another local port and from an
  opaque `null` origin. They answer `401 invalid_session` or
  `403 invalid_origin`. Afterwards the running job is still running and no
  second analysis started.
- `TestSecurityHeadersOnEveryResponseClass` checks `Cache-Control: no-store`,
  `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, the
  Content-Security-Policy directives and the absence of
  `Access-Control-Allow-Origin` on thirteen response classes:
  - the application page, an asset and capabilities (`200`);
  - a created job (`202`) and a conflict (`409`);
  - a missing session (`401`), a cross-origin request and a foreign `Host`
    (`403`);
  - an unknown job and an unknown page (`404`);
  - a wrong method (`405`), an oversized body (`413`) and a cleaned-path
    redirect.
- `TestSessionCookieIsProcessScopedAndStrict` checks that the cookie is
  `HttpOnly`, `SameSite=Strict`, `Path=/`, has no `Domain`, and has no
  `Expires` or `Max-Age`. Its value is a 256-bit hexadecimal token, and two
  server instances never share it.
- `TestResponsesCarryOnlyDocumentedData` runs a real analysis of this checkout
  and requires the documented fields exactly:
  - the eleven status fields;
  - the seven job-list summary fields, which carry no repository path;
  - report top-level fields that `docs/report-schema.json` defines, including
    every required one;
  - no e-mail address in the default report.
- `TestDefaultListenerIsLoopback` pins `127.0.0.1:8080`, and
  `cmd/commitography/serve_test.go` adds
  `TestServeFlagsDefaultToLoopbackWithoutBrowser`: the `serve` flags default to
  that address, no `--open` and no extra allowed root.

The cleaned-path case accepts any redirect status, because Go's router chooses
the status code and it has differed between Go versions.

### Verification

Recorded 2026-09-13 on the Windows host recorded under WP-6.1, with Go 1.27.0,
Git 2.55.0.windows.3, Node 24.18.1 and npm 12.0.2.

- `gofmt -l internal/server cmd/commitography` printed nothing and
  `go vet ./...` passed.
- The eight WP-6.7 tests passed.
- `go test -count=1 -v ./...` passed: 214 tests passed, none failed, and
  `TestValidateRepositoryRejectsSymlinkEscape` skipped for the reason recorded
  under WP-6.1.

**Acceptance criteria**

| Criterion | Evidence |
|---|---|
| No unauthenticated state-changing request succeeds | `TestStateChangingRoutesRequireSessionAndSameOrigin`; M3 host, origin and session tests; DNS rebinding tests from M5. |
| No API endpoint reads arbitrary files | `TestRouteTraversalReadsNoFiles`, the allowed-root and path tests of M3 and WP-6.5, and the manual review below. |
| No raw history or cache file is exposed | `TestResponsesCarryOnlyDocumentedData` and the route review below. Phase 1.5 writes no cache file. |
| The default listener is loopback-only | `TestDefaultListenerIsLoopback`, `TestServeFlagsDefaultToLoopbackWithoutBrowser`, and the `netstat` check under WP-6.5. |
| Response headers match the security contract | `TestSecurityHeadersOnEveryResponseClass`; every response passes through `applySecurityHeaders` before the host check. |

**Manual review — routes.** `Handler` in `internal/server/handler.go` has four
routes:

- `/assets/` serves only the embedded `render.AssetFS()`. `filesOnly` refuses
  directory paths, and the file system cannot reach the disk.
- `/api/v1/` holds the capability and job routes. The report is built from the
  job's in-memory result; there is no file download route.
- Any other `/api/` path answers `404`.
- `/` serves the fixed application shell, and any other path answers `404`.

No handler opens a file named by the request. The server writes no file; the
report is kept in memory, and `model.History` never reaches a response.

**Manual review — what a job reads.** A job reads only inside a repository that
passed the allowed-root check:

- **Git commands:** `git --version`, `rev-parse`, `symbolic-ref`,
  `rev-list --all --date-order`, `log --numstat --no-renames`,
  `ls-tree -r --name-only HEAD`, and
  `blame --line-porcelain -w -M HEAD -- <path>` for sampled files.
- **Worktree files:** tracked files as HEAD lists them, opened once to detect
  binaries and read again to count lines.
- **Repository root:** `.gitattributes` and `.commitography.yml`.
- **Git directory:** the `shallow` and `info/grafts` markers.

At startup the server lists and checks the allowed roots and reads container
markers. The client cannot supply a config path or output directory.

**Residual risk — tracked symbolic links.** Line counting opens tracked paths
with `os.Open` and `os.ReadFile`, which follow a symbolic link. A repository
inside an allowed root that tracks a link to a file outside it therefore has
that file read. Only a newline count and a binary or text decision reach the
report, never content, and only for a repository the local user chose to
analyze. This is recorded as a known limitation in section 12 of
`phase-1.5.md`.

**Manual review — listener.** A native server binds `127.0.0.1:8080` unless
`--listen` names another address. An explicit non-loopback address is honored
and announced. For such an address, the Host check still accepts only the
loopback names and the named host; a wildcard adds nothing. The Docker image
listens on `0.0.0.0` inside the container, and the documented command
publishes the port only on `127.0.0.1`.

**Dependency audit.** `npm audit --omit=dev` in `web/` found 0
vulnerabilities, so the embedded bundle's runtime packages have no advisory.
The full audit reports five advisories, all in development tooling:

| Package | Severity | Advisory | Exposure |
|---|---|---|---|
| `vitest` 2.1.9 | critical | UI server file read (GHSA-5xrq-8626-4rwp); mocker redirect path traversal (GHSA-82fw-gwwq-j7x9) | `npm run test` is `vitest run`; no UI server is started. |
| `vite` 5.4.x | high | optimized-deps `.map` path traversal (GHSA-4w7w-66w2-5vf9); `server.fs.deny` bypass on Windows (GHSA-fx2h-pf6j-xcff); `launch-editor` UNC hash disclosure (GHSA-v6wh-96g9-6wx3) | Dev server only. The build uses `vite build`; `npm run dev` starts the affected server. |
| `esbuild` ≤0.24.2 | moderate | any site can query the dev server (GHSA-67mh-4wv8-2f99) | Dev server only. |
| `vite-node`, `@vitest/mocker` | moderate | inherited from `vite` and `vitest` | Test runner only. |

The fixes need `vite` 8 and `vitest` 5, both semver-major. They were not
applied in this package because they change the frontend build toolchain; the
risk is limited to a developer running `npm run dev` on an untrusted network.
Section 12 of `phase-1.5.md` records the upgrade as a known limitation.

## WP-6.8 - Final documentation and phase closure

### Deliverables

Update:

- `README.md`
- `docs/project-overview.md`
- `docs/phase-1.5.md`
- `docs/phase-1.5-detailed.md`
- `docs/phase-2.md`
- `docs/phase-3.md`

Record the final milestone status, known limitations, test commands and manual
verification evidence.

### Acceptance criteria

- No package remains marked complete without evidence.
- The phase exit criteria table is fully checked or has an explicit blocker.
- README examples work from a clean checkout.
- Phase 1, Phase 2 and Phase 3 documents describe the relationship accurately.
- The phase is not marked released until the project release process has been
  executed separately.

### Changes

- **`docs/phase-1.5.md`**
  - The status line now says the phase is implemented on `main`, not released,
    with criterion 14 blocked.
  - The Docker bullet is corrected, and a note says no published image
    contains `serve`.
  - Section 11 is an exit criteria table giving each criterion's status and
    evidence.
  - New section 12, Known Limitations: macOS, UNC spelling of a root, tracked
    symlinks, linked worktrees in a container, blame and working-tree cost, one
    job and in-memory history, frontend development tool advisories, and no
    published build with `serve`.
  - New section 13 lists the verification commands.
- **`docs/phase-1.5-detailed.md`**: the status line and M6 roll-up now name the
  blocker, and "7. Closure Summary" is new.
- **`docs/project-overview.md`**: the Phase 1.5 paragraph says the phase was
  implemented on 2026-09-13, is not released and has one blocked criterion.
- **`docs/phase-1-detailed.md`, `docs/phase-2.md` and `docs/phase-3.md`**:
  each gains a relationship paragraph.
  - Phase 1.5 reuses the Phase 1 engine without changing the CLI, and closes
    none of the Phase 1 release items.
  - It does not satisfy Phase 2 and does not need its cache; publishing the
    image with `serve` remains Phase 2 deliverable 5.
  - It is not Phase 3 server mode, and its local protections are not an
    authentication design for a networked server.
- **`README.md`**: adds `make perfcheck`, the frontend check commands and a link
  to this file.

### Verification

Recorded 2026-09-13 on the Windows host recorded under WP-6.1, with Go 1.27.0,
Git 2.55.0.windows.3, Node 24.18.1, npm 12.0.2 and Docker Desktop 29.7.2.

**Clean checkout.** A fresh clone of `origin/main` at `6f08ef2`, which holds
every code change of this milestone. The WP-6.8 change itself touches only
documentation.

| README example | Result |
|---|---|
| `go build -o commitography ./cmd/commitography` | Built with Go only; the committed bundle needed no Node. |
| `commitography ./repo -o out/` (on the clone, `--no-blame -q`) | Wrote `out/index.html` and `out/report.json`. |
| `./commitography serve` | Printed `Commitography listening at http://127.0.0.1:18089` on the chosen port; `GET /` answered `200`. |
| `make fixtures` (its recipe, `sh testdata/build-fixtures.sh`) | Built all ten fixtures. |
| `make lint` (its recipe) | `go vet ./...` passed; `gofmt -l cmd internal` printed nothing. |
| `make test` | `go test -count=1 ./...` passed in every package. |
| `make web` (`npm ci && npm run build`) | Built; the rebuilt `internal/render/assets` matched the committed files exactly. |
| `npm run typecheck && npm run test` | Passed; 5 files and 38 tests. |
| `make docker-image` (the no-`make` Git Bash steps in `docs/docker.md`), then the README `docker run` on port 18090 with `MSYS_NO_PATHCONV=1` | The image built; `GET /` answered `200` and `/api/v1/capabilities` returned the v1 capabilities. |

`make` is not installed on this host, so each target was run as its recipe.
`make docker-smoke` and `make perfcheck` ran under M5 and WP-6.6. No release
has been tagged, so no Homebrew, Scoop, `ghcr.io` or release download exists
yet. `go install …@latest` resolves to the latest `main` commit and includes
`serve`. The README was rewritten afterwards to describe these installation
channels accurately.

**Acceptance criteria**

| Criterion | Evidence |
|---|---|
| No package remains marked complete without evidence | Each Complete M6 package has a verification section in this file naming the host, tool versions, repositories and date. The M0 to M5 records are in their milestone files. WP-6.5 is Blocked, with its reason and the macOS commands to run. |
| The phase exit criteria table is fully checked or has an explicit blocker | Section 11 of `phase-1.5.md` marks 13 criteria Met, criterion 4 Met on Windows and Linux, and criterion 14 **Blocked** with its reason. |
| README examples work from a clean checkout | The table above. |
| Phase 1, Phase 2 and Phase 3 documents describe the relationship accurately | The relationship paragraphs listed under Changes. None claims Phase 2 or Phase 3 functionality. |
| The phase is not marked released | Every status line says "not released", and no tag was created. |

**Milestone status.** M0 to M5 are Complete. M6 is **Blocked**: seven of its
eight packages are complete, and WP-6.5 needs a macOS host. Phase 1.5 is
implemented but not complete and not released.
