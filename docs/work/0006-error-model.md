# WP-0006: Error model and reason codes

**Area:** foundation
**Implements:** ADR-0041, ADR-0032, ADR-0062, ADR-0064, ADR-0067
**Requires:** WP-0005

## Goal
Every error in the tree is one of two classes, every user-facing error carries a
reason code from the single enumerated set, exit codes and HTTP statuses are
produced by one mapping, no message or log line leaks an address, path or
hostname, and no code path reachable from repository content panics.

## In scope
1. Define two error classes in `internal/core`: **user error** and **internal
   error**. Classification is carried by the type, not by a string or a
   convention.
2. Define the reason code enumeration from `docs/metrics.md` section 13, and
   **extend section 13 with the codes it lacks, in this same change**
   (ADR-0062 clause 2). Section 13 was written for family status; ADR-0041
   clause 7 later merged user-error codes into the same set, and the set must
   grow to cover it. Add exactly these ten codes and no others:

   | Code | Condition |
   |---|---|
   | `invalid_invocation` | Mutually exclusive or malformed flags |
   | `invalid_configuration` | Configuration unreadable, unparseable, or carrying an invalid value or pattern |
   | `git_unavailable` | Git absent from the path, or its version cannot be determined |
   | `git_version_unsupported` | Git present but below the required version |
   | `path_not_found` | The supplied path does not exist |
   | `not_a_repository` | The path exists but is not a repository, or its git directory cannot be resolved |
   | `path_outside_allowed_roots` | A supplied path falls outside every allowed root |
   | `empty_repository` | The repository contains no commits |
   | `year_below_threshold` | The requested year has fewer analysed commits than the minimum |
   | `request_too_large` | A request body exceeds the server's cap |

   `path_outside_allowed_roots` is distinct from `symlink_escaped_root`: the
   first is a supplied path, the second is a traversal discovered during
   analysis, and their remedies differ. `empty_repository` is distinct from
   `empty_population`: the first refuses a run, the second describes a metric.
   Adding any code beyond this list requires a decision, not an assumption.
2a. Add the catalogue checker in both directions: it fails when a code is
   emitted that section 13 does not list, and when section 13 lists a code
   nothing can emit.
3. A user error MUST carry: its reason code, the offending value, and a remedy.
   A message naming neither the value nor a remedy is incomplete. Internal
   errors carry wrapping context sufficient to locate the origin.
3a. **Two frozen refusal messages change**, because one of them names neither a
   value nor a remedy. Both refusal golden files are in the allow list for this
   reason. The commit that changes them states that user errors now carry a
   remedy, per ADR-0019 clause 2. Exit codes do not change.
3b. Apply ADR-0067 to every message and log line: an artifact that can leave
   the machine carries no path, address or hostname; an interactive diagnostic
   may name a path **exactly as the operator supplied it**, never resolved, and
   never an address or hostname. Where a path is resolved to be checked, the
   check uses the resolved form and the message uses the supplied form.
4. Produce exit code and HTTP status from **one mapping over the class**. No
   call site may choose either. Grep must show a single construction site for
   each.
5. The exit code **values** do not change: `0` success, `1` internal, `2` user,
   configuration and repository validation including a refused shallow clone.
   The **mapping of conditions onto them** must match ADR-0034 clause 5, and
   where the current tree disagrees with that record, the record wins and the
   change is stated in the commit body. Specifically: a flag parsing failure
   currently exits `1` and is a usage error, so it becomes `2`. Preserving a
   record violation is not what "behaviour does not change" means.
6. **Create** the leak scan checker — none exists yet — covering the report and
   API responses, with the two destination rules from ADR-0067 clause 6. Its
   log half moves to WP-0007, which introduces the injected log sink it needs;
   record the narrowing next to the checker and name WP-0007.
6a. Fix the two violations the new checker finds: the job status response
   carries an absolute local path, which ADR-0067 clause 2 forbids outright;
   and the not-a-repository error embeds a resolved path, which clause 3
   permits only in its supplied form. The test asserting the second message's
   shape may be updated.
7. Add the top-level recovery layer in the server. A recovered panic is reported
   as an internal error, never as a user error and never as a success.
8. Add a test that runs the hostile-names fixture through the analysis path and
   asserts no panic and no leaked value.
9. Migrate existing error handling onto the two classes. Where a test asserts an
   error string, the assertion may be updated; the exit code it accompanies may
   not.

## Out of scope
- Localised or translated messages.
- Per-package error types.
- Changing any exit code value or HTTP status value.
- Changing what any test asserts other than an error message string.
- The reason codes families emit for `skipped` and `degraded` status, which
  WP-0008 wires into the report. This package defines the shared set; WP-0008
  uses it.

## Files
**May create or modify:** `internal/**`, `cmd/**`, `internal/checks/**`, linter
configuration, `docs/metrics.md` **section 13 only**, and the two refusal
golden files named in clause 3a.
**Must not touch:** `docs/decisions/**`, any part of `docs/metrics.md` other
than section 13, any golden file other than the two refusal files,
`internal/pipeline/interpret/taxonomy/**`, `web/**`.

## Steps
1. Add the ten codes to `docs/metrics.md` section 13, then define the two
   classes and the enumeration from it.
2. Add the enumeration checker in both directions and observe it failing before
   accepting it (ADR-0064 clause 6).
3. Add the single class-to-exit-code and class-to-status mappings.
4. Migrate call sites one package at a time, running the fast gate after each.
5. Extend the leak scan to logs; observe it failing on a deliberately leaked
   path, then revert.
6. Add the server recovery layer and the hostile-input panic test.

## Definition of done
- Every error crossing a package boundary is one of the two classes, enforced by
  the checker.
- Exit code and HTTP status each have exactly one construction site.
- No reason code is emitted that `docs/metrics.md` section 13 does not list, and
  no listed code is unreachable. Section 13 contains the ten codes in clause 2
  and no others added by this package.
- Every condition currently exiting `2` or returning a client error carries a
  code, and a flag parsing failure exits `2`.
- The leak scan exists, applies both ADR-0067 destination rules, and passes.
- No API response contains a path; a diagnostic naming a path reproduces it
  exactly as supplied.
- The hostile-names fixture produces no panic and no leaked address, path or
  hostname.
- Exit codes for the shallow and empty fixtures are unchanged from before this
  package.
- Every checker added here has been observed failing and then passing again;
  state the evidence.

## Verification
```
make gate-fast && make gate-full
git grep -n 'os.Exit(' -- cmd internal        # one site
go test ./internal/checks -run 'Leak|ReasonCode'
<binary> testdata/fixtures/shallow; echo $?   # 2, as before
<binary> testdata/fixtures/empty;   echo $?   # unchanged
```
