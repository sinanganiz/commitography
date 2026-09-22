# ADR-0076: Metric families declare three input kinds, and none is the working tree

**Status:** Accepted
**Supersedes:** ADR-0024, ADR-0075

## Context
ADR-0024 gave metric families four input kinds, one of them the working tree.
ADR-0075 corrected the `files` family's row. ADR-0073 clause 8 then made replay
read file contents from git objects, so nothing in the product needs a working
tree: a bare repository is fully analysable, and the working tree is only a
source of results that depend on uncommitted local state.

A family that read the working tree would also defeat the aggregate stage's
guarantee that it runs from cached inputs with the repository removed
(ADR-0020 clause 5). This record restates the family contract with the working
tree removed and both earlier corrections folded in, so one table is
authoritative.

## Decision
1. Every metric family implements a common interface and declares:
   - the inputs it requires, from exactly **`commit-records`, `replay-state`,
     `external-service`**;
   - the output namespace it owns in the report, as `docs/metrics.md` gives it;
   - its family version (ADR-0031);
   - its method statement, where it has one (ADR-0032 clause 8).
2. **No family reads the working tree.** File content at the analysed commit
   reaches a family only through replay state, which carries what families need
   so that aggregation never touches git.
3. The pipeline resolves declared inputs, and skips a family whose inputs are
   unavailable, recording the reason under ADR-0032.
4. A family MUST NOT read another family's output. Shared derived data comes
   from an earlier stage or from `core`, never through a family.
5. A family MUST NOT write outside its declared namespace.
6. The catalogue of families and their inputs is:

   | Family | Inputs |
   |---|---|
   | `temporal` | commit-records |
   | `commit-size` | commit-records |
   | `messages` | commit-records |
   | `files` | commit-records, replay-state |
   | `coupling` | commit-records |
   | `ownership` | replay-state |
   | `worktype` | replay-state |
   | `ai-archaeology` | commit-records, replay-state |
   | `hotspot` | commit-records, replay-state |
   | `static-analysis` | replay-state |

7. Adding a family requires an interface implementation, a declared input set,
   an owned namespace, a family version and golden fixture coverage. Nothing
   else in the pipeline may need to change.
8. The optional language-model prose is a consumer declaring
   `external-service`, and defaults to unavailable.
9. **Resolving declared inputs must not change any family's status.** If it
   does, a declaration is wrong, and the change stops there rather than being
   absorbed into a golden update.
10. **The one sanctioned exception:** retiring the working tree may take a
    family that currently reads it to `skipped` with `not_implemented` until that
    family is rebuilt on replay state. The change is recorded as a deviation
    naming the rebuilding package, and the golden change states it.
11. The reason code `worktree_unavailable` no longer has a producer and is
    removed from `docs/metrics.md` section 13 (ADR-0062 clause 6).

## Consequences
- Every family works on a bare repository, which server mode's remote mirrors
  are likely to be.
- A dirty working tree cannot change a result.
- Aggregation can run with the repository removed for every family, not every
  family but two.
- The hotspot family's complexity proxy needs line content at the analysed
  commit, so replay state must carry what it needs; that is the hotspot
  family's rebuild to arrange.

## Out of scope
- Where the pipeline reads repository configuration files such as the mailmap
  and the project configuration from. Those are configuration resolution
  (ADR-0026), not family inputs, and are decided separately.

## Assumption
Every family that reads file content wants it as of the analysed commit, never
as uncommitted local state.

## Acceptance criteria
- The interface admits exactly three input kinds.
- Every family declares exactly its clause 6 row.
- No family reads the working tree.
- `worktree_unavailable` is absent from the catalogue and from the tree.

## Dependencies
ADR-0020 and ADR-0073 must be implemented before this one.
