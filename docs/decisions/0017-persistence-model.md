# ADR-0017: Persistence stores reports, normalized commit records and an incremental checkpoint

**Status:** Accepted

## Context
Recurring re-analysis of a large repository is only viable if each run reads
only what is new. The replay stage (ADR-0020) walks history chronologically and
accumulates per-line ownership state; that state is exactly the definition of
"history processed up to commit X", so it can be checkpointed and resumed.

## Decision
1. Persistence MUST store three things:
   - generated reports, keyed as in clause 2;
   - normalized commit records, so that metric logic can be recomputed without
     reading git again;
   - the replay checkpoint, so that analysis can resume from the last processed
     commit.
2. The report cache key MUST be composed of: the repository identity, the
   analysed commit, the normalized analysis configuration digest (ADR-0026),
   the report document version, and the version of every metric family present
   (ADR-0031). The repository identity MUST be derived from repository content,
   specifically the first commit hash, and MUST NOT be a filesystem path or a
   remote URL.
3. Incremental resume MUST be permitted only when the new head is a descendant
   of the checkpointed commit. If it is not — force push, rebase, history
   rewrite, or an unrelated history — the checkpoint MUST be discarded and a
   full rebuild performed. This check MUST NOT be skippable by configuration.
4. An incremental analysis MUST produce a report identical to a full analysis of
   the same commit with the same configuration. This is a required invariant
   test (ADR-0019).
5. Reports and checkpoints MUST survive process restart.

## Consequences
- Repeated analysis cost becomes proportional to new commits rather than to
  total history.
- Changing metric logic requires re-aggregation but not re-reading git, because
  normalized commit records and the checkpoint are retained.
- The same repository reached by two different paths or URLs resolves to one
  cache entry.

## Out of scope
- Storing repository source content, blobs or working tree snapshots.
- A queryable event store from which reports are derived by query.

## Assumption
Checkpoint size stays within tens of megabytes for a repository of roughly one
million tracked lines. This MUST be measured against a fixture.

## Acceptance criteria
- An incremental run and a full run of the same commit produce byte-identical
  reports apart from generation metadata.
- A force-pushed fixture triggers a full rebuild rather than a resume.
- The same repository analysed from two different paths produces one cache
  entry.

## Dependencies
ADR-0020, ADR-0026 and ADR-0031 must be implemented before this one.
