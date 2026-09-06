# M4 — CLI Surface for CI

**Depends on:** M2. **Blocks:** M5, M6.
**Closes:** Phase 2 exit criteria 7, 8 and 9.

Everything a pipeline needs from the command line that Phase 1 did not provide: a digest it can post, a comparison against the last run, a way to fail a build, and an error message that names the fix for the CI system the user is actually on.

`--cache` and `--no-cache` are not here. They belong to [WP-2.6](m2-cache.md), because the cache cannot be tested without them.

| Package | Status |
|---|---|
| WP-4.1 `--summary` | ☐ |
| WP-4.2 `--baseline` and the comparison block | ☐ |
| WP-4.3 `--strict` and exit code 3 | ☐ |
| WP-4.4 CI environment detection | ☐ |
| WP-4.5 Output size budget and the per-author cap | ☐ |
| WP-4.6 README | ☐ |

---

## WP-4.1 — `--summary`

Create `internal/render/summary.go`.

```go
// WriteSummary emits a Markdown digest of a report. It is the only thing
// commitography ever writes to stdout.
func WriteSummary(w io.Writer, r *aggregate.Report) error
```

| Flag | Type | Default | Behaviour |
|---|---|---|---|
| `--summary` | bool | false | Write a Markdown digest to stdout |

Phase 1 deliberately left stdout empty by sending all progress to stderr. Markdown on stdout composes with `>> "$GITHUB_STEP_SUMMARY"`, with `> summary.md`, and with a pipe into a pull-request comment API, without the tool needing to know which. Writing to any of those destinations directly would make the feature GitHub-shaped, which is the one thing this product refuses to be.

### Normative output

Exactly these sections, in this order, with no others:

1. An H2 heading: the repository name and the analyzed period as `YYYY-MM-DD → YYYY-MM-DD`.
2. A two-column table of: commits analyzed, commits excluded, contributors, files touched, lines added, lines deleted, bus factor, conventional-commit ratio. When a comparison is present (WP-4.2), each numeric row gains a third column holding the signed delta.
3. `### Hotspots` — the top three entries of `mostTouchedFiles` as a list, each with its commit count.
4. `### Warnings` — every entry of `Report.Warnings` as a list. The section is omitted entirely when there are none.

### Normative rules

1. Output MUST be valid CommonMark and MUST NOT contain raw HTML.
2. Output MUST be under 8 KB for any input. Build annotations and pull-request comments are size-limited, and a digest that has to be truncated by its consumer is not a digest. The hotspot list is capped at three and the warning list at ten, with `and N more` when truncated.
3. Every path printed MUST be escaped so that a path containing backticks, pipes or underscores cannot break the table or the code spans.
4. `--summary` MUST NOT change what is written to the output directory. It adds stdout output and nothing else, and MUST compose with `--json`, `--wrapped` and `--quiet`.
5. `--quiet` MUST NOT suppress it. `--quiet` governs progress on stderr; suppressing requested output would make the flag combination useless in exactly the pipeline that wants both.

### Acceptance criteria

- stdout is byte-empty on every run without `--summary`, asserted across the existing CLI tests.
- With `--summary`, stdout contains the digest and stderr still contains the progress.
- A report with no warnings emits no `### Warnings` heading.
- A repository whose paths contain backticks and pipes produces a table that still parses.
- The digest stays under 8 KB on a report with 2,000 contributors and 50 warnings.

---

## WP-4.2 — `--baseline` and the comparison block

| Flag | Type | Default | Behaviour |
|---|---|---|---|
| `--baseline` | string | empty | Path to a previous `report.json` to compare against. Empty uses the report stored in the cache, when one is present |

### Report addition

`aggregate.Report` gains one optional field. Per decision D4, `schemaVersion` stays at `1`: Task 4.1 forbids removing or repurposing fields, not adding them.

```go
// Comparison is present when the run was compared against an earlier report.
type Comparison struct {
    BaselineGeneratedAt time.Time `json:"baselineGeneratedAt"`
    BaselineToolVersion string    `json:"baselineToolVersion"`
    CommitsDelta        int       `json:"commitsDelta"`
    ContributorsDelta   int       `json:"contributorsDelta"`
    AddedDelta          int       `json:"addedDelta"`
    DeletedDelta        int       `json:"deletedDelta"`
    BusFactorDelta      int       `json:"busFactorDelta"`
    NewHotspots         []string  `json:"newHotspots"`
    ResolvedHotspots    []string  `json:"resolvedHotspots"`
}
```

`docs/report-schema.json` MUST be updated in the same change, and the schema validation test MUST cover a report carrying a comparison and one without.

### Normative rules

1. The baseline MUST be rejected, with a warning and no comparison, when it describes a different repository. Sameness is decided by the first commit's hash, which is stable across clones, renames and moves. Repository name MUST NOT be used: renaming a directory would silently disable comparison, and two directories with the same name would silently enable a wrong one.
2. A baseline with a different `schemaVersion` MUST be rejected with a warning, not an error.
3. An unreadable or malformed explicit `--baseline` path MUST be a usage error, exit code 2. The user named a file; failing quietly would hide a typo. An unreadable baseline found *in the cache* MUST be a silent miss, because the user did not name it.
4. `NewHotspots` and `ResolvedHotspots` compare the top 25 `mostTouchedFiles` path sets, each capped at ten entries in the output.
5. A run with no baseline MUST omit the field entirely rather than emitting zeroes. Task 4.6's rule against placeholders for missing data applies here too.
6. The comparison MUST NOT alter any other computed value. It is a description of the difference between two reports, never an input to either.

### Acceptance criteria

- Two consecutive runs over a repository that gained commits produce correct deltas.
- A baseline from a different repository is rejected with a warning and no comparison.
- A missing cache baseline is silent; a missing explicit baseline exits 2.
- `report.json` with and without a comparison both validate against the schema.
- The dashboard renders correctly for both, with no empty comparison section when the field is absent.

---

## WP-4.3 — `--strict` and exit code 3

`README.md` already documents exit code 3 as "Reserved for `--strict`; not implemented". This package implements it.

| Flag | Type | Default | Behaviour |
|---|---|---|---|
| `--strict` | bool | false | Exit 3 when the report carries any warning |

### Normative rules

1. With `--strict`, the process MUST exit 3 if and only if `len(report.Warnings) > 0` after aggregation.
2. Exit 3 MUST happen **after** all output is written. The dashboard, `report.json`, the cache and the summary are all produced normally; the exit code reports on them rather than replacing them. A CI job that fails still needs the artifact that explains why.
3. Exit 3 MUST NOT be used for any other condition. Usage, configuration and repository errors stay at 2; internal errors stay at 1.
4. Warnings MUST be printed to stderr in strict mode even under `--quiet`, otherwise the exit code is unexplainable.
5. `--allow-shallow` raises a warning, so `--strict --allow-shallow` exits 3. This is correct and MUST NOT be special-cased: a pipeline that both accepts a truncated history and demands a clean run is asking for something contradictory, and the exit code should say so.

### Acceptance criteria

- A fixture producing warnings exits 3 under `--strict` and 0 without it.
- A fixture producing none exits 0 under `--strict`.
- Output files are present and complete after an exit-3 run.
- `--strict --quiet` prints the warnings that caused the failure.
- `README.md` no longer describes exit 3 as reserved.

---

## WP-4.4 — CI environment detection

Create `internal/cli/ci.go`.

```go
// Provider is a recognized CI system, or an empty string when the run does not
// appear to be in one.
type Provider string

// DetectProvider identifies the CI system from the environment.
func DetectProvider(env func(string) string) Provider

// ShallowFix returns the checkout setting that gives a provider a full clone.
func ShallowFix(p Provider) string
```

### Normative detection table

Tested in this order; first match wins.

| Provider | Condition | Fix |
|---|---|---|
| GitHub Actions | `GITHUB_ACTIONS` is `true` | `actions/checkout` with `fetch-depth: 0` |
| GitLab CI | `GITLAB_CI` is `true` | `GIT_DEPTH: 0` |
| Bitbucket Pipelines | `BITBUCKET_BUILD_NUMBER` is non-empty | `clone: depth: full` |
| Azure Pipelines | `TF_BUILD` is non-empty | `fetchDepth: 0` |
| Jenkins | `JENKINS_URL` is non-empty | Disable shallow clone in the Git SCM step |
| Generic | `CI` is non-empty and not `false` or `0` | — |

`DetectProvider` takes the environment as a function so that every case is testable without mutating the process environment.

### This modifies a message Phase 1 specified normatively

Phase 1 Task 1.2 fixes the shallow-clone error text exactly, and a test asserts it byte for byte. This package changes it, deliberately:

- **No provider detected:** the message is unchanged. The existing test MUST continue to pass unmodified for this case.
- **A specific provider detected:** the three-line list of CI systems is replaced by a single line naming that provider and its fix. Everything else — the opening sentence, the `git fetch --unshallow` block, the `--allow-shallow` line — stays word for word.
- **Generic CI detected:** the message is unchanged. Something is a CI system but we do not know which, so listing all of them is still the most useful thing to print.

The Phase 1 test MUST be extended with the per-provider cases rather than loosened, and the change MUST be recorded in [`phase-1-detailed.md`](../phase-1-detailed.md) alongside Task 1.2 so that document does not go stale.

### Additional behaviour

1. When a provider is detected, `--verbose` MUST log which one. A wrong detection is otherwise invisible.
2. Detection MUST NOT change any default. It affects the wording of one error message and nothing else. Silently turning on `--quiet` or `--no-blame` because a run looks like CI would make the tool behave differently in the place where reproducing behaviour matters most.

### Acceptance criteria

- One test per provider, driving `DetectProvider` with a synthesized environment.
- A shallow fixture under each synthesized environment produces that provider's fix line and exit code 2.
- With no CI variables set, the Phase 1 message is produced byte for byte.
- No default changes under any detected provider, asserted by comparing a full report produced with and without the variables set.

---

## WP-4.5 — Output size budget and the per-author cap

Decision D3. Measured sizes are 108 KB total at 2,584 commits, and most report arrays are already capped at top-N. The single unbounded section is `perAuthor`, which is opt-in and grows with contributor count.

### Normative rules

1. `PerAuthor` MUST be capped at **500 contributors**, ordered as Task 4.7 already specifies. When truncated, the section MUST carry `truncated: true` and `totalContributors: <n>` so the dashboard can say what it is not showing. Silently showing 500 of 5,000 would be a lie by omission.
2. The documented budget for `index.html` is **5 MB**. Exceeding it MUST produce a warning on stderr naming the actual size, and MUST NOT prevent the file being written. The number is a guardrail against a surprise, not a limit on what the user may have.
3. `report.json` has no separate budget. It is bounded by the same caps and is not served to browsers.
4. The budget MUST be a named constant with a comment stating where the number came from, so that raising it is a decision rather than a habit.

### Acceptance criteria

- A synthetic report with 5,000 contributors produces 500 entries, `truncated: true` and a correct total.
- The dashboard states the truncation rather than presenting the 500 as complete.
- A page over 5 MB warns and is still written.
- `docs/report-schema.json` covers the two new per-author fields.

---

## WP-4.6 — README

The flag table in `README.md` gains `--cache`, `--no-cache`, `--summary`, `--baseline` and `--strict`, and the exit-code table stops describing 3 as reserved.

### Normative rules

1. The convention in [`phase-2-detailed-work-packages.md`](../phase-2-detailed-work-packages.md) §9 requires each package to update the README as it lands. This package is the sweep that confirms the result is coherent, not the place where the documentation is first written.
2. A new section MUST describe what stdout carries and what stderr carries, in one short paragraph. Three flags now depend on that separation and it is currently stated only as a line under the flag table.
3. The CI integration page is M7's work. `README.md` MUST link to it and MUST NOT duplicate it.

### Acceptance criteria

- Every flag accepted by the binary appears in the README table, verified by a test that compares the cobra flag set against the table.
- The exit-code table matches the codes the binary actually returns.
