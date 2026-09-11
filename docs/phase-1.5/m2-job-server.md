# M2 - Job Manager and HTTP Server

**Depends on:** M1. **Blocks:** M3, M4, M5, M6.

M2 adds the local server lifecycle without introducing a database or a second
analysis implementation.

| Package | Status |
|---|---|
| WP-2.1 `serve` command and listener | Complete |
| WP-2.2 Embedded asset server | Complete |
| WP-2.3 Job state and bounded history | Complete |
| WP-2.4 Worker lifecycle and cancellation | Complete |
| WP-2.5 Progress projection for polling | Complete |
| WP-2.6 Result, error and stale handling | Complete |
| WP-2.7 Graceful shutdown | Complete |

---

## WP-2.1 - `serve` command and listener

### Deliverables

Add a Cobra `serve` subcommand under `cmd/commitography` with:

| Flag | Default | Behavior |
|---|---|---|
| `--listen` | `127.0.0.1:8080` | HTTP listen address |
| `--open` | `false` | Open the default browser after binding |
| `--allowed-root` | current directory | Repeatable filesystem root |

The parent CLI command and its flags must continue to work unchanged.

### Acceptance criteria

- `commitography serve --help` documents every server flag.
- The server prints the actual URL after binding.
- Bind errors are actionable and return a non-zero process status.
- `--open` failure is a warning, not a server failure.
- No server default binds to `0.0.0.0`.

## WP-2.2 - Embedded asset server

### Deliverables

Serve the React application and its static assets from the existing embedded
frontend build. The server must provide a stable root document and API paths
without requiring a sibling `dist` directory.

### Acceptance criteria

- A clean Go build with the committed frontend assets serves the UI.
- `/api/v1/*` is never handled by the frontend fallback.
- Unknown application routes return the application shell without exposing
  filesystem paths.
- The static CLI render path remains independent and self-contained.

## WP-2.3 - Job state and bounded history

### Deliverables

Create `internal/jobs` with opaque IDs, statuses, timestamps, repository display
metadata, latest progress event, warning summary and report reference.

Retain ten jobs maximum. Eviction is deterministic: the oldest terminal job is
evicted first; an active job is never evicted.

### Acceptance criteria

- IDs are generated with cryptographic randomness.
- Job state is safe for concurrent HTTP reads and worker writes.
- A second active start returns a conflict without creating a job.
- The ten-job limit is enforced under concurrent completion.

## WP-2.4 - Worker lifecycle and cancellation

### Deliverables

Run one analysis worker per server process. Connect job cancellation to the
analysis context and Git subprocesses.

### Acceptance criteria

- Start returns before a long analysis completes.
- Cancel transitions an active job to `cancelled` after the worker exits.
- Repeated cancel requests are idempotent.
- A cancelled job cannot later become `succeeded`.
- A client disconnect does not automatically cancel the job.

## WP-2.5 - Progress projection for polling

### Deliverables

Store the latest progress event and expose it through status polling. Include
sequence, stage, detail, elapsed time, current/total counters and estimated
fraction.

### Acceptance criteria

- Polling never returns a progress sequence lower than one already observed.
- The status endpoint remains useful if a stage has no fraction.
- The server does not fabricate exact percentages from missing data.
- The UI can distinguish an estimate from a counted fraction.

## WP-2.6 - Result, error and stale handling

### Deliverables

Store the completed report in memory only. Model failures, cancellation and
repository mutation as separate terminal states.

### Acceptance criteria

- Only `succeeded` jobs expose a report endpoint.
- Failed jobs expose a safe error kind and user-facing message without raw
  command output or secrets.
- Stale jobs expose the reason and suggest a rerun.
- Reports remain available until deletion or deterministic eviction.

## WP-2.7 - Graceful shutdown

### Deliverables

Handle process signals, stop accepting new jobs, cancel active work and wait for
the worker with a bounded shutdown timeout.

### Acceptance criteria

- The active Git process receives cancellation on shutdown.
- The server does not leave an orphaned analysis process.
- Shutdown completes within the documented timeout or reports that it did not.
- A second server start does not reuse stale in-memory job state.
