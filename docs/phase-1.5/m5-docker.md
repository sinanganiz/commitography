# M5 - Docker and Distribution

**Depends on:** M2 and M4. **Blocks:** M6.

M5 makes the local server usable from the existing container without breaking
the current CLI image contract.

| Package | Status |
|---|---|
| WP-5.1 Preserve CLI and add explicit server invocation | Not started |
| WP-5.2 Container path and mount documentation | Not started |
| WP-5.3 Runtime security and image behavior | Not started |
| WP-5.4 Docker smoke tests | Not started |
| WP-5.5 Release and image documentation | Not started |

---

## WP-5.1 - Preserve CLI and add explicit server invocation

### Deliverables

Keep the current `Dockerfile` entrypoint and CLI `CMD` behavior. Verify that a
command override can run:

```text
serve --listen 0.0.0.0:8080 --allowed-root /repos
```

### Acceptance criteria

- Running the image with no extra command still analyzes `/repo` into `/repo/out`.
- Running the image with `serve` starts the HTTP server.
- The image does not require a second server binary.
- The server does not silently bind the host interface.

## WP-5.2 - Container path and mount documentation

### Deliverables

Document native host path versus container-visible path for Linux, macOS and
Windows. Use explicit `--mount` examples and bind the host port to loopback.

### Acceptance criteria

- Examples use `127.0.0.1:8080:8080`.
- Examples mount repositories read-only.
- Windows PowerShell and POSIX shell examples do not mix path syntax.
- Docker Desktop file-sharing requirements are explicit.

## WP-5.3 - Runtime security and image behavior

### Deliverables

Verify the image contains only the runtime dependencies needed by Git and the
server, preserves `tini`, and does not grant write access to the repository
mount.

### Acceptance criteria

- The server cannot write report output into a read-only repository mount.
- `git` remains available inside the image.
- Signals reach the Go process through `tini`.
- The image does not expose a health endpoint or admin endpoint that is absent
  from the documented API.

## WP-5.4 - Docker smoke tests

### Deliverables

Run the image against a fixture repository and verify CLI and server modes.

### Acceptance criteria

- CLI mode creates the existing output files.
- Server mode serves the UI and completes a fixture analysis.
- The report shown through Docker matches the native server report.
- A missing or invalid mount produces a useful error.

## WP-5.5 - Release and image documentation

### Deliverables

Update README and release notes with the server command, image invocation,
port behavior and limitations. Do not change the existing Phase 1 release
claims to imply Phase 1.5 is already released.

### Acceptance criteria

- A fresh user can run native or Docker mode from the documentation.
- The documentation states that the browser sees container paths in Docker.
- No example exposes the server on all host interfaces.
