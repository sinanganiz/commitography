# WP-0010: Configuration planes and resolution

**Area:** core
**Implements:** ADR-0026, ADR-0021, ADR-0062
**Requires:** WP-0008

## Goal
Operational and analysis configuration are separate types, the resolved analysis
configuration is embedded in every report, extracting it and passing it to the
command reproduces that report byte for byte, and no operational value reaches
the report or affects any metric.

## In scope
1. Define two configuration types in a subpackage under `internal/core`:
   - **operational**: listen address, storage location, allowed roots,
     recurrence, mode, limits. Never in the report, never affecting a metric.
   - **analysis**: exclusion lists, thresholds, mailmap usage, merge counting,
     date source, work-type recency window, anonymisation, cardinality limits.
2. Implement the resolution order from ADR-0026 clause 6: built-in defaults,
   then the repository file, then an explicitly supplied file which **replaces**
   the repository file rather than merging with it, then explicit parameters.
3. Exclusion lists **append** to the built-in lists. Setting a list to empty
   disables the built-in list.
4. Unknown keys produce a warning and never fail the run.
5. Embed the **fully resolved, flattened** analysis configuration in the report
   section WP-0008 reserved. Intermediate layers and their precedence are not
   represented in the report; they may be logged.
6. Provide a normalised digest of the resolved analysis configuration. WP-0033
   uses it in the cache key; this package produces it and proves it stable
   against key ordering and formatting.
7. Add the round-trip checker: take the embedded configuration from a report,
   pass it to the command, and require a byte-identical report outside the
   metadata section. Observe it failing when an analysis value is dropped from
   the embedded section, then revert.
8. Add a checker that fails when an operational key appears in a report.
9. **Remove the `hash_emails` key.** ADR-0033 replaced optional hashing with a
   layered rule in which the report never carries a raw address, so the key
   offers a choice that no longer exists. Removing it is not a behavioural
   change under the new identity representation; it is removing a switch with
   one reachable position.

## Out of scope
- Storage or the cache itself (WP-0033).
- The server's configuration surface and recurrence (WP-0037, WP-0042).
- Adding any configuration key, or removing any key other than `hash_emails`.
  This package gives the existing keys a plane and a resolution order.
- Cardinality limit values, which `docs/metrics.md` section 12 defines and
  ADR-0062 makes authoritative.

## Files
**May create or modify:** `internal/core/**`, `internal/**`, `cmd/**`,
`internal/checks/**`, `testdata/**` golden files.
**Must not touch:** `docs/decisions/**`, `docs/metrics.md`, `docs/legacy/**`,
`internal/pipeline/interpret/taxonomy/**`, `web/**`.

## Steps
1. Split the existing configuration into the two types.
2. Implement resolution, then the append and empty-list rules, then the unknown
   key warning.
3. Embed the resolved analysis configuration in the reserved section.
4. Add the normalised digest and its stability tests.
5. Add the round-trip and operational-leak checkers, observing each fail once.
6. Regenerate golden files, stating in the commit body that the configuration
   section is now populated.

## Definition of done
- Extracting the embedded analysis configuration and passing it to the command
  reproduces the report byte for byte outside the metadata section.
- No operational key appears in any report; the checker fails when one is
  introduced.
- Two analyses differing only in one analysis value produce different digests;
  two differing only in key ordering or whitespace produce the same digest.
- An empty exclusion list disables the built-in list; a populated one appends.
- An unknown key warns on standard error and the run succeeds.
- `hash_emails` is absent from the configuration types, the documentation and
  the example configuration, and supplying it warns rather than failing.

## Verification
```
make gate-full
go test ./internal/checks -run 'ConfigRoundTrip|OperationalLeak|ConfigDigest'
jq '.configuration' <report> > cfg.yml && <binary> --config cfg.yml <fixture>
# resulting report identical outside the metadata section
```
