# Phase 1.5 - Detailed Work Packages

**Status:** Approved scope, implementation not started - 2026-09-11

**Scope:** [`phase-1.5.md`](phase-1.5.md)

This document is the index and contract for implementation. Work package files
under [`phase-1.5/`](phase-1.5/) contain the exact deliverables and acceptance
criteria. A package is not complete because code exists; its acceptance
criteria must be satisfied and its status checkbox must be updated in the same
change.

---

## 1. Approved Decisions

| Decision | Chosen value |
|---|---|
| Phase boundary | Phase 1.5, between the local CLI MVP and CI/CD integration |
| Server command | `commitography serve` in the existing binary |
| Native path input | Typed path, constrained by repeatable `--allowed-root` |
| Default allowed root | Current working directory when no root is supplied |
| Job concurrency | One active job; a second start returns `409 Conflict` |
| Job history | Ten most recent jobs in memory; no restart persistence |
| Progress | Estimated fraction plus structured stage and counters |
| Client updates | 500-1000 ms HTTP polling |
| Native bind | `127.0.0.1:8080` by default |
| Browser opening | Opt-in `--open` |
| Local protection | Process-scoped session cookie, Origin checks and security headers |
| Frontend | Full migration to React |
| UI components | MUI with a custom Commitography theme |
| Visualizations | Existing custom SVG approach retained |
| Cache | Not required by Phase 1.5; optional integration point for Phase 2 |
| Repository mutation | A changed repository produces `stale`, not a successful report |
| Docker | Existing image with explicit `serve`; CLI remains the default CMD |

These decisions are binding unless this index is changed before the affected
work package starts.

---

## 2. Non-Negotiable Invariants

1. The CLI remains a supported first-class interface.
2. The static output remains a single self-contained HTML file with no runtime
   network dependency.
3. The server never exposes raw `model.History` through HTTP.
4. The server never writes to a repository-selected `output_dir`.
5. The analysis engine does not know whether its caller is CLI or HTTP.
6. MUI assets are bundled locally; no CDN or external font is introduced.
7. Native server mode is not a public network service.
8. Phase 1.5 does not introduce a persistent database.
9. Per-author metrics remain opt-in and privacy defaults remain unchanged.
10. A cache miss or absent cache never changes correctness.

---

## 3. Milestones and Dependency Order

| Milestone | File | Depends on | Packages | Status |
|---|---|---|---:|---|
| M0 - Contracts and documentation | `phase-1.5/m0-contracts.md` | - | 4 | Complete |
| M1 - Shared analysis engine | `phase-1.5/m1-analysis-engine.md` | M0 | 7 | In progress |
| M2 - Job manager and HTTP server | `phase-1.5/m2-job-server.md` | M1 | 7 | Not started |
| M3 - API and local security | `phase-1.5/m3-api-security.md` | M2 | 7 | Not started |
| M4 - React and MUI application | `phase-1.5/m4-react-frontend.md` | M0, M3 | 8 | Not started |
| M5 - Docker and distribution | `phase-1.5/m5-docker.md` | M2, M4 | 5 | Not started |
| M6 - Verification and release documentation | `phase-1.5/m6-verification.md` | M1-M5 | 8 | Not started |
| **Total** |  |  | **46** |  |

M0 must land before implementation because it defines the API and security
contract. M1 must preserve the existing CLI before M2 can run asynchronous
jobs. M3 defines the boundary consumed by the React application. M5 is last in
the implementation chain because the image must contain the final server and
frontend assets. M6 is the release gate.

---

## 4. Cross-Cutting Technical Contract

### Analysis service

The shared package must expose an output-free, context-aware operation with
these conceptual values:

```go
type Options struct {
    RepoPath     string
    ConfigPath   string
    Since        string
    Until        string
    PerAuthor    bool
    Anonymize    bool
    NoBlame      bool
    AllowShallow bool
    CountMerges  bool
}

type ProgressEvent struct {
    Sequence  uint64
    Stage     string
    Detail    string
    Fraction  *float64
    Current   int
    Total     int
    Estimated bool
}

type ProgressSink func(ProgressEvent)

func Run(ctx context.Context, opts Options, sink ProgressSink) (*aggregate.Report, error)
```

The exact package path and additional diagnostic fields may be refined during
M1, but the operation must not render HTML, write report files or write to a
global warning sink.

### HTTP job contract

The API uses opaque IDs, JSON request/response bodies and report schema version
1. A create request contains a repository path and analysis options, but never
an output directory or arbitrary config path. Status responses expose the most
recent progress event, warning count, error kind and timestamps.

### Progress contract

`Fraction` is nullable. A non-null value is in the inclusive range `[0, 1]`
and is an estimate unless the event explicitly describes a counted operation.
The UI must never present an estimate as exact completion. Sequence numbers are
monotonic per job and make polling responses idempotent.

### Job contract

The manager has one active worker. `Start` returns a conflict when another
worker is active. Cancellation is cooperative and must terminate child Git
processes through context cancellation. A job is retained after completion
until the ten-item limit evicts it or the user deletes it.

### Path contract

Every requested path is made absolute, cleaned, symlink-resolved where
possible, and compared against canonical allowed roots using platform-aware
rules. The implementation must protect against Windows drive, UNC, junction,
case-folding and separator edge cases, as well as Unix symlink escapes.

---

## 5. Status Rules

- `Not started` means no acceptance criteria have been met as a package.
- `In progress` means code or documentation is actively being changed.
- `Blocked` means a dependency or platform is unavailable; the reason must be
  written beside the status.
- `Complete` means all package acceptance criteria and required tests pass.
- Manual verification must name the OS, tool version, repository shape and date.

The milestone files are the detailed status source. This index carries only the
roll-up and must be updated with them.

---

## 6. Open Implementation Constraints

These are implementation tasks, not unresolved product decisions:

- Pin compatible React, React DOM, MUI and Emotion versions in `web/package-lock.json`.
- Keep the Vite output as an embeddable IIFE and CSS asset.
- Decide the precise progress weights from measured stage duration, then record
  them in M1 rather than pretending they are universal truths.
- Use `httptest` for API behavior and a browser harness for the end-to-end UI.
- Keep the existing Go dependency policy unless a dependency is explicitly
  approved in a package update.
- Record any change to the Phase 1 report schema separately; Phase 1.5 should
  not require a schema version bump.
