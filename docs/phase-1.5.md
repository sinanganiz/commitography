# Phase 1.5 - Local Web Dashboard and Runner

**Status:** Implemented on `main`, not released. Verification recorded
2026-09-13; exit criterion 14 is blocked until a macOS host verifies path
behavior.

**Position:** An intermediate phase between Phase 1 and Phase 2. This phase is
not Phase 3 server mode. It adds a local operator interface around the Phase 1
analysis engine without adding remote repository management, accounts, a
scheduler, or persistent server storage.

---

## 1. Goal

Allow a developer to run Commitography locally as either a CLI or a localhost
web application:

```text
choose a repository path
        -> start an analysis
        -> watch progress and stage details
        -> cancel or wait for completion
        -> inspect the report in the same application
```

The existing command-line workflow remains valid and unchanged:

```bash
commitography ./repo -o out/
```

The new local workflow is:

```bash
commitography serve --open
```

The server binds to `127.0.0.1:8080` by default. `--open` is opt-in and opens
the browser after the listener is ready. The server prints the URL regardless
of whether a browser is opened.

---

## 2. Definition of Done

Phase 1.5 is complete when all of the following are true:

1. The existing CLI output, exit codes, static HTML output, Wrapped output and
   offline file-opening behavior continue to pass their existing tests.
2. `commitography serve` starts a local web application without a second
   runtime binary or a separate Node.js runtime.
3. The UI accepts a repository path that is inside one of the configured
   allowed roots and rejects unsafe or invalid paths before starting work.
4. A user can start one analysis, see its state and estimated progress, cancel
   it, and see a useful error when it fails.
5. A successful job displays the same report data as the CLI-generated
   `report.json` and renders the existing repository metrics in the same UI.
6. The server keeps the ten most recent job records for the lifetime of the
   process and rejects a new start request while another job is active.
7. A repository that changes during an analysis is reported as `stale` and is
   never presented as a successful current report.
8. The server API is same-origin, protected by a process-scoped session cookie,
   and does not expose raw history, cache files, arbitrary file reads or output
   directory writes.
9. The React and MUI frontend is embedded in the Go binary and makes no CDN,
   font, image or runtime network request other than calls to the local API.
10. The existing Docker image can run the server explicitly while retaining
    its current CLI default command.
11. Native and Docker usage are documented for Windows, macOS and Linux,
    including the difference between host paths and container paths.
12. Unit, API, frontend, browser and Docker smoke tests cover the exit criteria
    that can be automated, and manual verification is recorded for the rest.

---

## 3. Product Boundaries

### In scope

- A `serve` subcommand in the existing `commitography` binary.
- A local React application using MUI for controls and layout primitives.
- The existing custom SVG visualizations and report semantics.
- One active analysis job at a time.
- A process-local history of the ten most recent jobs.
- Path entry, path validation, advanced analysis options and privacy controls.
- Estimated progress, stage details, warnings, cancellation and stale results.
- A versioned localhost API under `/api/v1/`.
- Embedded frontend assets served by the same Go binary.
- Explicit Docker server usage through the existing image.

### Out of scope

- User accounts, SSO or organization authentication.
- Binding a native server to a public or LAN interface by default.
- Remote clone management, Git credentials or repository mirrors.
- Multiple repositories registered in a persistent catalog.
- Scheduled refreshes or background daemons.
- A database or restart-persistent job history.
- Browser upload of repository contents.
- A native OS file-picker launcher.
- Phase 2 incremental cache implementation.
- Phase 3 organization rollups or server-side historical storage.

---

## 4. User Experience

The application has four primary views:

1. **Start:** repository path, repository validation state, Start analysis
   action and an Advanced options disclosure.
2. **Progress:** job identifier, repository name, estimated percentage, active
   stage, detail text, elapsed time, warnings and Cancel action.
3. **Result:** the repository report rendered with the existing dashboard
   sections, plus a way back to recent jobs.
4. **Recent jobs:** the ten in-memory jobs with status, repository name,
   start/end time and an action for completed reports.

The first screen exposes only the path and the primary action. Advanced options
contain `--no-blame`, `--per-author`, `--anonymize`, `--allow-shallow`,
`--count-merges`, `--since` and `--until`. Per-author output remains opt-in and
the UI must explain that it names contributors.

The progress percentage is explicitly an estimate. The UI must display the
current stage and useful counters so that a slow Git operation is not shown as
an exact measurement. A stage may report an indeterminate fraction when a
reliable fraction is not available.

The browser cannot obtain an arbitrary host filesystem path from a normal web
page. Native usage therefore accepts a typed path. Docker usage accepts the
path visible inside the container, normally `/repos/project`, after the host
directory has been mounted.

---

## 5. Invocation and Path Rules

The initial command surface is:

```text
commitography serve [flags]
```

Required behavior:

- `--listen` defaults to `127.0.0.1:8080`.
- `--open` is false by default and opens the default browser when true.
- `--allowed-root` is repeatable. When omitted, the current working directory
  is the only allowed root.
- A repository path must resolve inside an allowed root after cleaning,
  evaluating symlinks and applying the platform's path comparison rules.
- The server accepts repository paths, not arbitrary files or Git object paths.
- The server never honors a repository config `output_dir` for server output.
- The server does not accept a client-supplied `configPath`.
- The repository-local `.commitography.yml` may still be read by the analysis
  engine, subject to the same validation as CLI analysis.

The server must not offer a native file listing endpoint. The UI may validate a
typed path by submitting it to the API, but it must not enumerate the allowed
root to autocomplete paths.

---

## 6. Architecture

The Phase 1 pipeline is moved behind a shared analysis service:

```text
CLI adapter ------------------+
                              |
                              v
                        internal/analysis
                              |
                    collect -> filter -> aggregate
                              |
                         aggregate.Report
                              |
       CLI render ------------+------------ HTTP job result
```

The local server is composed of:

- `internal/analysis`: context-aware, output-free analysis orchestration.
- `internal/jobs`: one-active-job lifecycle, progress, cancellation and recent
  history.
- `internal/server`: HTTP handlers, session protection and embedded asset
  serving.
- `web/src`: React/MUI application and shared report components.
- `internal/render`: static HTML rendering for CLI output, using the same
  report components' data contract where practical.

The analysis engine returns a privacy-transformed `aggregate.Report`. Raw
`model.History` values never cross the HTTP API boundary.

Phase 1.5 does not introduce a cache. The job manager may later consume the
Phase 2 cache through an explicit interface, but a Phase 1.5 installation must
work correctly with a full read on every job.

---

## 7. API Summary

The API is same-origin and versioned:

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/v1/capabilities` | Return server capabilities and allowed-root metadata safe for the UI |
| `GET` | `/api/v1/jobs` | List the ten recent jobs |
| `POST` | `/api/v1/jobs` | Validate input and start an analysis |
| `GET` | `/api/v1/jobs/{id}` | Return status, progress, warnings and error information |
| `GET` | `/api/v1/jobs/{id}/report` | Return the completed `aggregate.Report` |
| `POST` | `/api/v1/jobs/{id}/cancel` | Request cancellation of an active job |
| `DELETE` | `/api/v1/jobs/{id}` | Remove a completed, failed, cancelled or stale job from memory |

`POST /api/v1/jobs` returns `202 Accepted` with an opaque job ID. A second
start request while a job is active returns `409 Conflict`.

Job statuses are `queued`, `running`, `succeeded`, `failed`, `cancelled` and
`stale`. Phase 1.5 never queues a second job, so `queued` is reserved for the
short interval between acceptance and worker start.

The report endpoint returns schema version 1 and must remain compatible with
`docs/report-schema.json`. The API envelope is separate from the report schema.

---

## 8. Security and Privacy

- Native server binds to loopback by default.
- Each server process creates a cryptographically random session secret.
- The UI receives an `HttpOnly`, `SameSite=Strict` session cookie.
- State-changing requests validate the request `Origin` and same-host rules.
- Every request must name the server as `localhost`, `127.0.0.1`, `[::1]` or
  the explicit listen address. Other `Host` values are refused before a session
  cookie is issued, which blocks DNS rebinding.
- CORS is not enabled.
- Job IDs are generated with `crypto/rand` and are not sequential.
- Responses set `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`,
  `Referrer-Policy: no-referrer` and a restrictive Content Security Policy.
- Raw history, cache artifacts, arbitrary files and directory listings are not
  available through HTTP.
- Per-author data remains opt-in.
- E-mail hashing and anonymization keep their Phase 1 defaults.
- Repository paths are not included in public diagnostics unless needed by the
  local UI.
- Docker examples publish the host port only on `127.0.0.1` and mount the
  repository read-only.

The server is not a general-purpose network service. A public or LAN-facing
deployment is outside this phase and belongs to a future server-mode design.

---

## 9. Docker Contract

The existing image remains a CLI image by default. Server usage is explicit:

```bash
docker run --rm \
  --publish 127.0.0.1:8080:8080 \
  --mount type=bind,source="$PWD",target=/repos,readonly \
  ghcr.io/sinanganiz/commitography:latest \
  serve --listen 0.0.0.0:8080 --allowed-root /repos
```

The container listens on `0.0.0.0` only because the host port mapping is the
container boundary. The host binding remains loopback-only.

No published image contains `serve` yet. Until a release does,
`make docker-image` builds `commitography:local` from a checkout.

The Docker contract must document:

- Native host path versus container-visible path.
- `--mount` instead of `-v`. Docker Engine refuses a missing bind source, but
  Docker Desktop creates an empty folder in its place, so the tool itself
  reports a missing mount.
- Docker Desktop file sharing on macOS and Windows.
- Read-only repository mounts.
- Linked worktrees and submodules that require additional mounts.
- The performance impact of filesystem virtualization and the `--no-blame`
  option.

The user guide that documents this contract is [`docker.md`](docker.md).

---

## 10. Phase Relationships

Phase 1.5 depends on the Phase 1 analysis behavior but not on Phase 2 cache or
CI work. It does not close any Phase 2 exit criterion. Phase 2 may later add a
cache interface consumed by both CLI and local server.

Phase 1.5 is deliberately smaller than Phase 3. It has no repository catalog,
remote credentials, scheduler, authentication provider, persistent reports or
organization rollups. A future Phase 3 server may reuse the API and analysis
contracts, but must not be treated as a direct continuation of the in-memory
job manager.

---

## 11. Exit Criteria

The detailed work packages in [`phase-1.5-detailed.md`](phase-1.5-detailed.md)
are the source of truth for implementation status. Phase 1.5 is complete only
when every criterion below is met. The evidence is recorded in the milestone
files under [`phase-1.5/`](phase-1.5/), most of it in
[`m6-verification.md`](phase-1.5/m6-verification.md).

| # | Criterion | Status | Evidence |
|---:|---|---|---|
| 1 | Shared analysis produces byte-equivalent report values for CLI and server runs with the same inputs. | Met | `TestCLIAndAnalysisServiceProduceTheSameReport`; the Docker smoke test compares a container report with the native server's field by field (M5 WP-5.4). |
| 2 | Existing CLI tests and static render tests pass without weakening their assertions. | Met | `go test ./...` passes with the fixtures built; the only skip is a Windows symlink test covered on Linux (WP-6.1). |
| 3 | `serve` starts, prints its URL and serves the React application on the default loopback address. | Met | With default flags the server listened only on `127.0.0.1:8080` and printed its URL (WP-6.5); the browser audit runs the application (WP-6.4). |
| 4 | Allowed-root validation rejects outside paths, symlink escapes, invalid repositories and unsafe Git directory relationships. | Met on Windows and Linux | Path and endpoint tests (M3, WP-6.2), Windows junction, case and UNC tests and Linux symlink escape (WP-6.5). macOS: see 14. |
| 5 | A valid job can be started, observed through polling, cancelled and removed. | Met | `TestAPIEndpointMatrix` (WP-6.2) and the browser audit (WP-6.4). |
| 6 | A second start request is rejected while the first job is active. | Met | `409 active_job` with exactly one started analysis (WP-6.2). |
| 7 | Progress events include stage, sequence, detail and estimated fraction semantics. | Met | Progress contract and sequence tests (M1, M2); estimates labelled in the UI (WP-6.3, WP-6.4). |
| 8 | A successful report is rendered in the same application and validates against the existing report schema. | Met | Report rendered without a reload (WP-6.4); report fields checked against the schema (WP-6.7). |
| 9 | Failure, cancellation and stale repository states are distinct and useful. | Met | Job view and recent jobs tests and audits (WP-4.7, WP-6.3, WP-6.4). |
| 10 | The ten-job in-memory history is bounded and oldest entries are evicted deterministically. | Met | `TestHistoryKeepsExactlyTheTenNewestJobs` and eviction through the API (WP-6.2); retained memory measured (WP-6.6). |
| 11 | Session, Origin, response-header and no-raw-data security tests pass. | Met | Security matrix and manual review (WP-6.7). |
| 12 | The React/MUI bundle is embedded and the static offline dashboard remains self-contained. | Met | Render and bundle URL tests (WP-6.3); static pages audited offline (WP-4.8, WP-6.4). |
| 13 | Docker server smoke tests pass with a read-only mount and a loopback host binding. | Met | `make docker-smoke` (M5 WP-5.4). |
| 14 | Windows, macOS and Linux path behavior is either tested or explicitly recorded as manual verification. | **Blocked** | Windows and Linux are tested (WP-6.5). No macOS host was available; the commands to run are recorded in WP-6.5. |
| 15 | README, project overview and phase relationship documentation describe the new mode without claiming Phase 2 or Phase 3 functionality. | Met | WP-5.5 and WP-6.8. |

Phase 1.5 is therefore not complete until criterion 14 is verified on macOS.
It is not released either: a release follows the project release process
separately.

---

## 12. Known Limitations

- Path behavior has not been verified on macOS.
- A UNC spelling of a folder inside an allowed root is refused. Type the root
  in the form it was given to `--allowed-root`.
- Counting lines follows a tracked symbolic link that points outside the
  allowed root. Only a line count, never file content, reaches the report.
- A linked worktree cannot be analyzed from a container on its own.
- Blame dominates analysis time on long histories, and a working tree with many
  files adds a read of every file. Skipping blame helps; Docker Desktop mounts
  are slower still. Measurements are in WP-6.6.
- One analysis runs at a time, and jobs and reports are lost when the server
  stops.
- The frontend development tools, `vite` 5 and `vitest` 2, have published
  advisories for their development servers. The fixes need major upgrades that
  have not been made. The embedded bundle's runtime packages have none, and
  `npm run dev` should not be used on an untrusted network. Details are in
  WP-6.7.
- No published binary or image includes `serve`.

## 13. Verification Commands

```bash
make fixtures                      # build the test repositories
go test ./...                      # Go unit, API and security tests
cd web && npm run typecheck && npm run test   # frontend types and components
cd web && npm run e2e              # browser audit; needs Chrome, Go and Git
make docker-smoke                  # container image smoke tests
make perfcheck                     # performance and resource measurements
```

Without `make`, run the Go commands the Makefile targets name.
