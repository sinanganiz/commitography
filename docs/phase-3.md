# Phase 3 — Server Mode

**Status:** Planned. Scope defined, implementation not yet broken down.

**Prerequisite:** Phase 2 complete and in real use. Phase 3 is the largest increase in operational surface area in the project and should not begin while earlier phases are still changing shape.

**Phase 1.5 relationship:** Phase 1.5 is a single-user localhost runner, not a
server-mode implementation. It has no repository catalog, remote credentials,
scheduler, persistent reports or organization rollups. Phase 3 may reuse its
analysis and API contracts, but it must replace the in-memory job manager with
the persistence and operational controls defined below.

---

## 1. Purpose

Phases 1 and 2 analyze one repository at a time and produce a static artifact. Phase 3 addresses organizations that want a single always-available place showing many repositories, refreshed on a schedule, with an in-browser control to refresh on demand.

This is the mode that answers "our team has forty repositories on a self-hosted git server and wants one dashboard for all of them."

---

## 2. Scope

### In scope

- A long-running self-hosted service managing a configured set of repositories.
- Mirror clones maintained locally, refreshed periodically via a background scheduler.
- On-demand refresh triggered from the web interface.
- A repository index page and per-repository dashboards.
- Organization-level rollups across repositories, at repository granularity rather than individual granularity.
- Authentication for the web interface, with a small set of pluggable providers.
- Credential storage for accessing private repositories over HTTPS or SSH.
- A persistent store holding normalized history and computed reports across repositories.
- Deployment as a single container with a mounted data volume, plus a documented compose setup.
- A read-only HTTP API exposing the same data as the dashboards.

### Out of scope

- A multi-tenant hosted service operated by the project.
- Billing, licensing, or usage metering.
- Write access to any repository. The service remains strictly read-only.
- Cross-organization or cross-customer data sharing of any kind.
- Individual developer performance dashboards. The positioning stated in the project overview applies identically here, and organization-level rollup makes the temptation stronger rather than weaker.

---

## 3. Key Design Considerations

### Operational surface

This phase introduces, for the first time, a process that runs continuously, holds credentials, listens on a network port, and stores data on disk. Every one of those is a new class of responsibility: upgrades, backups, secrets handling, and vulnerability response. The value must clearly justify that cost before implementation starts.

A guiding constraint: the service must remain a thin orchestration layer around the Phase 1 engine. If server-specific analysis logic starts appearing, the design has gone wrong.

### Storage

Phase 1 needs no database. Phase 2 uses SQLite as a cache. Phase 3 genuinely needs persistence: many repositories, historical reports, scheduling state, and user accounts.

SQLite remains the preferred default because it preserves single-container deployment with no external dependency. Whether a server-backed database is ever supported as an option is an open question to be decided against real load, not assumed in advance.

### Credentials

Accessing private repositories requires storing tokens, app passwords, or SSH keys. This must be designed conservatively: encryption at rest, no credentials in logs, no credentials in the API surface, and clear documentation of exactly what access is required. Read-only credentials must be sufficient, and the documentation must say so explicitly for each supported host.

### Scheduling and load

Refreshing forty mirrors on a shared schedule creates a load spike. The scheduler needs staggering, concurrency limits, per-repository intervals, and backoff on failure. A failing repository must not block or degrade the others.

### On-demand refresh

The refresh control is the most obvious feature and the easiest to get wrong. It must be rate-limited, must not allow a user to trigger unbounded concurrent work, must show honest progress, and must degrade gracefully when a repository is unreachable.

### Multi-repository rollups

Aggregating across repositories raises the same positioning risk described in the project overview, amplified. Rollups compare repositories, not people. Any per-contributor view remains behind the same opt-in flag and must never be the default landing experience.

### Authentication

The service will often sit inside a corporate network. Requirements are likely to include an internal-only mode with no authentication, simple local accounts, and at least one standard single sign-on integration. Which of these ship first should be driven by what users actually ask for.

### Migration path

Existing users arrive from Phase 1 and Phase 2 with working configurations. Adding a repository to the server must not require rewriting configuration that already works locally. The same `.commitography.yml` semantics apply.

---

## 4. Deliverables

1. A server binary or subcommand running the scheduler and web interface.
2. Repository registration and management, configurable by file and by interface.
3. Mirror clone management with scheduled and on-demand refresh.
4. Persistent storage of history and reports across repositories.
5. Encrypted credential storage for private repository access.
6. Repository index, per-repository dashboards, and organization rollups.
7. Authentication with at least one non-trivial provider.
8. A documented read-only HTTP API.
9. A container image and a documented compose deployment with a mounted data volume.
10. An operations guide covering backup, upgrade, resource sizing, and troubleshooting.

---

## 5. Exit Criteria

Phase 3 is complete when an administrator can deploy a single container, register a set of repositories including private ones on a self-hosted git server, and have every dashboard refresh automatically on a schedule and on demand from the interface, with credentials stored safely, with one failing repository not affecting the rest, and with an operations guide sufficient for someone who did not build the system to run it.

---

## 6. Sequencing Note

Detailed task breakdown for Phase 3 is deliberately deferred, and more strongly than for Phase 2. This phase is worth building only if Phase 1 and Phase 2 adoption produces sustained demand for it. If most users are satisfied by a scheduled CI job publishing to Pages, the correct decision is to not build Phase 3 at all, and that outcome should be treated as a success rather than an abandoned plan.
