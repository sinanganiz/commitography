# M6 - Verification and Release Documentation

**Depends on:** M1, M2, M3, M4 and M5.

M6 is the release gate. It records evidence rather than relying on source
inspection for behavior that can be exercised.

| Package | Status |
|---|---|
| WP-6.1 Analysis and CLI regression tests | Complete |
| WP-6.2 Job lifecycle and API tests | Not started |
| WP-6.3 Frontend build and component tests | Not started |
| WP-6.4 Browser end-to-end audit | Not started |
| WP-6.5 Cross-platform path verification | Not started |
| WP-6.6 Performance and resource verification | Not started |
| WP-6.7 Security verification | Not started |
| WP-6.8 Final documentation and phase closure | Not started |

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

## WP-6.3 - Frontend build and component tests

### Deliverables

Add type, build and component-level tests for form validation, progress states,
terminal states, report rendering and static bootstrap.

### Acceptance criteria

- `npm.cmd run typecheck` passes.
- `npm.cmd run build` passes.
- The bundle contains no external `http` or `https` asset references.
- Static embedded data and server-fetched data render the same report sections.

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

## WP-6.5 - Cross-platform path verification

### Deliverables

Verify path and listener behavior on Windows, macOS and Linux. Manual evidence
must include OS version, Go version, Git version and date.

### Acceptance criteria

- Windows drive, UNC, case and junction cases are covered.
- macOS and Linux symlink escape cases are covered.
- Native loopback binding and `--open` behavior are recorded.
- Docker path behavior is recorded separately from native behavior.

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
