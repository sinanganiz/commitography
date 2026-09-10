# M6 - Verification and Release Documentation

**Depends on:** M1, M2, M3, M4 and M5.

M6 is the release gate. It records evidence rather than relying on source
inspection for behavior that can be exercised.

| Package | Status |
|---|---|
| WP-6.1 Analysis and CLI regression tests | Not started |
| WP-6.2 Job lifecycle and API tests | Not started |
| WP-6.3 Frontend build and component tests | Not started |
| WP-6.4 Browser end-to-end audit | Not started |
| WP-6.5 Cross-platform path verification | Not started |
| WP-6.6 Performance and resource verification | Not started |
| WP-6.7 Security verification | Not started |
| WP-6.8 Final documentation and phase closure | Not started |

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
