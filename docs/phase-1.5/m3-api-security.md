# M3 - API and Local Security

**Depends on:** M2. **Blocks:** M4, M5, M6.

M3 makes the local server safe by default. Localhost is a deployment boundary,
not an excuse to expose arbitrary filesystem access without validation.

| Package | Status |
|---|---|
| WP-3.1 API resource schemas and routing | Complete |
| WP-3.2 Job creation and validation | Complete |
| WP-3.3 Status, report, cancel and delete handlers | Complete |
| WP-3.4 Session and request protection | Not started |
| WP-3.5 Allowed-root and symlink enforcement | Not started |
| WP-3.6 Response safety and data minimization | Not started |
| WP-3.7 API and security test matrix | Not started |

---

## WP-3.1 - API resource schemas and routing

### Deliverables

Implement `/api/v1/` routing and document JSON shapes for capabilities, job
list, job creation, job status, report, cancellation and deletion.

### Acceptance criteria

- JSON content types are explicit.
- Unsupported methods return a structured error.
- Unknown job IDs return `404` without revealing whether another ID exists.
- The report response is the existing report object, not a second report model.
- API versioning is isolated from `report.schemaVersion`.

## WP-3.2 - Job creation and validation

### Deliverables

Accept a request shaped like:

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

Reject unknown filesystem-control fields such as `outputDir` and `configPath`.

### Acceptance criteria

- Malformed JSON, oversized bodies and invalid options return `400`.
- A path outside the allowed roots is rejected before a worker is created.
- A valid request returns `202` and an opaque ID.
- A second valid request while active returns `409`.
- `perAuthor` and `anonymize` behavior reaches the shared analysis service.

## WP-3.3 - Status, report, cancel and delete handlers

### Deliverables

Implement all lifecycle endpoints and their terminal-state behavior.

### Acceptance criteria

- Status polling is safe to repeat.
- Report access is rejected for non-successful jobs.
- Cancel is idempotent and only affects the identified job.
- Delete removes the in-memory report and metadata.
- Deleting an active job is rejected; cancellation must happen first.

## WP-3.4 - Session and request protection

### Deliverables

Create a process-scoped random session secret and set an HttpOnly,
SameSite=Strict cookie for the application. Validate Origin and host rules for
state-changing requests.

### Acceptance criteria

- Requests without the session cookie are rejected except for the initial UI
  bootstrap and capability negotiation required to receive it.
- Cross-origin state-changing requests are rejected.
- Cookies do not contain repository paths or job IDs.
- No wildcard CORS header is emitted.
- Restarting the process invalidates the old session.

## WP-3.5 - Allowed-root and symlink enforcement

### Deliverables

Implement platform-aware canonical path validation for Windows, macOS and
Linux. Handle symlinks, junctions, UNC paths, drive casing and linked Git
directories.

### Acceptance criteria

- A path with a lexical prefix but a resolved target outside the root is
  rejected.
- A Git worktree whose real Git directory is outside the allowed roots follows
  a documented and tested rule.
- `..`, alternate separators and case variants cannot bypass the root check.
- No directory listing endpoint is added as a workaround.

## WP-3.6 - Response safety and data minimization

### Deliverables

Add response headers and sanitize API diagnostics. Ensure raw history, cache,
environment variables and arbitrary file contents never appear in responses.

### Acceptance criteria

- Responses include `Cache-Control: no-store`, `X-Content-Type-Options:
  nosniff`, `Referrer-Policy: no-referrer` and a restrictive CSP.
- Error responses contain a stable error code and safe message.
- Git stderr is not copied verbatim into browser-visible errors.
- Repository path display is limited to what the local UI needs.

## WP-3.7 - API and security test matrix

### Deliverables

Add `httptest` coverage for every endpoint, session behavior, conflict behavior,
path traversal and response headers.

### Acceptance criteria

- Tests cover valid and invalid JSON.
- Tests cover missing and invalid session cookies.
- Tests cover cross-origin state changes.
- Tests cover path escape attempts on the host platform.
- Tests prove that raw history and arbitrary file reads are unavailable.
