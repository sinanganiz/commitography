# M2 — Cache Engine

**Depends on:** M0. **Blocks:** M3, M4, M8.
**Closes:** Phase 2 exit criteria 3, 4 and 5.

M2 makes a repeat run read only the commits git has not already been asked about. The measurements in [`phase-2-detailed-work-packages.md`](../phase-2-detailed-work-packages.md) §5 establish why this is worth building: a blame-less run costs exactly what `git log --numstat` costs, so removing the re-read removes essentially the whole cost.

The design is deliberately small. There is no database (decision B1), no new dependency (E1), and no new serialization format: the cache stores the `model.History` artifact that [`internal/collect/writer.go`](../../internal/collect/writer.go) already writes and reads, next to a manifest describing when it is still valid.

**The governing rule, from [`phase-2.md`](../phase-2.md) §3, applies to every package in this milestone:** the cache is a performance optimization and never a source of truth. Any inconsistency MUST cause a full re-read rather than a partial or silently wrong result. Deleting the cache directory at any moment MUST change nothing but runtime.

| Package | Status |
|---|---|
| WP-2.1 Cache package, layout and manifest | ☐ |
| WP-2.2 Fingerprints | ☐ |
| WP-2.3 Load and validate | ☐ |
| WP-2.4 Incremental collection | ☐ |
| WP-2.5 Store | ☐ |
| WP-2.6 Wire into the run, `--cache` and `--no-cache` | ☐ |
| WP-2.7 Test matrix | ☐ |

---

## WP-2.1 — Cache package, layout and manifest

Create `internal/cache/cache.go`.

### Directory layout

A cache is a directory holding four files. Three are written by this milestone; `blame.json` is written by M3 and MUST be tolerated as absent.

```
<cache dir>/
├── manifest.json     # what this cache holds and when it is valid
├── history.json      # model.History, written with collect.WriteHistory
├── report.json       # the previous run's report, the default --baseline source
└── blame.json        # M3; absent until then
```

### Location

```go
// Dir returns the cache directory for a repository. An explicit directory is
// used verbatim; otherwise the cache lives under the user's cache directory,
// keyed by the repository's absolute path.
func Dir(explicit, repoPath string) (string, error)
```

**Normative rules**

1. A non-empty `explicit` is returned unchanged after `filepath.Clean`.
2. Otherwise the directory is `<os.UserCacheDir()>/commitography/<key>`, where `key` is the first 16 hex characters of the SHA-256 of the repository's absolute, `filepath.Clean`-ed path. On Windows the path MUST be lowercased before hashing, so that two spellings of the same path share one cache; elsewhere it MUST NOT be, because paths are case-sensitive there.
3. If `os.UserCacheDir` fails, `Dir` MUST return an error. The caller treats that as "no cache available" and proceeds without one; it MUST NOT be fatal.
4. The cache directory MUST NOT be placed inside the analyzed repository under any circumstance, including when the user passes `--cache` pointing there. If `explicit` resolves to a path inside the analyzed repository's working tree, `Dir` MUST return an error naming the problem. Never writing to the repository under analysis is a load-bearing claim of this tool, and a cache is not an exception to it.

### Manifest

```go
// FormatVersion is the version of the cache layout and of the commit records
// inside it.
//
// It MUST be incremented whenever the parsing in internal/collect/gitlog.go
// changes, whenever model.Commit or model.FileChange gains, loses or
// reinterprets a field, or whenever the meaning of any manifest field changes.
// Forgetting to increment it is the one way this cache can serve wrong data,
// so any change to those files is required to consider it.
const FormatVersion = 1

// Manifest records what a cache directory holds and the conditions under which
// its contents remain valid.
type Manifest struct {
    FormatVersion        int       `json:"formatVersion"`
    HistorySchemaVersion int       `json:"historySchemaVersion"`
    HistoryFingerprint   string    `json:"historyFingerprint"`
    ReportFingerprint    string    `json:"reportFingerprint"`
    RepoPath             string    `json:"repoPath"`
    CommitCount          int       `json:"commitCount"`
    HistoryBytes         int64     `json:"historyBytes"`
    HasReport            bool      `json:"hasReport"`
    ToolVersion          string    `json:"toolVersion"`
    UpdatedAt            time.Time `json:"updatedAt"`
}
```

`RepoPath` is diagnostic only: it makes a cache directory identifiable by a human reading it, and MUST NOT participate in validation. A repository that moved is still the same repository.

`ToolVersion` is likewise diagnostic. It MUST NOT invalidate the cache — a patch release that changes nothing about collection would otherwise discard every cache in existence. `FormatVersion` is the field that carries that responsibility, which is why its doc comment states the obligation so plainly.

### Acceptance criteria

- `Dir("", repo)` returns a stable path across runs for the same repository, and different paths for two different repositories.
- On Windows, `Dir("", "C:\\Repo")` and `Dir("", "c:\\repo")` return the same path.
- `Dir(insideTheRepo, repo)` returns an error.
- A `Manifest` round-trips through JSON unchanged.

---

## WP-2.2 — Fingerprints

Create `internal/cache/fingerprint.go`.

Decision B5 requires that any configuration change which could alter a cached artifact invalidates it. Detailed design splits that into one fingerprint per artifact, because the two artifacts do not depend on the same inputs. This is a refinement of B5, not a relaxation: nothing that can change an artifact is omitted from that artifact's fingerprint. It is recorded as departure 3 in [`phase-2-detailed-work-packages.md`](../phase-2-detailed-work-packages.md) §7.

### What actually affects each artifact

Collection stores every commit and every file change unfiltered; exclusion, identity resolution, merge handling and bulk detection all happen afterwards in `internal/filter` and `internal/aggregate`, which re-run on every invocation regardless of the cache. So `history.json` depends on very little, and `report.json` depends on everything.

```go
// HistoryFingerprint covers every input that can change a collected commit
// record. Anything that only affects filtering or aggregation is deliberately
// absent: those stages re-run on every invocation, so a cached commit cannot
// go stale because an exclusion pattern changed.
func HistoryFingerprint(repoPath string, opts collect.Options) (string, error)

// ReportFingerprint covers the history fingerprint plus every input that can
// change a computed report.
func ReportFingerprint(historyFingerprint string, cfg config.Config, perAuthor, noBlame bool, year int) (string, error)
```

### Normative composition

`HistoryFingerprint` is the hex SHA-256 of these fields, each on its own line, in this order, with no other content:

```
formatVersion=<cache.FormatVersion>
historySchemaVersion=<model.SchemaVersion>
useMailmap=<true|false>
since=<opts.Since>
until=<opts.Until>
mailmap=<hash>
```

- `mailmap` is the hex SHA-256 of the contents of `.mailmap` in the analyzed repository root when `UseMailmap` is true and the file exists; the literal `absent` when it does not exist; and the literal `off` when `UseMailmap` is false. Git reads `.mailmap` only when asked to, so its content cannot affect a collection that did not ask.

`ReportFingerprint` is the hex SHA-256 of:

```
formatVersion=<cache.FormatVersion>
history=<historyFingerprint>
config=<json>
perAuthor=<true|false>
noBlame=<true|false>
year=<n>
gitattributes=<hash>
```

- `config` is `json.Marshal(cfg)` of the fully resolved `config.Config`. That type contains no maps, so Go's field-order marshaling is deterministic. Slice order MUST be preserved rather than sorted: `Identities` is order-sensitive, since the first email in each entry is the canonical identity ID.
- `gitattributes` is the hex SHA-256 of the root `.gitattributes`, or the literal `absent`.

### Normative rules

1. Both functions MUST be pure with respect to time, environment and working directory. Two invocations with the same inputs MUST produce the same string.
2. A file that cannot be read for a reason other than absence MUST produce an error, not the `absent` marker. Treating an unreadable `.mailmap` as absent would silently reuse a cache that a readable `.mailmap` would have invalidated.
3. Adding a field to either fingerprint MUST bump `FormatVersion` in the same change, because existing caches computed their fingerprint without it.

### Acceptance criteria

- Changing `exclude_paths` changes the report fingerprint and leaves the history fingerprint untouched.
- Changing `use_mailmap`, `--since`, `--until`, or the contents of `.mailmap` changes the history fingerprint.
- Editing `.mailmap` while `use_mailmap: false` changes neither fingerprint.
- Reordering entries in `identities` changes the report fingerprint.
- An unreadable `.mailmap` produces an error rather than a fingerprint.

---

## WP-2.3 — Load and validate

Add to `internal/cache/cache.go`.

```go
// Entry is a validated cache, ready to be used for an incremental read.
type Entry struct {
    Dir      string
    Manifest Manifest
    History  *model.History
}

// Load reads and validates the cache in dir. A cache that is absent, damaged,
// or no longer applicable is not an error: Load returns (nil, nil) and the
// caller performs a full read. An error is returned only for a condition the
// caller should know about, such as an unreadable directory.
func Load(dir, historyFingerprint string) (*Entry, error)
```

### Normative invalidation triggers

`Load` MUST return `(nil, nil)` when any of these holds. Each MUST be logged at verbose level with the specific reason, so that a user asking "why is this still slow" gets an answer.

1. `manifest.json` is absent, unreadable, or fails to decode.
2. `manifest.FormatVersion != FormatVersion`.
3. `manifest.HistorySchemaVersion != model.SchemaVersion`.
4. `manifest.HistoryFingerprint != historyFingerprint`.
5. `history.json` is absent or fails to decode.
6. The size of `history.json` on disk differs from `manifest.HistoryBytes`.
7. `len(history.Commits) != manifest.CommitCount`.
8. `collect.ReadHistory` rejects the artifact's schema version.

### Why size and count rather than a content hash

Triggers 6 and 7 exist to catch a torn or interleaved write: two concurrent runs on the same repository write the four files without a lock, so a manifest can in principle end up describing a different history than the one on disk. A truncated JSON file also fails to decode, which trigger 5 already catches; size and count catch the remaining case, where a complete but *different* file is present.

A SHA-256 over the history file would be strictly stronger and MUST NOT be added. On a large repository that file runs to hundreds of megabytes, and hashing it on every run would cost about as much as the parse it is protecting — spending the saving to insure against a race whose worst outcome is one unnecessary re-read.

### Concurrency

No lock file is used. Every individual file is written by atomic rename, so no reader ever observes a partial file. Two simultaneous runs can leave a mismatched set, which the integrity triggers detect and resolve by re-reading. A lost update costs one re-read. This MUST be documented in `docs/ci.md` by M7, because a CI matrix running several jobs against one cache path is a realistic way to hit it.

### Acceptance criteria

- One automated test per trigger, each asserting `(nil, nil)` and a distinct verbose reason.
- A valid cache loads and its `History` matches what was stored.
- Corrupting one byte in the middle of `history.json` invalidates rather than panicking.
- Truncating `history.json` invalidates rather than panicking.

---

## WP-2.4 — Incremental collection

Create `internal/collect/incremental.go`.

```go
// Reuse is what an incremental read did, for reporting and for tests.
type Reuse struct {
    Reused  int // commits taken from the cache
    Read    int // commits read from git in this run
    Dropped int // cached commits no longer reachable, discarded
}

// CollectIncremental reads history, taking from cached whatever git no longer
// needs to be asked about. A nil cached history makes this a full read.
func CollectIncremental(opts Options, cached *model.History) (*model.History, Reuse, error)
```

### Normative algorithm

1. Run `Preflight`. Its result is used fresh: `HeadCommit` and `DefaultBranch` change between runs and MUST NOT be taken from the cache.
2. Run `revList(opts)` to obtain the ordered hash list. **If `revList` fails, discard the cache entirely and fall through to `collectStream(opts, nil)`**, matching the existing fallback in `readHistory` for repository shapes `rev-list` cannot enumerate.
3. Build `cachedByHash` from `cached.Commits`.
4. Partition the rev-list output:
   - `missing` — hashes absent from `cachedByHash`, in rev-list order.
   - `Reuse.Reused` — the count present in both.
   - `Reuse.Dropped` — cached hashes absent from the rev-list output. These are discarded.
5. If `missing` is empty, no `git log` process is started at all. This is the case exit criterion 3 measures.
6. Otherwise read exactly `missing`, choosing between `collectStream(opts, missing)` and `collectSharded(opts, missing, shardCount(len(missing)))` by the same rule `readHistory` already applies. Both take a hash list on stdin with `--no-walk --stdin`, so this needs no new git plumbing.
7. Assemble the result by walking the rev-list output in order and taking each commit from the newly read set or from `cachedByHash`. The output order is therefore identical to a full read's by construction.
8. If any hash in the rev-list output is present in neither set after step 6, return an error. The caller MUST respond by discarding the cache and performing a full read; it MUST NOT return a short history.

### Dropped commits are discarded, not fatal

Step 4 discards cached commits that are no longer reachable instead of invalidating the whole cache. [`phase-2.md`](../phase-2.md) §3 originally listed dropped branches as a full-invalidation trigger; departure 2 in [`phase-2-detailed-work-packages.md`](../phase-2-detailed-work-packages.md) §7 refines that, and WP-0.2 writes the refinement into the scope document.

The justification is that git object names are content-addressed. A commit still present in the rev-list output has byte-identical content to when it was cached, so the retained subset cannot be stale. Rewritten history is still handled correctly and by the same mechanism: rewritten commits have different hashes, so the old ones vanish from the rev-list output and are dropped, and the new ones are absent from the cache and are read.

`Reuse.Dropped` MUST be reported as a verbose-level message naming the count, because a large drop is a useful signal that history was rewritten.

### Parse failures

The `maxParseFailureRatio` check in `Collect` MUST be applied to the commits read **in this run** only. Cached commits parsed successfully in an earlier run; folding them into the denominator would let a run that failed to parse everything it read look acceptable because a previous run went well.

Warnings raised during an earlier run MUST NOT be replayed from the cache.

### Ordering guarantee

Step 7 depends on `git rev-list --all --date-order` and `git log --all --date-order` producing the same sequence. That equivalence is not new: the sharded reader added in Phase 1 already relies on it, and `TestShardedReadMatchesSingleStream` asserts it across four fixtures. WP-2.7 extends that test rather than duplicating its reasoning.

### Acceptance criteria

- With a cache covering the full history and no new commits, no `git log` process is started, and the returned history equals a full read's, field by field.
- With a cache covering all but the newest N commits, exactly N commits are read and the result equals a full read's, in the same order.
- With a cache whose commits are entirely absent from the repository, everything is dropped and the result equals a full read's.
- A cache from a repository whose most recent commits were rewritten produces the same result as a full read.
- `Reuse` counts are correct in each case.

---

## WP-2.5 — Store

Add to `internal/cache/cache.go`.

```go
// Store writes a history, and optionally a report, into the cache directory,
// replacing whatever was there.
func Store(dir string, h *model.History, r *aggregate.Report, historyFP, reportFP string) error
```

### Normative rules

1. `history.json` MUST be written with `collect.WriteHistory`, which already writes to a temporary file and renames.
2. `report.json` MUST be written the same way. It MUST be written even when the run rendered no HTML, and MUST be skipped when `r` is nil.
3. `manifest.json` MUST be written **last**, after both other files are in place, so a manifest never describes files that are not yet there.
4. `HistoryBytes` MUST be read back from the file on disk with `os.Stat` after the rename, not computed from the encoder. A value derived from anything other than the file being described would defeat trigger 6.
5. A failure to write any part of the cache MUST NOT fail the run. The cache is an optimization; a full analysis that succeeded and then could not be cached has still succeeded. Report it as a warning on stderr and exit 0.
6. `Store` MUST create the directory with `0o700`, not `0o755`. A cache holds commit messages, author names and email addresses from a repository the user may not have shared with everyone on the machine.

### Size reporting

`Store` MUST return, or make available, the resulting `history.json` size, so that WP-2.6 can emit a warning when it exceeds the budget documented by M7. The warning MUST name the path and the size and MUST NOT delete anything: the user decides what to do about their own disk.

### Acceptance criteria

- After `Store`, the directory contains exactly the files written, each valid on its own.
- Killing the process between the history write and the manifest write leaves a cache that `Load` rejects, not one it accepts.
- A read-only cache directory produces a warning and exit code 0, with the report still written to the output directory.
- The directory is created with mode `0o700` on platforms where that is meaningful.

---

## WP-2.6 — Wire into the run, `--cache` and `--no-cache`

### Flags

Add to `cli.Options` and to the flag set in [`cmd/commitography/main.go`](../../cmd/commitography/main.go):

| Flag | Type | Default | Behaviour |
|---|---|---|---|
| `--cache` | string | empty | Cache directory. Empty selects the per-user default from `cache.Dir` |
| `--no-cache` | bool | false | Disable the cache entirely: read nothing, write nothing |

`--no-cache` MUST take precedence over `--cache`. Passing both is not an error; disabling wins, because that is the reading under which the user is never surprised by a cache they asked not to have.

Both flags MUST be documented in the `README.md` flag table in this package, per the convention in [`phase-2-detailed-work-packages.md`](../phase-2-detailed-work-packages.md) §9.

### Order of operations in `cli.Run`

The cache slots into [`internal/cli/run.go`](../../internal/cli/run.go) between configuration resolution and collection:

1. Resolve configuration as today.
2. If `--no-cache`, set the cache to unavailable and continue.
3. Otherwise resolve the directory with `cache.Dir`. A failure here is a warning, not an error; continue without a cache.
4. Compute `HistoryFingerprint`.
5. `cache.Load`. A miss is normal and silent at default verbosity.
6. `CollectIncremental` with whatever the load produced.
7. Filter, aggregate and render exactly as today. **No stage after collection may behave differently because a cache was used.** This is what makes exit criterion 5 checkable.
8. Compute `ReportFingerprint` and `cache.Store`.

### Progress

The existing progress reporter MUST distinguish the two cases, because "nothing is happening" and "everything was already known" look identical otherwise:

- Full read: unchanged from today.
- Incremental read: a stage line naming how many commits are being read and how many were reused.
- Nothing to read: a stage line saying the history was reused in full.

Progress goes to stderr. `--quiet` stays silent.

### Wrapped mode

`runWrapped` narrows analysis to one calendar year and returns before the dashboard is rendered. It MUST use the cache for collection exactly as the dashboard path does, and it MUST NOT write `report.json` into the cache: a wrapped report covers one year and would be a wrong baseline for the next full run. `Store` is therefore called with a nil report from the wrapped path.

### Acceptance criteria

- `--no-cache` produces no reads from and no writes to any cache directory, verified by pointing `--cache` at an empty directory and asserting it stays empty.
- Two consecutive runs produce reports identical except for `generatedAt`, and the second starts no `git log` process.
- Deleting the cache directory between runs changes only runtime.
- A wrapped run leaves `report.json` in the cache untouched.
- `README.md` documents both flags.

---

## WP-2.7 — Test matrix

Create `internal/cache/cache_test.go` and extend `internal/collect/collect_test.go`.

### Required cases

**Correctness — the cached result must be indistinguishable from an uncached one.**

1. Full read, then cached read, on each of the `basic`, `merges`, `noise`, `bots` and `coupling` fixtures. Reports MUST be identical except `generatedAt`. This is exit criterion 5 and MUST compare the whole report, not a summary of it.
2. Fixture with commits appended between runs: the second run reads exactly the appended commits.
3. Fixture whose tip is rewritten between runs (amend, then rebase): the result matches a full read.
4. Fixture with a branch deleted between runs: the result matches a full read and `Reuse.Dropped` is non-zero.
5. Empty and single-commit fixtures: no panic, correct results, cache written and reused.

**Invalidation — one test per trigger in WP-2.3, plus:**

6. `exclude_paths` changed between runs: the history is reused and the report differs correctly.
7. `.mailmap` edited between runs with `use_mailmap: true`: full re-read.
8. `.mailmap` edited between runs with `use_mailmap: false`: history reused.
9. `--since` changed between runs: full re-read.

**Robustness**

10. Cache directory deleted between runs.
11. Cache directory read-only: warning, exit 0, correct output.
12. `history.json` truncated, and separately corrupted mid-file.
13. `manifest.json` describing a different commit count than `history.json` contains.

### How "no git log was started" is asserted

Case 1's second run and case 5 need to prove that no `git log` ran, not merely that the run was fast. `internal/collect` MUST expose a test-only counter of `git log` invocations, incremented in the one place where the command is constructed. A timing-based assertion is not acceptable: it would pass on a slow machine that ran git anyway.

### Race detector

M2 adds no new concurrency — `collectSharded` is reused unchanged. Phase 1 recorded that the sharded reader has never been run under `-race` because no C toolchain is installed. That gap is unchanged, not widened, and remains the opportunistic errand described in [`phase-1-detailed.md`](../phase-1-detailed.md). WP-2.7 MUST NOT claim race-freedom it has not tested.

### Acceptance criteria

- Every case above exists as a named test and passes.
- No test depends on network access or on wall-clock timing.
- `go test ./internal/cache/... ./internal/collect/...` passes on Windows and Linux.
