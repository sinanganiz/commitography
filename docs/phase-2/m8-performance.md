# M8 — Performance Verification

**Depends on:** M2 and M3.
**Closes:** Phase 2 exit criterion 6.

Phase 2's central claim is that a repeat run is cheap. This milestone measures it, and is also where two open questions get answered by data rather than by argument: whether the JSON history artifact is fast enough to parse (decision B1's escape hatch), and where the shard threshold should actually sit (decision B6).

**Every performance number quoted anywhere in Phase 2 documentation MUST originate here.** Copying a figure from an earlier document without re-measuring is not permitted; Phase 1's measurements were trustworthy because they were re-taken, and the same discipline applies.

| Package | Status |
|---|---|
| WP-8.1 Benchmark harness | ☐ |
| WP-8.2 Incremental versus full, and the B1 escape hatch | ☐ |
| WP-8.3 Shard threshold calibration | ☐ |
| WP-8.4 Record the results | ☐ |

---

## WP-8.1 — Benchmark harness

Create `testdata/bench.sh`.

### Normative rules

1. It measures **wall-clock time of the whole command**, including process start, git subprocesses and rendering. Go benchmarks measure Go code, and the measurements in [`phase-2-detailed-work-packages.md`](../phase-2-detailed-work-packages.md) §5 establish that Go code is not where the time goes.
2. It takes a repository path and a scenario name, runs the scenario at least three times, and reports each run plus the median. A single timing on a machine with a page cache is a coin toss.
3. It MUST control the cache state explicitly per scenario — cold, warm-with-no-new-commits, warm-with-N-new-commits — and never inherit whatever happened to be on disk.
4. It MUST record, alongside every result: the machine, core count, operating system, git version, Commitography version, repository name and commit count. A number without them cannot be compared to a later number.
5. It MUST NOT require network access at run time. Reference repositories are cloned once, by hand, before benchmarking.
6. It MUST NOT be wired into `go test`. It takes minutes and needs repositories that are not in the tree.

### Scenarios

| Name | Definition |
|---|---|
| `cold` | No cache. The Phase 1 baseline |
| `cold-noblame` | No cache, `--no-blame`. Isolates git's diff cost |
| `warm` | Full cache, no new commits. The best case Phase 2 offers |
| `warm-noblame` | Full cache, no new commits, `--no-blame`. Isolates the M2 contribution from the M3 contribution |
| `warm-plus-100` | Cache missing the most recent 100 commits. The realistic scheduled-run case |
| `warm-rewritten` | Cache whose most recent 100 commits were rebased. The worst realistic case |

### Acceptance criteria

- The harness runs every scenario against a given repository and prints a comparable table.
- Two consecutive invocations of the same scenario agree within 10%, or the harness says they did not.
- Every result carries its environment metadata.

---

## WP-8.2 — Incremental versus full, and the B1 escape hatch

### Repositories

At minimum:

1. **A repository above 100,000 commits.** Exit criterion 6 is stated in those terms. Phase 1 used a full clone of `python/cpython` at 171,014 commits and its numbers are the comparison point, so the same repository SHOULD be used.
2. **A mid-size repository, 2,000–5,000 commits.** This is the shape most users have, it sits below the current shard threshold, and it is the repository the §5 baseline measurements were taken on.
3. **A fixture-scale repository.** Confirms the cache is not a net loss where there is nothing to save.

### Exit criterion 6

An incremental run on the large repository MUST be at least five times faster than the equivalent full run. Against the Phase 1 figure of 34.5 s with `--no-blame`, that means `warm-noblame` at or under 6.9 s.

### This target is at risk, and that is what the escape hatch is for

The warm path's cost is dominated by parsing `history.json`, not by git. On a repository that size the artifact runs to hundreds of megabytes, and `encoding/json` decoding into `[]model.Commit` may well not leave 6.9 s of room once `rev-list` has taken its 1.1 s and aggregation has run.

Decision B1 chose the JSON artifact with an explicit escape hatch: switch the cache to `encoding/gob` if measurement demands it. **This package is where that decision is made, and it MUST be made on the measurement rather than deferred.**

Normative procedure:

1. Measure `warm-noblame` on the large repository and record the total.
2. Break it down into three parts at minimum: `rev-list`, history parse, aggregation. Without the breakdown, a missed target has no actionable cause.
3. If the target is met, record it and keep JSON. Human-readable and inspectable is worth something, and it costs nothing once it is fast enough.
4. If the target is missed **and the history parse is the dominant term**, switch the cache to `encoding/gob`. Both are standard library, so decision E1's zero-new-dependency promise holds either way. Bump `cache.FormatVersion`, update [WP-2.1](m2-cache.md) and the WP-7.2 contract, and re-measure.
5. If the target is missed and the parse is **not** dominant, do not switch formats. Record what actually dominates and treat it as a new finding, not a known problem with a ready answer.

Whichever way it goes, the outcome MUST be written into decision B1's row in [`phase-2-detailed-work-packages.md`](../phase-2-detailed-work-packages.md) §4, so the record says what was chosen and why rather than only what was proposed.

### Acceptance criteria

- All six scenarios measured on all three repositories.
- Exit criterion 6 is recorded as met or not met, with the number.
- The three-part breakdown exists for the large repository's warm run.
- The format decision is recorded either way.

---

## WP-8.3 — Shard threshold calibration

Decision B6. `shardThreshold` in [`internal/collect/gitlog.go`](../../internal/collect/gitlog.go) is 5,000, chosen in Phase 1 on the reasoning that below it the extra `rev-list` pass and process spawns cost more than the parallelism returns. That reasoning was never measured against a repository near the boundary, and the §5 measurement suggests it is too high: a 2,584-commit repository spends 12.0 s inside a single `git log --numstat`.

### Method

1. Choose at least four repositories spread across roughly 500 to 20,000 commits. They MUST differ in files-per-commit as well as commit count: the cost is git's diff computation, so a repository with 2,000 large commits and one with 2,000 tiny commits are different measurements and the threshold has to serve both.
2. For each, measure `cold-noblame` at forced shard counts of 1, 2, 4, 8 and 16, bypassing the threshold.
3. Identify, per repository, the smallest commit count at which sharding wins by more than measurement noise.
4. Set `shardThreshold` to a value justified by the results, and rewrite its comment to cite them. The current comment states a rationale; the new one MUST state a measurement.

### Normative rules

1. `rev-list` cost MUST be reported separately. It is the fixed overhead the threshold exists to avoid paying pointlessly, and on CPython it was 1.1 s.
2. `TestShardedReadMatchesSingleStream` MUST be extended to cover any shard count the new threshold makes reachable on the fixtures. Correctness is the precondition for touching this at all.
3. If the measurements do not justify a change, the threshold stays at 5,000 and the finding is recorded. A calibration that confirms the existing value is a successful calibration.
4. This package MUST NOT redesign the sharding strategy. `maxShards`, `minCommitsPerShard` and the slicing scheme are Phase 1 work and are out of scope; only the threshold is in question.

### Acceptance criteria

- A table of shard count against wall time for every repository measured.
- The threshold is either changed with the measurement cited in its comment, or kept with the measurement recorded.
- The sharded-equals-single-stream test covers the reachable shard counts.
- No behaviour outside the threshold constant changed.

---

## WP-8.4 — Record the results

### Where results live

This file. Append a Results section carrying every measurement, in the shape of the tables in [`phase-1-detailed.md`](../phase-1-detailed.md), which record before-and-after against a named repository at a named commit count on named hardware.

### Normative rules

1. Every number MUST carry its environment: machine, cores, operating system, git version, Commitography version, repository and commit count.
2. Measurements that **disprove** something MUST be recorded with the same prominence as those that confirm it. If M3 turns out not to pay for itself, that belongs here in the same table, not in a footnote. Phase 1's most useful documentation is the part admitting what was not verified.
3. Any claim made elsewhere in Phase 2 documentation MUST cite a row here. A claim with no row is removed from the other document, not justified retroactively.
4. When a measurement changes a decision, the decision's row in [`phase-2-detailed-work-packages.md`](../phase-2-detailed-work-packages.md) §4 MUST be updated in the same commit.

### Required roll-up table

| Repository | Commits | Cold | Cold `--no-blame` | Warm | Warm `--no-blame` | Warm +100 | Ratio |
|---|---|---|---|---|---|---|---|

The Ratio column is cold against warm on the same row, and it is the number exit criterion 6 is judged on.

### Acceptance criteria

- The roll-up table is complete for every repository measured.
- Exit criterion 6 is marked met or not met with its number.
- Decisions B1 and B6 carry their measured outcomes in §4 of the index.
- No performance claim in Phase 2 documentation lacks a row here.
