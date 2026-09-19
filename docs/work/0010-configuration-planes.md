# WP-0010: Configuration planes and resolution

**Area:** core
**Implements:** ADR-0026, ADR-0021, ADR-0062, ADR-0068, ADR-0053
**Requires:** WP-0008

## Goal
Operational and analysis configuration are separate types, the resolved analysis
configuration is embedded in every report, extracting it and passing it to the
command reproduces that report byte for byte, and no operational value reaches
the report or affects any metric.

## In scope
1. Define two configuration types in a subpackage under `internal/core`. The
   test is ADR-0026 clause 1: a value belongs to the analysis plane when it
   changes a metric value, and to the operational plane when it does not.
   - **operational**: listen address, storage location, allowed roots,
     recurrence, mode, resource limits, **and `output_dir`**, which decides
     where a file is written and changes no number.
   - **analysis**: exclusion lists, identity merges, thresholds, mailmap usage,
     merge counting, date source, work-type recency window, anonymisation,
     cardinality limits, **and the date bounds and the year**, which decide
     which commits are analysed and therefore every metric derived from them.
1a. **Add the analysis-plane keys that do not exist yet**, because a report
   cannot be reproduced from a configuration that cannot express what produced
   it (ADR-0026 clause 4): the lower and upper date bounds, the year, and the
   work-type recency window, which ADR-0020 clause 4 already requires to be
   configurable with a 30-day default.
1b. **Cardinality limits are analysis-plane but not operator-settable.**
   ADR-0053 clause 6 places them in the plane so they enter the cache key;
   ADR-0062 clause 6 makes `docs/metrics.md` section 12 their only definition.
   Both hold: the values are embedded in the report and enter the digest, and
   the loader **verifies** a supplied value against the catalogue and fails with
   `invalid_configuration` on a mismatch, rather than accepting it. A report
   therefore states which limits produced it, and a catalogue change invalidates
   the cache.
2. Implement the resolution order from ADR-0026 clause 6: built-in defaults,
   then the repository file, then an explicitly supplied file which **replaces**
   the repository file rather than merging with it, then explicit parameters.
3. Exclusion lists **append** to the built-in lists. Setting a list to empty
   disables the built-in list.
4. Unknown keys produce a warning and never fail the run.
5. Embed the **fully resolved, flattened** analysis configuration in the report
   section WP-0008 reserved, and extend `docs/report-schema.json` to describe
   it. Intermediate layers and their precedence are not represented in the
   report; they may be logged. Filling a reserved section is a **document minor
   version increment** (ADR-0031 clause 1); record it.
5a. Apply ADR-0068: every identifying value in the embedded configuration is a
   reference. Addresses become identity digests; the loader accepts a digest
   wherever it accepts an address. Under anonymisation, operator-supplied
   display names become the stable pseudonyms the report uses, and the loader
   accepts a pseudonym wherever it accepts a display name. Class patterns such
   as a bot name suffix stay literal.
6. Provide a normalised digest of the resolved analysis configuration. WP-0033
   uses it in the cache key; this package produces it and proves it stable
   against key ordering and formatting.
7. Add the round-trip checker: take the embedded configuration from a report,
   pass it to the command, and require a byte-identical report outside the
   metadata section. Observe it failing when an analysis value is dropped from
   the embedded section, then revert.
8. Add a checker that fails when an operational key appears in a report.
9. **Remove the `hash_emails` and `theme` keys.** `hash_emails` offers a choice
   ADR-0033 removed; `theme` is validated and never read, and under ADR-0034 the
   command renders nothing for it to apply to. Removing a switch with one
   reachable position is not a behavioural change.
9a. **Fix the example configuration.** Its header states that the values shown
   are the built-in defaults, while `exclude_authors: []` and
   `exclude_paths: []` are the empty-list form that **disables every built-in
   exclusion** (clause 3). Anyone copying the file, and every analysis of this
   repository itself, silently loses bot and generated-path filtering. Comment
   those two keys out, as the file already does for `identities`, and correct
   the header.
9b. **Record two deviations naming WP-0017.** `--no-blame` and `--per-author`
   are accepted and never read; ADR-0020 forbids the first to exist at all and
   ADR-0009 clause 1 makes the second unnecessary. The year currently filters
   the analysis, while ADR-0008 clause 2 requires Wrapped to be generated from
   the same report as the dashboard, so the year must eventually stop reaching
   the pipeline. Removing any of the three changes caller-visible behaviour and
   belongs to WP-0017. This package embeds the year as an analysis value and
   records that it will leave. ADR-0033 replaced optional hashing with a
   layered rule in which the report never carries a raw address, so the key
   offers a choice that no longer exists. Removing it is not a behavioural
   change under the new identity representation; it is removing a switch with
   one reachable position.

## Out of scope
- Storage or the cache itself (WP-0033).
- The server's configuration surface and recurrence (WP-0037, WP-0042).
- Adding any configuration key other than those named in clause 1a, or removing
  any key other than `hash_emails` and `theme`.
- Removing `--no-blame`, `--per-author`, or the year's effect on the analysis.
  Clause 9b records all three against WP-0017.
- Making cardinality limits settable. Clause 1b embeds and verifies them; it
  does not open them.
- Cardinality limit values, which `docs/metrics.md` section 12 defines and
  ADR-0062 makes authoritative.

## Files
**May create or modify:** `internal/core/**`, `internal/**`, `cmd/**`,
`internal/checks/**`, `testdata/**` golden files, `docs/report-schema.json`
**for the configuration section only**, and `.commitography.yml`.
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
- The date bounds, the year and the recency window are settable from the
  configuration file, and a report produced with them on the command line is
  reproduced from its embedded configuration alone.
- A cardinality limit supplied by the operator that disagrees with
  `docs/metrics.md` section 12 fails with `invalid_configuration`.
- No raw address appears in any configuration section; a file naming addresses
  and one naming their digests produce byte-identical reports.
- With anonymisation on, the configuration section contains no display name
  absent from the identities section.
- `.commitography.yml` no longer disables the built-in exclusion lists, and its
  header describes what the file actually does.
- `hash_emails` is absent from the configuration types, the documentation and
  the example configuration, and supplying it warns rather than failing.

## Verification
```
make gate-full
go test ./internal/checks -run 'ConfigRoundTrip|OperationalLeak|ConfigDigest'
jq '.configuration' <report> > cfg.yml && <binary> --config cfg.yml <fixture>
# resulting report identical outside the metadata section
```
