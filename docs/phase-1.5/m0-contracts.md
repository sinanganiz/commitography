# M0 - Contracts and Documentation

**Depends on:** Nothing. **Blocks:** M1-M6.

M0 records the product and API contract before code is moved. It must be
completed first so the local server does not accidentally grow into Phase 3.

| Package | Status |
|---|---|
| WP-0.1 Phase relationship and scope documents | Complete |
| WP-0.2 Job and API contract | Complete |
| WP-0.3 Security and path contract | Complete |
| WP-0.4 CLI, Docker and frontend contracts | Complete |

---

## WP-0.1 - Phase relationship and scope documents

### Deliverables

- Create `docs/phase-1.5.md`.
- Create `docs/phase-1.5-detailed.md`.
- Update `docs/project-overview.md` from three phases to four delivery stages.
- Add Phase 1.5 to the phase relationship text in `docs/phase-2.md`.
- State in `docs/phase-3.md` that Phase 1.5 is not the multi-repository server.

### Acceptance criteria

- A reader can determine that Phase 1.5 is local-only, single-user and
  non-persistent.
- Phase 2 is not described as a prerequisite for the local runner.
- Phase 1.5 is not described as satisfying any Phase 2 exit criterion.
- Phase 3 remains the owner of multi-repository server responsibilities.

## WP-0.2 - Job and API contract

### Deliverables

Document the request, response and status shapes for:

- `GET /api/v1/capabilities`
- `GET /api/v1/jobs`
- `POST /api/v1/jobs`
- `GET /api/v1/jobs/{id}`
- `GET /api/v1/jobs/{id}/report`
- `POST /api/v1/jobs/{id}/cancel`
- `DELETE /api/v1/jobs/{id}`

The following shapes are normative.

### Capabilities response

`GET /api/v1/capabilities` returns `200`:

```json
{
  "apiVersion": "v1",
  "reportSchemaVersion": 1,
  "maxRecentJobs": 10,
  "activeJobLimit": 1,
  "pollIntervalMilliseconds": 750,
  "supportsCancel": true,
  "supportsOpen": true
}
```

It must not return the absolute allowed-root paths. The UI only needs the
capability values above.

### Create request and response

`POST /api/v1/jobs` accepts:

```json
{
  "repoPath": "/repos/project",
  "options": {
    "noBlame": false,
    "perAuthor": false,
    "anonymize": false,
    "allowShallow": false,
    "countMerges": false,
    "since": "",
    "until": ""
  }
}
```

It returns `202`:

```json
{
  "id": "opaque-job-id",
  "status": "queued"
}
```

The server may add optional analysis fields later, but it must reject
`outputDir`, `configPath`, arbitrary environment variables and raw Git command
arguments.

### Job status response

`GET /api/v1/jobs/{id}` returns `200`:

```json
{
  "id": "opaque-job-id",
  "status": "running",
  "repoName": "project",
  "repoPath": "/repos/project",
  "createdAt": "2026-09-11T12:00:00Z",
  "startedAt": "2026-09-11T12:00:01Z",
  "finishedAt": null,
  "progress": {
    "sequence": 12,
    "stage": "collecting",
    "detail": "Reading commit history",
    "fraction": 0.42,
    "estimated": true,
    "current": 4200,
    "total": 10000
  },
  "warnings": [],
  "error": null
}
```

`repoPath` is returned only to the same local session that created the job. It
must not be copied into generic error logs or public diagnostics.

### Job list response

`GET /api/v1/jobs` returns `200`:

```json
{
  "jobs": [
    {
      "id": "opaque-job-id",
      "status": "succeeded",
      "repoName": "project",
      "createdAt": "2026-09-11T12:00:00Z",
      "startedAt": "2026-09-11T12:00:01Z",
      "finishedAt": "2026-09-11T12:00:20Z",
      "warningCount": 0
    }
  ]
}
```

The newest job appears first. The response contains at most ten entries.

### Report, cancel and delete responses

- `GET /api/v1/jobs/{id}/report` returns `200` with the complete
  `aggregate.Report` object only when status is `succeeded`.
- `POST /api/v1/jobs/{id}/cancel` returns `202` while cancellation is being
  processed, or `200` if the job is already `cancelled`. It is idempotent.
- `DELETE /api/v1/jobs/{id}` returns `204` for a terminal job.

### Error response

Every API error uses:

```json
{
  "error": {
    "code": "invalid_repository_path",
    "message": "The repository path is outside the allowed roots."
  }
}
```

The normative status mapping is:

| Status | Meaning |
|---:|---|
| `400` | Malformed JSON or invalid request values |
| `401` | Missing or invalid local session |
| `403` | Origin, host or allowed-root rejection |
| `404` | Unknown endpoint or job ID |
| `409` | Another job is active or the requested state transition is invalid |
| `413` | Request body exceeds the configured limit |
| `500` | Unexpected server failure |

### Acceptance criteria

- Each endpoint has its success status, validation errors and conflict behavior.
- The report body references the existing report schema instead of duplicating
  it.
- No endpoint accepts `outputDir`, arbitrary `configPath`, raw history or a
  filesystem listing request.
- Job status names and progress event fields match the index contract.

## WP-0.3 - Security and path contract

### Deliverables

Document:

- Loopback default and explicit listen behavior.
- Session cookie and Origin validation rules.
- Allowed-root canonicalization.
- Symlink, junction, UNC and linked-worktree handling.
- Raw data and repository mutation rules.

### Acceptance criteria

- A security reviewer can identify every filesystem read initiated by a job.
- A path outside every allowed root is rejected before `git` starts.
- No design requires CORS, public binding, user accounts or repository upload.

## WP-0.4 - CLI, Docker and frontend contracts

### Deliverables

Document:

- `commitography serve` flags and defaults.
- `--open` behavior.
- Existing Docker CLI default preservation.
- Explicit Docker server invocation and host/container path distinction.
- React/MUI bundle and static offline compatibility requirements.

### Acceptance criteria

- Existing CLI invocation examples remain valid.
- A Docker user can run the server without changing the image entrypoint.
- The frontend contract requires no CDN or runtime dependency outside the local API.
