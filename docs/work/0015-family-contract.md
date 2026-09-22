# WP-0015: Family contract

**Area:** pipeline
**Implements:** ADR-0076, ADR-0032, ADR-0031, ADR-0062
**Requires:** WP-0008

## Goal
Every metric family in ADR-0076 clause 6 has a package that declares its inputs,
its namespace, its version and its method statement through one interface, and a
checker verifies every declaration against the catalogue — with nothing routed
through the declarations yet and no golden file moved.

## Why this package was narrowed

The first version also built the registry and routed family output through it.
Assembling the ten families requires importing them, which only `pipeline` or
`cmd` may do (ADR-0040), and this package cannot touch `pipeline`. The contract
is a shape and belongs in `core`; the registry is a composition and belongs to
the stage that composes. Registry, routing and the scratch-family demonstration
moved to WP-0061.

## In scope
1. Define the family interface in `internal/core`. A family declares:
   - the inputs it requires, from exactly `commit-records`, `replay-state`,
     `external-service` — **three kinds**; the working tree is not one
     (ADR-0076 clause 2);
   - its report namespace;
   - its family version (ADR-0031 clause 2);
   - its **method statement**, where the family has one (ADR-0032 clause 8).
     This is the place the pipeline root's comments anticipated; the root still
     writes method text into the report until WP-0061 reads it from here.
2. Give **every family in ADR-0076 clause 6** a declaring package under
   `internal/metrics/`. Seven exist; add declaration-only packages for the
   families that have no implementation yet, each declaring `not_implemented`
   as its status source. A declaration-only package computes nothing.
3. **Declare the catalogue's namespace**, not the report's current key. The
   catalogue is authoritative (ADR-0062 clause 3), and three families differ:
   `commit_size`, `ai_archaeology`, `static_analysis`. The report keeps its
   current keys until WP-0061 routes output through the declarations; **record
   that gap as a deviation naming WP-0061**.
4. **Every family declares exactly its ADR-0076 clause 6 row.** In particular,
   `files` and `hotspot` declare `commit-records` and `replay-state`, and
   `static-analysis` declares `replay-state`.
5. Add the **family declaration checker**, in `internal/checks`, which may
   import every family package. It fails when:
   - a family in ADR-0076 clause 6 has no declaring package;
   - an input outside the three kinds is declared, or none is;
   - a family's declared inputs differ from its ADR-0076 clause 6 row;
   - a declared namespace differs from the one `docs/metrics.md` gives;
   - two families declare the same namespace;
   - a family declares another family's namespace as an input;
   - a version is missing.
   It holds its own list of the ten packages; WP-0061 replaces that list with the
   registry, so that one list exists in the end.
6. Every test this package adds is named with the prefix
   `TestFamilyDeclaration`, so its verification command matches nothing else.
   Observe each failure mode in clause 5 at least once (ADR-0064 clause 6).
7. Correct the Makefile comment that still lists the family contract and
   namespace checkers as not yet existing.

## Out of scope
- **The registry, routing output through declarations, the "only route"
  guarantee, reading method text from declarations, and the scratch-family
  demonstration.** All are WP-0061.
- **Renaming the report's keys.** WP-0061 does it, with the schema and the
  golden regeneration it requires.
- Changing any metric computation.
- Switching `hotspot` off the working tree. Declaring its ADR-0076 row is this
  package's work; changing what it computes is WP-0061's and WP-0026's.

## Files
**May create or modify:** `internal/core/family.go` and neighbouring core files,
`internal/metrics/**` **for declarations and declaration-only packages only**,
`internal/checks/**`, `.golangci.yml` **for the namespace deviation entry
only**, and `Makefile` **for the stale comment only**.
**Must not touch:** `internal/pipeline/**`, `cmd/**`, `testdata/**`,
`docs/decisions/**`, `docs/metrics.md`, `docs/report-schema.json`.

## Steps
1. Define the interface with the method statement field.
2. Add a declaration to each existing family package, one at a time.
3. Add the declaration-only packages.
4. Add the checker, and observe each of its failure modes.
5. Record the namespace deviation.
6. Correct the Makefile comment.

## Definition of done
- Every family in ADR-0076 clause 6 has a declaring package.
- The interface admits exactly three input kinds.
- Every declared namespace equals the one `docs/metrics.md` gives.
- Every family declares exactly its ADR-0076 clause 6 row.
- The declaration checker exists, and each failure mode in clause 5 has been
  observed failing and then passing.
- The namespace gap is recorded as a deviation naming WP-0061.
- **Every golden file is byte-identical**: nothing routes through the
  declarations yet.

## Verification
```
make gate-full
go test ./internal/checks -run '^TestFamilyDeclaration'
git diff --stat <base> -- testdata/        # empty
```
