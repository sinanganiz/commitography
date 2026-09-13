# Running Commitography in Docker

The image runs the same `commitography` binary in two modes:

- **CLI mode**, the default, writes a static dashboard for the repository
  mounted at `/repo`.
- **Server mode** runs the local web dashboard when you override the command
  with `serve`. It is never started unless you ask for it.

> **Availability.** The local web dashboard is not part of a tagged release
> yet, and no published image contains `serve`. Until one does, build the image
> from this repository as described in [Building the image
> locally](#building-the-image-locally). The examples use that local tag,
> `commitography:local`.

---

## Host paths and container paths

A container cannot see your files until you mount them, and it only knows them
by the path you mount them at. Everything the container reads, including every
path you type into the dashboard, is a **container path**.

| Host | Repository on the host | Mounted with `source=` | Path you type in the dashboard |
|---|---|---|---|
| Linux | `/home/ana/src/api` | `/home/ana/src` | `/repos/api` |
| macOS | `/Users/ana/src/api` | `/Users/ana/src` | `/repos/api` |
| Windows | `C:\Users\ana\src\api` | `C:\Users\ana\src` | `/repos/api` |

Mount a folder that contains your repositories at `/repos` and every
repository below it becomes `/repos/<name>`. A host path such as
`C:\Users\ana\src\api` or `/Users/ana/src/api` is never valid in the
dashboard; it answers *Nothing was found at this path*.

---

## Server mode

The examples below make the current directory's repositories available
read-only and publish the dashboard on the host loopback only. Open
<http://127.0.0.1:8080> afterwards.

**Linux and macOS (bash, zsh)**

```bash
docker run --rm \
  --publish 127.0.0.1:8080:8080 \
  --mount type=bind,source="$PWD",target=/repos,readonly \
  commitography:local \
  serve --listen 0.0.0.0:8080 --allowed-root /repos
```

**Windows PowerShell**

```powershell
docker run --rm `
  --publish 127.0.0.1:8080:8080 `
  --mount "type=bind,source=$PWD,target=/repos,readonly" `
  commitography:local `
  serve --listen 0.0.0.0:8080 --allowed-root /repos
```

**Windows Git Bash**

Git Bash rewrites arguments that look like POSIX paths, which turns `/repos`
into a path under the Git installation. Turn that off for the command and pass
the source as a Windows path:

```bash
MSYS_NO_PATHCONV=1 docker run --rm \
  --publish 127.0.0.1:8080:8080 \
  --mount type=bind,source="$(pwd -W)",target=/repos,readonly \
  commitography:local \
  serve --listen 0.0.0.0:8080 --allowed-root /repos
```

When the current directory is itself a repository, type `/repos` in the
dashboard. When it holds several repositories, type `/repos/<name>`.

### Why the flags look the way they do

- `--publish 127.0.0.1:8080:8080` makes the dashboard reachable from this
  computer only. Never use `--publish 8080:8080` or `-P`: both publish the port
  on every host interface, where other machines on your network can reach it.
  To use another host port, change only the first number, for example
  `127.0.0.1:9000:8080`, and open <http://127.0.0.1:9000>.
- `--listen 0.0.0.0:8080` is required inside the container, because a
  published port arrives on the container's network interface, not on its
  loopback. The host-side loopback binding above is what keeps the dashboard
  private. If you leave `--listen` out, the server prints a note explaining
  why it cannot be reached.
- `readonly` is safe because the server never writes to repositories. The
  report stays in the server's memory.
- `--allowed-root /repos` limits the dashboard to the mounted folder. A path
  outside it is refused.

Stop the server with `Ctrl+C`, or with `docker stop` when it runs detached.

---

## CLI mode

With no command, the image analyzes `/repo` and writes the dashboard to
`/repo/out`. To keep the repository read-only, mount a separate output folder
and point `-o` at it. `--mount` requires the output folder to exist, so create
it first.

**Linux and macOS (bash, zsh)**

```bash
mkdir -p out
docker run --rm \
  --mount type=bind,source="$PWD",target=/repo,readonly \
  --mount type=bind,source="$PWD/out",target=/out \
  commitography:local /repo -o /out
```

**Windows PowerShell**

```powershell
New-Item -ItemType Directory -Force out | Out-Null
docker run --rm `
  --mount "type=bind,source=$PWD,target=/repo,readonly" `
  --mount "type=bind,source=$PWD\out,target=/out" `
  commitography:local /repo -o /out
```

**Windows Git Bash**

```bash
mkdir -p out
MSYS_NO_PATHCONV=1 docker run --rm \
  --mount type=bind,source="$(pwd -W)",target=/repo,readonly \
  --mount type=bind,source="$(pwd -W)/out",target=/out \
  commitography:local /repo -o /out
```

The dashboard is written to `out/index.html`. Any flag from the native CLI can
follow the path, for example `/repo -o /out --no-blame`.

On Linux, files the container writes belong to root. Add
`--user "$(id -u):$(id -g)"` after `docker run --rm` to write them as your own
user; Git inside the image still reads the mounted repository.

---

## Docker Desktop file sharing

Docker Desktop runs containers in a virtual machine, so it can only mount host
folders it shares with that machine.

- **macOS:** `/Users`, `/Volumes`, `/private`, `/tmp` and `/var/folders` are
  shared by default. A repository anywhere else must be added under
  *Settings → Resources → File sharing*.
- **Windows with the WSL 2 backend:** Windows drives are available without any
  setting. A repository stored inside a WSL distribution is faster to analyze
  from that distribution's shell, using its Linux path as the source.
- **Windows with the Hyper-V backend:** the drive or folder must be listed under
  *Settings → Resources → File sharing*.
- **Linux with Docker Engine:** no sharing step exists. On hosts that enforce
  SELinux, bind mounts need a relabel option, which `--mount` cannot express;
  use `-v "$PWD:/repos:ro,z"` there instead.

Docker Desktop does not reject a mount source that does not exist. It creates an
empty folder in its place, and the container then reports that the path is not
a Git repository. If a path that should work is refused, check the spelling of
`source=` first.

---

## Worktrees and submodules

- **Linked worktrees** refer to their main repository by an absolute host path
  written in their `.git` file. That path does not exist inside the container,
  so a worktree mounted on its own is not a readable repository. Analyze the
  main repository instead. On Linux and macOS you can also mount the folder at
  the same path it has on the host, for example
  `--mount type=bind,source=/home/ana/src,target=/home/ana/src,readonly` with
  `--allowed-root /home/ana/src`, and type the host path. This is not possible
  for Windows drive paths. The server refuses a worktree whose Git directory
  lies outside the allowed root in either case.
- **Submodules** refer to the Git directory inside their superproject by a
  relative path. Mount the superproject, or a folder that contains it, rather
  than the submodule folder alone.

---

## Performance

Reading files through a Docker Desktop mount is slower than reading the host
disk, and blame is the part of an analysis that reads the most. For a large
repository, tick *Skip blame* in the dashboard's advanced options, or pass
`--no-blame` in CLI mode. Line ownership and knowledge concentration are left
out of that report. Running the native binary avoids the overhead entirely.

---

## Troubleshooting

| Symptom | Cause and fix |
|---|---|
| The browser cannot connect to <http://127.0.0.1:8080>. | `--listen 0.0.0.0:8080` is missing, or the port is published differently. The container log shows how the server was started. |
| *Nothing was found at this path.* | A host path was typed. Type the container path, such as `/repos/api`. |
| *This path is outside the folders the server may read.* | The path is not below `--allowed-root`. Mount the folder that contains the repository and allow that mount point. |
| *This folder is not a Git repository* for a path that looks right. | The `source=` folder is misspelled, so Docker Desktop mounted an empty folder, or the folder is not shared with Docker Desktop. |
| In Git Bash, `/repos` turns into `C:/Program Files/Git/repos`. | Set `MSYS_NO_PATHCONV=1` for the command. |
| CLI mode fails with `read-only file system`. | The default output `/repo/out` is inside a read-only mount. Mount an output folder and pass `-o /out`. |
| The page answers *request host is not allowed*. | The dashboard was opened through a name other than `127.0.0.1` or `localhost`. Only those names are accepted, which stops other websites from reaching the dashboard through DNS rebinding. |

---

## Building the image locally

The `Dockerfile` copies a prebuilt Linux binary from its build context. Releases
supply that binary through goreleaser; for a local image, this command builds it
for the architecture of your Docker daemon and tags the result
`commitography:local`:

```bash
make docker-image
```

Without `make`, run the same steps from the repository root.

**bash, zsh or Git Bash**

```bash
rm -rf dist/docker && mkdir -p dist/docker
CGO_ENABLED=0 GOOS=linux GOARCH="$(docker version --format '{{.Server.Arch}}')" \
  go build -trimpath -o dist/docker/commitography ./cmd/commitography
cp Dockerfile dist/docker/Dockerfile
docker build -t commitography:local dist/docker
```

**Windows PowerShell**

```powershell
Remove-Item -Recurse -Force dist\docker -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force dist\docker | Out-Null
$env:CGO_ENABLED = '0'; $env:GOOS = 'linux'; $env:GOARCH = docker version --format '{{.Server.Arch}}'
go build -trimpath -o dist\docker\commitography ./cmd/commitography
Remove-Item Env:CGO_ENABLED, Env:GOOS, Env:GOARCH
Copy-Item Dockerfile dist\docker\Dockerfile
docker build -t commitography:local dist\docker
```

The PowerShell variant removes the three variables again, because they would
otherwise make every later `go build` in that window produce a Linux binary.
