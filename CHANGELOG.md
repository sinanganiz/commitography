# Changelog

## Unreleased

These changes are on `main` and not part of a tagged release. No published
binary or container image includes them yet.

### Local web dashboard

- `commitography serve` runs the analysis behind a dashboard in the browser:
  repository path entry with advanced options, progress by stage with an
  estimated percentage, cancellation, the report in the same page, and the ten
  most recent jobs.
- The static dashboard and the local dashboard render the report with the same
  React components. The static page stays a single self-contained file.
- Flags: `--listen` (default `127.0.0.1:8080`), `--open` and a repeatable
  `--allowed-root` (default: the current directory).

### Container image

- The image keeps its CLI default. Override the command to run the dashboard:

  ```bash
  docker run --rm \
    --publish 127.0.0.1:8080:8080 \
    --mount type=bind,source="$PWD",target=/repos,readonly \
    commitography:local \
    serve --listen 0.0.0.0:8080 --allowed-root /repos
  ```

- In Docker, the dashboard sees container paths such as `/repos/<name>`, not
  host paths. [`docs/docker.md`](docs/docker.md) covers Linux, macOS, Windows
  PowerShell and Git Bash.
- `make docker-image` builds the image locally; `make docker-smoke` tests it.
- Git's `safe.directory` setting moved to the system configuration, so the
  image also reads mounted repositories when run with `--user`.
- `ca-certificates` is no longer requested explicitly.
- Inside a container, a missing repository or allowed root prints a hint about
  the mount, and `serve` warns when an allowed root is empty.

### Ports and network behavior

- The dashboard listens on the loopback interface by default. Outside a
  container, a non-loopback `--listen` address prints a warning; inside one, a
  wildcard listener prints how to publish it on the host loopback.
- Requests must name the server as `localhost`, `127.0.0.1`, `[::1]` or the
  explicit listen address. Other host names are refused, which blocks DNS
  rebinding.
- The image declares no exposed port, so `docker run -P` publishes nothing.
  Publish the dashboard with `--publish 127.0.0.1:8080:8080` only.

### Limitations

- One analysis runs at a time. Jobs and reports live in memory and are lost
  when the server stops.
- Single user, local repositories only: no accounts, remote cloning,
  scheduling or persistent storage.
- A linked worktree cannot be analyzed from a container on its own, because it
  refers to its main repository by a host path.
- Analysis through a Docker Desktop mount is slower than native; skipping blame
  helps on large repositories.
- Path handling on macOS and Linux hosts has not been verified on those systems
  yet.
