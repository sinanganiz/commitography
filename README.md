# Commitography

Commitography analyses the commit history of a local git repository and
describes the repository. It is open source under the MIT License.

> [!IMPORTANT]
> **Project status: pre-release.** No version has been released, and no
> binary, package or container image is published.
>
> The code in this repository was written before the current decision set and
> does not yet conform to it. Its command-line and server behaviour will change,
> so it is not documented here. [`docs/work/audit.md`](docs/work/audit.md) lists
> where the code and the decision set differ.

---

## Requirements

| Requirement | Needed for |
|---|---|
| Go, at the version declared in [`go.mod`](go.mod) or later | Building and testing |
| `git` on `PATH` | Running the tests and any analysis |
| Node.js and `make` | Only for `make build`, which also rebuilds the frontend bundle |

The built frontend bundle is committed, so `go build` does not need Node.js.

---

## Build

**Linux and macOS**

```bash
git clone https://github.com/sinanganiz/commitography.git
cd commitography
go build -o commitography ./cmd/commitography
./commitography --version
```

**Windows (PowerShell)**

```powershell
git clone https://github.com/sinanganiz/commitography.git
cd commitography
go build -o commitography.exe ./cmd/commitography
.\commitography.exe --version
```

## Test

```bash
go test ./...
```

Tests that need fixture repositories skip when the fixtures are absent.

---

## Decisions and work

| Document | Status | Contents |
|---|---|---|
| [`docs/decisions/INDEX.md`](docs/decisions/INDEX.md) | **Binding** | Architecture decision records. Read this before changing anything. |
| [`docs/metrics.md`](docs/metrics.md) | **Binding** | The metric catalogue. |
| [`docs/conventions.md`](docs/conventions.md) | Guidance | Conventions that are not enforced by tooling. |
| [`docs/work/INDEX.md`](docs/work/INDEX.md) | Work | Numbered work packages. |
| [`docs/project-overview.md`](docs/project-overview.md) | Orientation | Where the rules live and what the decision set covers. |
| [`AGENTS.md`](AGENTS.md) | Instructions | How to work in this repository. |

---

## License

Commitography is released under the MIT License. See [LICENSE](LICENSE).
