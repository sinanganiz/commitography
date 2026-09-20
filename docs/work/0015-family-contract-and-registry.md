# WP-0015: Family contract and registry

**Area:** pipeline
**Implements:** ADR-0024, ADR-0032, ADR-0031, ADR-0062
**Requires:** WP-0008

## Goal
Every metric family implements one interface, declares its inputs, owns a
namespace, carries a version and reports a status, and the registry is the only
route by which a family's output reaches the report — with no metric value
changed and no golden file moved.

## In scope
1. Define the family interface. A family declares:
   - the inputs it requires, from exactly this set: `commit-records`,
     `replay-state`, `worktree`, `external-service`;
   - the output namespace it owns;
   - its family version (ADR-0031 clause 2).
2. Define the registry. **Registration is the only way a family's output
   reaches the report.** A computation that writes into the report without
   being registered is a defect.
3. Wire status and reason per family (ADR-0032): `ok`, `skipped` with a reason
   code, `degraded` with a reason code and a confidence indicator. A `skipped`
   family's data fields are empty, never null-filled or zero-filled.
4. Enforce namespace ownership: a family writes only into its own namespace.
   Add the namespace violation checker (ADR-0063 table 2).
5. Add the **family contract checker**: every family in ADR-0024 clause 5
   implements the interface, declares an input set, owns a namespace, carries a
   version, and has golden fixture coverage.
6. **Register the existing metric packages with their behaviour unchanged.**
   The packages under `internal/metrics/` already exist; this package gives
   them the contract, not new definitions. **Golden files must not change.**
   Bringing each family up to its catalogue definition is WP-0018 to WP-0027.
7. Families with no implementation register as `skipped` with
   `not_implemented`, which is what the report already emits, so the report is
   unchanged by registration.
8. A family reads no other family's output (ADR-0024 clause 3). The linter
   already forbids the import; the contract checker asserts it at the data
   level too, by confirming no family declares another family's namespace as an
   input.

## Out of scope
- **Running the families.** The aggregate stage is WP-0061.
- Changing any metric value, threshold, definition or output shape.
- External-service families. The interface admits the input kind; the language
  model consumer is WP-0031.
- Shared derived data. If two families need the same derivation, it moves to
  `core` or an earlier stage (ADR-0024 clause 4); this package does not create
  such a layer speculatively.

## Files
**May create or modify:** `internal/core/family.go` and neighbouring core files,
`internal/metrics/**` **for contract implementation only**,
`internal/checks/**`.
**Must not touch:** `testdata/**` golden files, `internal/pipeline/**`,
`docs/decisions/**`, `docs/metrics.md`, `docs/report-schema.json`.

## Steps
1. Define the interface and the registry against the families that already have
   implementations.
2. Register one family at a time, running the golden comparison after each and
   stopping if any golden moves.
3. Register the unimplemented families as `skipped`.
4. Add the contract and namespace checkers; observe each failing against a
   deliberately non-conforming family, then revert.

## Definition of done
- Every family in ADR-0024 clause 5 is registered with declared inputs, an
  owned namespace and a version.
- The contract checker and the namespace checker exist and have each been
  observed failing and then passing.
- No family declares another family's namespace as an input.
- **Every golden file is byte-identical to its state before this package.**
- Adding a family requires only: an implementation, a declared input set, a
  namespace, a version and golden coverage — verified by adding a trivial
  family in a scratch commit and reverting it.

## Verification
```
make gate-full
git diff --stat <base> -- testdata/    # empty: no golden changed
go test ./internal/checks -run 'FamilyContract|Namespace'
```
