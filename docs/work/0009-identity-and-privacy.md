# WP-0009: Identity model and privacy layers

**Area:** core
**Implements:** ADR-0033, ADR-0010, ADR-0032
**Requires:** WP-0008

## Goal
The internal working layer holds raw identities, every exported artifact carries
a display name and a stable digest and no raw address, path or hostname, the
digest is stable for the same address across repositories, and candidate
identity merges are suggested but never applied automatically.

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
   as **suggestions**. Nothing applies them (ADR-0010 clause 4).
6. Implement anonymised output: display names become stable pseudonyms, in
   addition to the rules above (ADR-0033 clause 5).
7. Extend the leak scan checker to the exported artifacts that now exist:
   report, API responses and logs. Exported images arrive with WP-0057.
8. Where an identity cannot be resolved, the affected family reports `degraded`
   with a reason code, rather than silently splitting one person into several
   (ADR-0032).

## Out of scope
- Cross-repository person records (WP-0036).
- The reader's identity selection interface (WP-0051).
- The dual-breakdown projection mathematics (WP-0024, ADR-0018). This package
  provides the identity; that package provides the merge arithmetic.
- Deciding which identities are the same. This package suggests; a person
  decides.

## Files
**May create or modify:** `internal/core/**`, `internal/**`, `cmd/**`,
`internal/checks/**`, `testdata/**` golden files.
**Must not touch:** `docs/decisions/**`, `docs/metrics.md`, `docs/legacy/**`,
`internal/pipeline/interpret/taxonomy/**`, `web/**`.

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
- An unresolvable identity produces a `degraded` family with a reason code.

## Verification
```
make gate-full
go test ./internal/checks -run 'Leak|Digest|Identity'
grep -oiE '[a-z0-9._%+-]+@[a-z0-9.-]+' <report> <logs>   # no output
<binary> --anonymize <fixture> && jq '.. | .display_name? // empty' <report>
```
