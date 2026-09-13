# M5 - Docker and Distribution

**Depends on:** M2 and M4. **Blocks:** M6.

M5 makes the local server usable from the existing container without breaking
the current CLI image contract.

| Package | Status |
|---|---|
| WP-5.1 Preserve CLI and add explicit server invocation | Complete |
| WP-5.2 Container path and mount documentation | Complete |
| WP-5.3 Runtime security and image behavior | Complete |
| WP-5.4 Docker smoke tests | Complete |
| WP-5.5 Release and image documentation | Not started |

---

## Clarified scope - 2026-09-13

The repository was reviewed against this milestone before implementation. The
work packages below remain normative; this section records what they mean for
the code as it exists.

### Findings

1. The `Dockerfile` expects goreleaser to place a prebuilt Linux
   `commitography` binary in the build context. goreleaser has never been
   executed (Phase 2 M1), so no published image contains `serve`, and there is
   no documented way to build the image locally.
2. The image declares no `EXPOSE` and no `HEALTHCHECK`. Adding `EXPOSE` would
   let `docker run -P` publish the server on every host interface.
3. `serve` compares the `Origin` header with the `Host` header but never checks
   `Host` itself. A DNS-rebinding page served from its own name passes both
   checks, receives a session cookie and can start jobs and read reports. The
   M0 contract already lists host rejection as a `403`; it was not enforced.
   Inside a container the server listens on `0.0.0.0`, so this matters here.
4. `safe.directory` is written to root's global Git configuration. A container
   started with `--user` has no access to that file, and Git then refuses a
   mounted repository owned by another uid.
5. `serve` without `--listen` binds loopback inside the container, which a
   published port cannot reach, and a wildcard listener prints
   `http://[::]:8080`, which is not an address a browser should open.
6. The CLI default writes `/repo/out` and therefore needs a writable mount. The
   server writes nothing and works with a read-only mount.
7. Git Bash rewrites container paths such as `/repos` into Windows paths unless
   `MSYS_NO_PATHCONV=1` is set.
8. Docker Desktop on Windows does not reject a bind source that does not exist.
   With either `--mount` or `-v` it creates an empty host directory, and the
   CLI then reports only `/repo is not a git repository`. Documentation cannot
   rely on Docker to catch a mistyped source, so the tool's own errors must
   explain a missing mount.

### Decisions

| ID | Decision |
|---|---|
| D1 | `make docker-image` cross-compiles a static binary for the Docker daemon's architecture into `dist/docker/` and builds `commitography:local`. The `Dockerfile` stays compatible with goreleaser. M5 does not publish an image. |
| D2 | The image keeps no `EXPOSE` and no `HEALTHCHECK`, and says why in the `Dockerfile`. |
| D3 | `serve` never binds a reachable interface silently: a non-loopback listener outside a container prints a warning, and inside a container a loopback listener prints how to publish the server correctly. A wildcard listener prints loopback guidance instead of a `0.0.0.0` URL. |
| D4 | Every request must carry a `Host` of `localhost`, `127.0.0.1`, `[::1]` or the explicit non-wildcard listen address. The port is not compared, because a published port may be remapped. Other hosts receive `403 invalid_host`. |
| D5 | `safe.directory` moves to the system Git configuration so any uid can read mounted repositories. The image user stays root so the CLI default can still write `/repo/out`. |
| D6 | Docker smoke tests are Go tests behind the `dockersmoke` build tag, run by `make docker-smoke`. They build the image themselves, or test `COMMITOGRAPHY_IMAGE` when it is set, and skip when Docker or the fixtures are unavailable. |
| D7 | `docs/docker.md` is the single Docker guide. README and `CHANGELOG.md` describe the server as unreleased until a tagged release includes it. |

### Execution order

1. **WP-5.1:** D1, D2 and D3, with listener tests, then verify the default
   command and the `serve` override against a locally built image.
2. **WP-5.2:** `docs/docker.md` covering host and container paths for Linux,
   macOS, Windows PowerShell and Git Bash, read-only `--mount`, loopback
   publishing, Docker Desktop file sharing, worktrees, submodules and
   filesystem performance.
3. **WP-5.3:** D4 and D5, then verify read-only analysis, Git, `tini` signal
   delivery and the absence of undocumented endpoints.
4. **WP-5.4:** D6, including report parity with the native server and useful
   errors for a missing mount, a missing allowed root and a mistyped bind
   source that Docker creates as an empty directory.
5. **WP-5.5:** README, `CHANGELOG.md` and a fresh-user walkthrough of every
   documented command.

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

### Verification

Recorded 2026-09-13 on Windows 11 Pro 10.0.26200 with Docker Desktop 29.7.2
(Linux engine, amd64), Go 1.27.0 and the image's Git 2.45.4, against the
generated `basic` fixture. The image was built with the `make docker-image`
recipe; `make` itself is not installed on that host, so its commands were run
in Git Bash.

- With no command, a mounted copy of `basic` produced `out/index.html` and
  `out/report.json` and exited `0`.
- `serve --listen 0.0.0.0:8080 --allowed-root /repos` answered `200` through
  `--publish 127.0.0.1:18090:8080`, and `docker stop` ended it in 364 ms with
  exit code `0`.
- `/usr/local/bin` holds only `commitography`; the entrypoint and `CMD` are
  unchanged.
- The image declares no exposed port, so `docker run -P` published nothing.
- A wildcard listener in the container announces the host loopback URL and
  `--publish` form instead of `0.0.0.0`; a loopback listener in the container
  explains why it is unreachable. Outside a container, a non-loopback listener
  prints a warning. These rules are unit-tested in `internal/server`.

## WP-5.2 - Container path and mount documentation

### Deliverables

Document native host path versus container-visible path for Linux, macOS and
Windows. Use explicit `--mount` examples and bind the host port to loopback.

### Acceptance criteria

- Examples use `127.0.0.1:8080:8080`.
- Examples mount repositories read-only.
- Windows PowerShell and POSIX shell examples do not mix path syntax.
- Docker Desktop file-sharing requirements are explicit.

### Verification

The guide is [`docs/docker.md`](../docker.md). Its commands were run as written
on 2026-09-13, on the Windows host recorded under WP-5.1, against copies of the
`basic` and `coupling` fixtures. The server examples were started detached so
the API could be exercised; everything else was unchanged.

- The PowerShell server command answered through `127.0.0.1:8080`; a job for
  `/repos/basic` succeeded and its report returned 50 analyzed commits.
- The Git Bash server command, with `MSYS_NO_PATHCONV=1` and `pwd -W`, completed
  `/repos/coupling`. Typing the host path instead was refused with
  `invalid_repository_path`.
- Without `MSYS_NO_PATHCONV=1`, Git Bash turned `/repos` into
  `C:/Program Files/Git/repos`, as the guide warns.
- Both read-only CLI commands wrote `index.html` and `report.json` to the
  separately mounted output folder and exited `0`.
- The bash and PowerShell image build steps each produced a Linux ELF binary
  and an image; the PowerShell steps left no `GOOS`, `GOARCH` or `CGO_ENABLED`
  behind.
- The Linux and macOS commands are the Git Bash commands without the two
  Windows adaptations. They were not run on Linux or macOS; that evidence
  belongs to WP-6.5.

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

### Changes

- **D4:** every request must name the server as `localhost`, `127.0.0.1`,
  `[::1]` or the explicit listen address, before any session cookie is issued.
  Before this change a foreign `Host` received a cookie and a matching `Origin`
  created a job through the container's published port.
- **D5:** `safe.directory` is in `/etc/gitconfig`, so the image also reads
  mounted repositories under `--user`. Before, uid 1000 was told
  `/repo is not a git repository`.
- `ca-certificates` is no longer requested, since the tool never contacts a
  remote. Alpine's `git` package still installs it through `libcurl`.
- `/assets/` listed the embedded files. Directory paths now answer `404`, as
  the security contract requires no directory listing.

### Verification

Recorded 2026-09-13 on the Windows host recorded under WP-5.1, against copies
of the `basic` fixture and a 2,632-commit repository.

- Installed packages are the Alpine base, `tini`, `git` and the libraries
  `git` depends on. `git --version` reports 2.45.4.
- The server analyzed `basic` from a read-only mount. Against a writable mount,
  the repository's recursive listing of paths, sizes, modification times and
  modes was identical before and after the analysis.
- The CLI ran as uid 1000 against a read-only repository and wrote its output
  to a separate mount.
- Through the published port, `127.0.0.1` and `localhost` answered `200`;
  `attacker.example` and `192.168.1.20` answered `403 invalid_host` without a
  cookie; a rebinding `POST` carrying a valid cookie answered `403`.
- `/healthz`, `/health`, `/metrics`, `/debug/pprof/`, `/debug/vars`,
  `/api/v1/admin`, `/api/v1/health`, `/api/v2/jobs` and `/.env` answered `404`,
  and so does `/assets/` after the change above.
- PID 1 is `/sbin/tini`. `docker stop` during a running server analysis ended
  the container in 378 ms with exit code `0`; during a CLI analysis it ended in
  313 ms with exit code `143`, the SIGTERM the Go process received.

## WP-5.4 - Docker smoke tests

### Deliverables

Run the image against a fixture repository and verify CLI and server modes.

### Acceptance criteria

- CLI mode creates the existing output files.
- Server mode serves the UI and completes a fixture analysis.
- The report shown through Docker matches the native server report.
- A missing or invalid mount produces a useful error.

### Changes

- `internal/dockersmoke` holds the smoke tests behind the `dockersmoke` build
  tag, run by `make docker-smoke`. They build a throwaway image from the
  checkout, or test `COMMITOGRAPHY_IMAGE`, and remove what they start.
- Inside a container, a path Git does not recognize and a missing allowed root
  print a mount hint after the error. Outside a container the output is
  unchanged. `serve` warns at startup when an allowed root is empty, which is
  what a mistyped Docker Desktop mount source produces.
- The dashboard's *not a Git repository* guidance mentions a mistyped mount
  source.

### Verification

Recorded 2026-09-13 on the Windows host recorded under WP-5.1. `make` is not
installed there, so the target's command,
`go test -tags dockersmoke -count=1 -v ./internal/dockersmoke`, was run
directly: 8 tests passed in 44.7 s against an image built from the checkout.

| Criterion | Test and result |
|---|---|
| CLI mode creates the existing output files | `TestCLIDefaultCommandWritesTheDashboard`: the default command wrote `out/index.html` and a schema-version-1 `out/report.json`. `TestCLIReadsReadOnlyRepositoriesAsAnyUser`: uid 1000 analyzed a read-only repository into a separate mount, and the default output inside a read-only mount failed with `read-only file system` and exit code `1`. |
| Server mode serves the UI and completes a fixture analysis | `TestServerModeMatchesTheNativeServer`: `/`, `app.js` and `app.css` answered `200` and a `basic` job with per-author metrics succeeded from a read-only mount. |
| The Docker report matches the native server report | The same test ran the job through an in-process native server; every report field except `generatedAt` and `toolVersion` was equal. |
| A missing or invalid mount produces a useful error | `TestMissingMountsExplainThemselves`: nothing mounted at `/repo` exits `2` with a mount hint; nothing mounted at the allowed root exits `1` with a mount hint; a mistyped source, which Docker Desktop mounted as an empty folder, printed the hint; an empty allowed root produced the startup warning. |

The suite also re-verifies WP-5.1 and WP-5.3: the image contract, `git`, no
exposed port or health check, `403` for a foreign host, `404` for undocumented
paths, a graceful `docker stop` through `tini`, and a mounted repository whose
files are unchanged by an analysis.

Two results differed from expectations and were resolved:

- The CLI names the repository after the path it analyzes, so the default
  command reports `repo`. The test expects that, and
  [`docs/docker.md`](../docker.md) shows how to keep the real name.
- Directory modification times proved unusable as evidence of writes. On the
  host, 76 of 166 directory times shifted by up to 88 ms in the ten seconds
  after copying a fixture, and 66 shifted when a container that ran nothing
  mounted the settled copy. The write check therefore compares files by size,
  time and mode and directories by path.

## WP-5.5 - Release and image documentation

### Deliverables

Update README and release notes with the server command, image invocation,
port behavior and limitations. Do not change the existing Phase 1 release
claims to imply Phase 1.5 is already released.

### Acceptance criteria

- A fresh user can run native or Docker mode from the documentation.
- The documentation states that the browser sees container paths in Docker.
- No example exposes the server on all host interfaces.
