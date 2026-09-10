# M1 - Shared Analysis Engine

**Depends on:** M0. **Blocks:** M2, M3, M6.

M1 extracts the current synchronous CLI pipeline into a reusable service. The
goal is reuse, not a second implementation of Git analysis.

| Package | Status |
|---|---|
| WP-1.1 Analysis service types and options | Complete |
| WP-1.2 Extract the CLI pipeline | Complete |
| WP-1.3 Context-aware Git commands | Complete |
| WP-1.4 Structured progress events | Complete |
| WP-1.5 Warning and output isolation | Complete |
| WP-1.6 Repository consistency check | Complete |
| WP-1.7 CLI parity tests and adapter | Not started |

---

## WP-1.1 - Analysis service types and options

### Deliverables

Create `internal/analysis/` with the shared options, progress event, result and
service contracts described in `docs/phase-1.5-detailed.md`.

The service options must cover all analysis behavior exposed by the current
CLI except output rendering and Wrapped file generation. The service returns a
privacy-transformed `aggregate.Report`.

### Acceptance criteria

- The package has no dependency on `cobra`, HTTP, browser code or filesystem
  output rendering.
- The options preserve current defaults.
- The returned report validates against `docs/report-schema.json`.
- Context cancellation is part of the public service contract.

## WP-1.2 - Extract the CLI pipeline

### Deliverables

Move the orchestration currently in `internal/cli/run.go` into the analysis
service without changing the order of preflight, config, collection, identity,
filtering and aggregation.

The CLI becomes an adapter that resolves flags, calls the service and then
renders JSON, HTML or Wrapped output exactly as it does today.

### Acceptance criteria

- The CLI produces the same report fields for the existing fixtures.
- `--json` still writes only `report.json`.
- `--wrapped` still writes the Wrapped page and enforces its commit threshold.
- No HTTP package is imported by the analysis package.

## WP-1.3 - Context-aware Git commands

### Deliverables

Extend `internal/gitcmd` with context-aware command helpers. All Git processes
started by collection, preflight, blame and code metrics must be cancellable.

### Acceptance criteria

- Cancelling the context terminates the Git child process.
- No analysis path silently falls back to an uncancellable `exec.Command`.
- Existing command argument boundaries and shell-injection protections remain.
- Git stderr is returned as a structured analysis error, not sent to the HTTP
  response directly.

## WP-1.4 - Structured progress events

### Deliverables

Replace direct progress printing inside the engine with a sink that emits
sequence-numbered events for:

- preflight
- collection
- identity resolution
- filtering
- temporal metrics
- code metrics and blame
- message metrics
- social metrics
- notables
- finalization

Collection and blame should provide counted progress where practical. Other
stages may provide an estimated or indeterminate fraction.

### Acceptance criteria

- Events are monotonic by sequence number per run.
- Fractions are either nil or within `[0, 1]`.
- The event contains a human-readable detail string.
- The CLI progress adapter continues to write progress only to stderr.

## WP-1.5 - Warning and output isolation

### Deliverables

Replace the global `config.Warn` mutation with a per-run warning sink or an
equivalent isolated mechanism. Ensure the server path never uses `output_dir`.

### Acceptance criteria

- Two analyses cannot mix configuration warnings.
- The server cannot be made to write into an arbitrary path through
  `.commitography.yml` or an API request.
- CLI warning behavior remains compatible with existing tests.

## WP-1.6 - Repository consistency check

### Deliverables

Capture the repository identity and relevant HEAD/ref state before collection
and check it again before returning success. The exact snapshot must be stable
enough to detect a branch switch or new commit during the run.

### Acceptance criteria

- A changed HEAD produces a distinct stale result for the server adapter.
- A stable repository does not produce false stale results.
- The CLI preserves its current behavior unless the CLI explicitly opts into
  stale detection.
- The check does not expose raw refs or paths through the HTTP API.

## WP-1.7 - CLI parity tests and adapter

### Deliverables

Update `internal/cli` tests and add parity fixtures comparing direct service
execution with the CLI report path.

### Acceptance criteria

- `go test ./...` passes.
- Existing exit code tests remain unchanged or are strengthened.
- A fixture report from CLI and service matches field by field apart from
  timestamps that are explicitly time-dependent.
