# ADR-0059: Constraint-bearing code carries record references

**Status:** Accepted

## Context
There are many records and no implementer reads all of them. Where a checker
does not exist and the relevant record was not read, the decision is effectively
absent. Decisions therefore need to be local to the code they constrain.

## Decision
1. The following MUST carry, in a file-level comment, the record numbers that
   govern them:
   - each metric family package;
   - each pipeline stage package;
   - the storage interface and its implementation;
   - the git invocation package;
   - the mode switch and capability matrix;
   - report type definitions;
   - the archetype taxonomy definition file;
   - the frontend design token definitions.
2. A checker MUST verify that every referenced record exists and that every file
   in the list above carries at least one reference. The checker MUST NOT
   attempt to verify that a reference is apt.
3. Per-function references MUST NOT be required.
4. Code MUST NOT be embedded in records, because records are not edited after
   acceptance (ADR-0001) and embedded code would drift.
5. `AGENTS.md` MUST state: **if you are modifying a file that carries record
   references, read those records first.**

## Consequences
- An implementer opening a constrained file learns that constraints exist before
  writing anything.
- The reference list is a low-maintenance index from code back to reasoning.
- The checker cannot confirm relevance, only presence; that is accepted as the
  cheap part of a cheap mechanism.

## Out of scope
- Bidirectional traceability matrices.
- Generated documentation linking records to symbols.

## Assumption
Presence of a reference is enough to prompt reading. It cannot guarantee it.

## Acceptance criteria
- Every file in clause 1 carries at least one resolvable record reference.
- A reference to a non-existent record fails the build.
- `AGENTS.md` contains the instruction in clause 5.

## Dependencies
ADR-0001 and ADR-0055 must be implemented before this one.
