# WP-0061: Aggregate stage and family registry

**Area:** pipeline
**Implements:** ADR-0020, ADR-0024, ADR-0052, ADR-0032, ADR-0031, ADR-0062, ADR-0075
**Requires:** WP-0013, WP-0015

## Goal
The aggregate stage holds the single registry of families, routes every family's
output into the report only through its declaration, resolves each family's
declared inputs, runs the families concurrently, writes the catalogue's
namespaces, and produces an identical report from cached inputs with the
repository unreadable.

## In scope
1. **Build the registry** in `internal/pipeline/aggregate`: the one list of the
   ten family declarations. It replaces the list the WP-0015 declaration checker
   holds, so exactly one list exists.
2. **Registration is the only route** by which a family's output reaches the
   report. Remove the direct writes the stage performs today, and the post-hoc
   method writes in the pipeline root; method text comes from each declaration.
3. Resolve each family's declared inputs and run it. A family whose inputs are
   unavailable is skipped with the matching reason and never run
   (ADR-0024 clause 2).
4. **Resolving declared inputs must not change any family's status**
   (ADR-0075 clause 4). If it does, an input declaration is wrong: **stop and
   report**, do not absorb it into a golden update. In particular, if declaring
   `worktree` makes a family skip that computes today, the `worktree` question
   must be answered before this package lands.
5. **Rename the three report keys to the catalogue's namespaces**:
   `commit_size`, `ai_archaeology`, `static_analysis` (ADR-0062 clause 3).
   Update `docs/report-schema.json` to match. Renaming a key removes a field, so
   this is a **document major version increment** (ADR-0031 clause 1). Remove
   the namespace deviation WP-0015 recorded.
6. Regenerate every golden file **in one commit**, stating the rename and the
   major increment in its body. **The only differences in any golden file are
   the three keys and the document version**; verify this by normalising the
   three keys back and diffing, which must show nothing but the version.
7. Run families **concurrently** (ADR-0052 clause 3); the degree defaults to a
   value derived from available cores, and **never changes the output**. Extend
   the parallel determinism checker to family execution.
8. The stage is **stateless and re-runnable without touching git** (ADR-0020
   clause 5). Add the checker that runs aggregate from cached collect output and
   replay state **with the repository directory removed**, and requires a
   byte-identical report. Observe it failing against a family that reaches for
   the repository.
9. Enforce namespace ownership at write time: a family writes only into its own
   namespace. Add the checker.
10. **Demonstrate ADR-0024 clause 6 in the working tree, without committing.**
    Add a scratch eleventh family and its registry entry. Confirm that **no
    pipeline code other than the registry entry** had to change. Expect exactly
    three failures, named here in advance so none is a surprise: the catalogue
    family check, because ADR-0024 clause 5 fixes the set; the schema, because
    its families object is closed; and the absence of a `docs/metrics.md`
    section. Those three are the designed friction of adding a family, not
    pipeline changes. Revert, and report what changed and what failed.
11. Every test this package adds is named with the prefix `TestAggregate`.

## Out of scope
- The interpret stage (WP-0016).
- Persisting collect output or the replay checkpoint (WP-0033, WP-0034).
- The correctness of any family against its catalogue definition (WP-0018 to
  WP-0027).
- Changing any metric value. Apart from the rename in clause 5, running a family
  through the registry must produce what it produced before.
- Answering the `worktree` question, unless clause 4 forces it — in which case
  stop and report rather than decide.

## Files
**May create or modify:** `internal/pipeline/aggregate/**`,
`internal/pipeline/run.go`, `internal/core/**`, `internal/checks/**`,
`docs/report-schema.json`, `testdata/**` golden files, and `.golangci.yml` **for
removing the namespace deviation entry only**.
**Must not touch:** `internal/metrics/**` other than to read declarations,
`internal/pipeline/collect/**`, `internal/pipeline/replay/**`, `cmd/**`,
`docs/decisions/**`, `docs/metrics.md`.

## Steps
1. Build the registry from the WP-0015 declarations; switch the declaration
   checker to read it.
2. Route each family through the registry one at a time, running the golden
   comparison after each and stopping if any status changes.
3. Move method text onto the declarations and remove the post-hoc writes.
4. Rename the three keys, update the schema, increment the document major
   version, and regenerate the golden files in one commit.
5. Add the no-repository, concurrency and namespace-write checkers.
6. Run the scratch-family demonstration in the working tree and revert it.

## Definition of done
- One registry exists, and the declaration checker reads it.
- No family output reaches the report except through its declaration.
- No family's status changed when declared inputs began to be resolved.
- The report uses `commit_size`, `ai_archaeology` and `static_analysis`; the
  schema agrees; the document major version increased.
- **Normalising the three keys back makes every golden file identical to before
  except for the document version.**
- Aggregate produces a byte-identical report with the repository removed.
- Reports at parallelism degrees 1, 2 and N are byte-identical.
- No family writes outside its namespace.
- The scratch-family demonstration changed only the registry entry, failed
  exactly the three named checks, and was reverted; the report states both.

## Verification
```
make gate-full
go test ./internal/checks -run '^TestAggregate'
# normalise the rename and confirm only the document version differs
git grep -n 'commit-size\|ai-archaeology\|static-analysis' -- testdata/golden docs/report-schema.json
# no output
```
