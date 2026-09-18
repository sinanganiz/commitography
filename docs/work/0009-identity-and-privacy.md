# WP-0009: Identity model and privacy layers

**Area:** core
**Implements:** ADR-0033, ADR-0010, ADR-0032, ADR-0062
**Requires:** WP-0008

## Goal
The internal working layer holds raw identities, every exported artifact carries
a display name and a stable digest and no raw address, path or hostname, the
digest is stable for the same address across repositories, and candidate
identity merges are suggested but never applied automatically.

## Where identities live in the report

WP-0008 defines the **identities section** and emits `id` and `display_name`
into it. This package completes it. That section is not a metric family, and it
is the only place a report names a contributor outside the family breakdowns
that depend on replay state.

## In scope
1. Define the identity type in a subpackage under `internal/core` (ADR-0066
   clause 2). It carries a display name and a **stable digest**; the raw address
   is held only where the type cannot be serialised with it.
2. The digest is the first 16 hexadecimal characters of the SHA-256 of the
   address after lowercasing and trimming. Stability across repositories is
   required by ADR-0033 clause 4 and is what makes cross-repository matching
   possible without raw addresses leaving the internal layer.
3. **Make serialising a raw address structurally impossible**: keep it in an
   unexported field with no marshalling path, and add a checker that fails if a
   raw address field gains one.
4. Resolve identities through `.mailmap` first, then configuration override,
   before anything is aggregated.
5. Compute candidate merge signals in the internal layer, where raw data is
   available: identical display name after normalisation, identical address
   local part, and the hosting provider `noreply` address pattern. Expose them
   as **suggestions** on the identities section, each naming the signal that
   produced it. Nothing applies them (ADR-0010 clause 4).
5a. Complete the identity entry with the fields WP-0008 left out: the number of
   source addresses folded into this identity, and the merge candidates from
   clause 5. Adding fields is a **document minor version increment**
   (ADR-0031 clause 1). Update `docs/report-schema.json` accordingly.
6. Implement anonymised output: display names become stable pseudonyms, in
   addition to the rules above (ADR-0033 clause 5).
7. Extend the leak scan checker to the exported artifacts that now exist:
   report, API responses and logs. Exported images arrive with WP-0057.
8. Where a commit's author cannot be resolved into an identity, the affected
   family reports `degraded` rather than silently dropping or splitting it
   (ADR-0032). **Add the code `unresolved_identity` to `docs/metrics.md`
   section 13 in this same change** (ADR-0062 clause 2); section 13 has no code
   for this condition. Add that code and no others.
9. **ADR-0033 clause 6 is not implementable here.** It governs public mode,
   which WP-0045 introduces. Implement clauses 1 to 5 and 7, record the
   narrowing next to the checker naming **WP-0045**, and do not treat the
   missing clause as satisfied.

## Out of scope
- Cross-repository person records (WP-0036).
- The reader's identity selection interface (WP-0051).
- The dual-breakdown projection mathematics (WP-0024, ADR-0018). This package
  provides the identity; that package provides the merge arithmetic.
- Deciding which identities are the same. This package suggests; a person
  decides.

## Files
**May create or modify:** `internal/core/**`, `internal/**`, `cmd/**`,
`internal/checks/**`, `testdata/**` golden files, `docs/report-schema.json`,
and `docs/metrics.md` **for the identities section's remaining fields and the
one reason code in clause 8 only**.
**Must not touch:** `docs/decisions/**`, any other part of `docs/metrics.md`,
`docs/legacy/**`, `internal/pipeline/interpret/taxonomy/**`, `web/**`.

## Steps
1. Define the identity type with the raw address unserialisable.
2. Add the digest function and a test proving stability across two fixtures that
   share an address.
3. Move mailmap and configuration resolution ahead of aggregation.
4. Add the candidate signals and their suggestion surface.
5. Add anonymised output.
6. Extend the leak scan; observe it failing on a deliberately serialised
   address, then revert.
7. Regenerate golden files, stating in the commit body that identity
   representation changed.

## Definition of done
- No report, API response or log line contains a raw address, an absolute local
  path or a hostname, verified by the leak scan.
- The same address produces the same digest in two different fixtures.
- A raw address field cannot be marshalled; the checker fails when one gains a
  marshalling path.
- Candidate merges are produced as suggestions and are never applied by any code
  path.
- Anonymised output replaces display names with stable pseudonyms and changes
  nothing else.
- An unresolvable author produces a `degraded` family with the
  `unresolved_identity` code, and section 13 gained that code and no other.
- The identities section carries source address counts and merge candidates,
  and the schema validates them.
- The narrowing for ADR-0033 clause 6 is recorded and names WP-0045.

## Verification
```
make gate-full
go test ./internal/checks -run 'Leak|Digest|Identity'
grep -oiE '[a-z0-9._%+-]+@[a-z0-9.-]+' <report> <logs>   # no output
<binary> --anonymize <fixture> && jq '.. | .display_name? // empty' <report>
```
