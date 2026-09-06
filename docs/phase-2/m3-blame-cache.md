# M3 — Blame Cache

**Depends on:** M2. **Blocks:** part of M8.
**Designated scope-reduction point.** If Phase 2 must be cut short, M3 is dropped first. Decision B3 then reverts to accepting blame as the incremental floor, and exit criterion 6 is renegotiated because it can no longer be met.

Measured on a 2,584-commit repository: a full run takes 22.5 s, of which 11.4 s is blame. A commit cache cannot touch that half, because blame is HEAD-relative rather than commit-range-relative. Without M3, an incremental run on that repository improves from 22.5 s to roughly 11.5 s. With it, to roughly 1 s.

| Package | Status |
|---|---|
| WP-3.1 Enumerate HEAD with blob object names | ☐ |
| WP-3.2 Blame cache file and validity rule | ☐ |
| WP-3.3 Integrate into the blame pass | ☐ |
| WP-3.4 Tests and measurement | ☐ |

---

## Correctness first: why a blob object name alone is not a sufficient key

Decision B3 states the key as `(path, blob oid)` and justifies it by saying that a blob object name pins file content, so an unchanged file's blame output is identical. **That justification is not quite true, and the design below repairs it.**

Two cases break it:

1. **Change and revert.** A file is modified and then restored to byte-identical content. Its blob object name at HEAD returns to the old value, but `git blame` now attributes the affected lines to the reverting commit, not to the original one. Content is unchanged; blame output is not.
2. **Rewritten history.** A rebase that changes author dates leaves file content untouched while moving lines between years.

The path must also be part of the key for a separate reason: `git blame -M` follows history, so two files with identical content at different paths have different blame output.

Both failure cases are detectable for free, because M2 already computes the information needed. The exact validity rule is stated in WP-3.2. No extra git invocation is required to make the cache correct.

---

## WP-3.1 — Enumerate HEAD with blob object names

`internal/aggregate/code.go` currently lists HEAD with:

```
git ls-tree -r --name-only HEAD
```

which yields paths only.

### Change

Drop `--name-only` and parse the full form, whose records are `<mode> SP <type> SP <object> TAB <path>`.

```go
// headEntry is one blob in HEAD: the path as git emits it and the object name
// of its content.
type headEntry struct {
    Path string
    OID  string
}

// headEntries lists the blobs in HEAD. Entries that are not blobs — submodule
// gitlinks, which git reports with type "commit" — are omitted, because blame
// cannot say anything about them.
func headEntries(repoPath string) ([]headEntry, error)
```

### Normative rules

1. Only records whose type field is exactly `blob` are returned. This is a correctness improvement independent of caching: today a submodule gitlink reaches `blameYears`, which runs `git blame` against it and records a "blame failed" warning on every run of any repository containing a submodule.
2. Paths MUST be handled exactly as the existing code does, including the `-c core.quotePath=false` behaviour already applied through `internal/gitcmd`. Path handling MUST NOT be re-implemented in this package.
3. The existing filtering chain is unchanged: `textCandidates` and `sampleFiles` continue to operate on paths in the same order, so the sampled set is identical to today's for a repository containing no submodules. The deterministic sample required by Task 4.3 MUST NOT shift as a side effect of this change.

### Acceptance criteria

- On the existing fixtures, the sampled path list is byte-identical to the list produced before this change.
- A fixture containing a submodule produces no "blame failed" warning, where it produced one before.
- Every returned entry carries a 40-character object name.

---

## WP-3.2 — Blame cache file and validity rule

Create `internal/cache/blame.go`.

### File format

`blame.json` inside the cache directory:

```go
// BlameCache maps a file's path and content to the year distribution git blame
// reported for it.
type BlameCache struct {
    FormatVersion int                    `json:"formatVersion"`
    Params        string                 `json:"params"`
    Entries       map[string]BlameEntry  `json:"entries"`
}

// BlameEntry is one file's blame result. OID pins the content the years were
// computed from.
type BlameEntry struct {
    OID   string         `json:"oid"`
    Years map[string]int `json:"years"`
}
```

- The `Entries` key is the file path exactly as git emits it.
- `Years` is keyed by four-digit year as a string, because JSON object keys are strings; the value is the blamed line count.
- `Params` is a literal string describing the blame invocation, currently `blame --line-porcelain -w -M HEAD`. Any change to the flags passed to `git blame` MUST change this string, which invalidates every entry.
- `FormatVersion` is independent of `cache.FormatVersion` and covers this file's own layout.

### Normative validity rule

A cached entry for path *P* MAY be used in place of running `git blame` if and only if **all three** hold:

1. `entry.OID` equals the object name `ls-tree` reports for *P* at HEAD.
2. No commit **read from git during this run** lists *P* among its file changes.
3. The incremental read reported `Reuse.Dropped == 0`.

Condition 1 covers ordinary modification. Condition 2 covers change-and-revert: a commit that touched *P* and left its content unchanged is exactly the case a content key misses, and the set of commits read this run is already in hand from WP-2.4 at no cost. Condition 3 covers rewritten history: M2 detects a rewrite as cached commits vanishing from the rev-list output, and a rewrite can move lines between years without changing any content, so the whole blame cache is discarded when one is seen.

Conditions 2 and 3 are conservative in the harmless direction. An old commit read for the first time because the cache was partial will re-blame files it touched, costing time and never correctness.

```go
// LoadBlame reads the blame cache. Absent or unusable is not an error.
func LoadBlame(dir, params string) (*BlameCache, error)

// Usable reports whether the entry for path may be reused, given the object
// name at HEAD, the set of paths touched by commits read in this run, and
// whether any cached commit was dropped.
func (b *BlameCache) Usable(path, oid string, touched map[string]bool, dropped bool) bool

// StoreBlame writes the blame cache, replacing whatever was there.
func StoreBlame(dir string, b *BlameCache) error
```

### Retention

`Entries` MUST be rewritten to contain only the paths present in HEAD at the end of the run. Without this the file grows without bound across renames and deletions on a long-lived CI cache. Retaining the entry for a path still in HEAD but not sampled this run is required: the deterministic sample shifts as the file set changes, and discarding unsampled entries would throw away work that the next run may want.

### Acceptance criteria

- An entry whose object name matches, whose path was untouched, on a run with no drops, is reported usable.
- Each of the three conditions, violated alone, makes the same entry unusable.
- Changing `Params` makes every entry unusable.
- After a run, `blame.json` contains no path absent from HEAD.
- A damaged `blame.json` yields no entries rather than an error or a panic.

---

## WP-3.3 — Integrate into the blame pass

### Changes to `internal/aggregate/code.go`

`blameYears(repoPath string, paths []string)` currently runs one `git blame` per sampled path and aggregates. It MUST be extended to consult the cache first, without changing what it computes.

```go
// BlameSource supplies cached blame results and receives fresh ones. A nil
// BlameSource means every sampled file is blamed, which is the pre-M3
// behaviour and the behaviour under --no-cache.
type BlameSource interface {
    Lookup(path, oid string) (map[string]int, bool)
    Record(path, oid string, years map[string]int)
}
```

`aggregate.Input` gains a `Blame BlameSource` field. `cli.Run` supplies an implementation backed by the cache; every existing caller passing a zero `Input` keeps today's behaviour, since a nil interface means no cache.

### Normative rules

1. The aggregated year distribution MUST be identical whether it came from cache or from git. This is the criterion the tests assert, and it is the whole claim of the milestone.
2. `CodeAgeSampledFiles` and `CodeAgeTotalFiles` MUST report the same numbers as an uncached run. They describe sampling, not work performed, and a user reading the dashboard's sampling ratio MUST NOT see it move because of a cache.
3. Cache hits MUST NOT suppress the `blame` progress stage. The stage line SHOULD state how many files were blamed and how many were reused, for the same reason WP-2.6 distinguishes read from reused.
4. A blame failure for a path MUST still produce the existing warning and MUST NOT be cached. A failure is a property of the run, not of the content.
5. `--no-blame` MUST skip this path entirely and MUST NOT read or write `blame.json`.

### Acceptance criteria

- With a warm blame cache and an unchanged HEAD, no `git blame` process is started, asserted with a test-only invocation counter rather than by timing.
- `codeAge` and `survivingFromFirstYear` are identical to an uncached run on every fixture.
- `CodeAgeSampledFiles` is unaffected by cache state.
- `--no-blame` leaves `blame.json` untouched.

---

## WP-3.4 — Tests and measurement

### Required cases

1. Cold then warm run on the `basic` fixture: identical `codeAge`, zero `git blame` invocations on the second.
2. One sampled file modified between runs: exactly one file re-blamed, result matches a full run.
3. **Change and revert:** a file modified in one commit and restored to byte-identical content in the next, both between runs. The blob object name at HEAD is unchanged, condition 2 must reject the cached entry, and the result must match a full run. This case is the reason the validity rule is what it is and MUST have a dedicated fixture.
4. History rewritten between runs: the whole blame cache is discarded and the result matches a full run.
5. A file deleted between runs: its entry is gone from `blame.json` afterwards.
6. A file renamed between runs: the result matches a full run. `-M` makes blame follow the rename, and the new path has no cached entry.
7. Submodule present: no blame warning, and the submodule contributes nothing to `codeAge`.
8. `--no-blame` on a repository with a populated `blame.json`: the file is not read and not modified.

### Measurement

Record in [`m8-performance.md`](m8-performance.md), on the same repository as the baseline measurements:

| Run | Expected |
|---|---|
| Cold, blame enabled | ≈ 22.5 s |
| Warm, no new commits, blame cached | The number that decides whether M3 was worth building |
| Warm, no new commits, `--no-blame` | Isolates the M2 contribution |

If the warm cached run does not land under 3 s on that repository, the result MUST be recorded as measured rather than explained away, and the cost of the cache lookups themselves investigated before the milestone is marked complete.

### Acceptance criteria

- Every case above exists as a named test and passes.
- Case 3 has its own fixture in `testdata/build-fixtures.sh`, and the fixture script remains idempotent.
- The three measurements are recorded in `m8-performance.md`.
