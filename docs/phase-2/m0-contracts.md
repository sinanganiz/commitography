# M0 — Contracts and Documents

**Depends on:** nothing. **Blocks:** every other milestone.

M0 changes the written contract that M2 and M3 then implement. Governance in [`project-overview.md`](../project-overview.md) §12 requires the document to be revised before the scope changes, not after, and the SQLite decision is a scope change to a document that names SQLite by product.

Nothing in M0 touches code. All three packages are documentation edits, and all three MUST land before WP-2.1 begins.

| Package | Status |
|---|---|
| WP-0.1 Replace SQLite in the scope documents | ☐ |
| WP-0.2 Update the repository structure and exit criteria in `phase-2.md` | ☐ |
| WP-0.3 Record the waiver and the macOS consequence in `phase-1-detailed.md` | ☐ |

---

## WP-0.1 — Replace SQLite in the scope documents

Decision B1 removes SQLite from Phase 2. Four passages currently promise it and MUST be rewritten.

### Edits

**1. [`project-overview.md`](../project-overview.md) §5, "Storage".** The line reading

> SQLite is used from Phase 2 onward purely as a local incremental cache file, not as an application database.

MUST be replaced with a statement that the Phase 2 incremental cache is a local directory holding the same JSON history artifact the collect stage already produces, plus a manifest, and that it introduces no database and no new dependency. The surrounding bullets ("No backend service", "No database server", "The published artifact is JSON") stay as they are.

**2. [`project-overview.md`](../project-overview.md) §8, "Technology" table.** The row

> | Intermediate cache | SQLite, single file, Phase 2 onward |

MUST become a row naming the JSON history artifact plus manifest, in a local cache directory, from Phase 2 onward.

**3. [`project-overview.md`](../project-overview.md) §11, "Known Pitfalls" table.** The mitigation cell for "Very large repositories" reads "incremental cache in Phase 2" and is still correct. It MUST NOT be changed.

**4. [`phase-2.md`](../phase-2.md) §2 and §3.** Both name SQLite: §2's in-scope bullet "Incremental analysis backed by a local SQLite cache" and §3's "The intended approach is a SQLite cache file storing normalized commit records". Both MUST be rewritten to describe the chosen design. The rest of §3's incremental-analysis paragraph — that the cache is never a source of truth, that inconsistency causes a full re-read, that the file is safe to commit to a CI cache and safe to delete at any time — is unchanged and remains binding.

**5. [`phase-3.md`](../phase-3.md) §3, "Storage".** The sentence "Phase 1 needs no database. Phase 2 uses SQLite as a cache." MUST be corrected. The rest of that section, which argues for SQLite as the Phase 3 persistence default, is a Phase 3 question and MUST NOT be touched — Phase 3 genuinely needs a database, and this decision says nothing about it.

### Normative rules

1. Each rewritten passage MUST state the reason in one sentence, so the decision is legible to a reader who never sees this plan: cgo would break cross-compilation, a pure-Go SQLite would violate the dependency policy, and the pipeline holds every commit in memory anyway.
2. No passage MAY be deleted without replacement. A reader arriving from an old link must find the current answer, not silence.
3. The word "SQLite" MUST NOT survive anywhere in `project-overview.md` or `phase-2.md` after this package, except where it describes a rejected option.

### Acceptance criteria

- `git grep -i sqlite docs/project-overview.md docs/phase-2.md` returns only lines that describe SQLite as rejected.
- `docs/phase-3.md` still argues for SQLite as the Phase 3 default and no longer claims Phase 2 uses it.
- Departure 1 in [`phase-2-detailed-work-packages.md`](../phase-2-detailed-work-packages.md) §7 is reflected in both scope documents.

---

## WP-0.2 — Update the repository structure and exit criteria in `phase-2.md`

### Edits

**1. Exit criteria.** [`phase-2.md`](../phase-2.md) §5 is a single prose paragraph. It MUST be replaced by a pointer to the twelve-row table in [`phase-2-detailed-work-packages.md`](../phase-2-detailed-work-packages.md) §6, keeping the original paragraph as the one-sentence summary above the pointer. The table MUST NOT be duplicated: two copies drift.

**2. Sequencing note.** §6 says detailed task breakdown is deliberately deferred until Phase 1 usage exists. That is now historical. It MUST be replaced with a statement that the breakdown exists, where it lives, and that the prerequisite was waived on 2026-09-06 — including that the question §6 raised, whether incremental analysis is genuinely needed, was answered by measurement rather than assumption, and where those measurements are recorded.

**3. Departure 2.** The refinement in [`phase-2-detailed-work-packages.md`](../phase-2-detailed-work-packages.md) §7 item 2 — that a dropped branch discards only the commits that disappeared rather than the whole cache — MUST be written into `phase-2.md` §3, replacing the current sentence that lists dropped branches as a full-invalidation trigger. Rewritten history MUST remain listed as a full-invalidation trigger.

**4. New top-level paths.** Phase 2 introduces three paths that Phase 1's repository structure (Task 0.1) does not list:

```
action.yml                          # composite GitHub Action, must be at the repository root
templates/
├── github-actions/
│   ├── dashboard.yml               # scheduled run publishing to GitHub Pages
│   └── dashboard-artifact.yml      # private-repository fallback, artifact upload
├── gitlab-ci/
│   └── .gitlab-ci.yml
└── bitbucket/
    └── bitbucket-pipelines.yml
docs/ci.md                          # the CI integration page
```

These MUST be added to `phase-2.md` §4 as the concrete location of deliverables 1, 2, 3, 4 and 8. `action.yml` MUST be at the repository root: GitHub resolves `owner/repo@ref` to an action definition at the root and nowhere else.

### Acceptance criteria

- `phase-2.md` §5 points at the criteria table rather than restating it.
- `phase-2.md` §6 no longer claims the breakdown is deferred.
- `phase-2.md` §3 describes the refined invalidation rule.
- Every deliverable in `phase-2.md` §4 names the path that will hold it.

---

## WP-0.3 — Record the waiver and the macOS consequence in `phase-1-detailed.md`

[`phase-1-detailed.md`](../phase-1-detailed.md) is the document a reader consults to learn what is and is not verified. It currently says Phase 1 is closed with three criteria carried forward. Phase 2 changes the disposition of all three, and two of the changes are good news while one is not.

### Edits

**1. The release checklist table** at the top of `phase-1-detailed.md` MUST gain a column, or a following note, recording for each of the three criteria where it is now handled:

| Criterion | New disposition |
|---|---|
| 1 — macOS | **Still blocked, and Phase 2 does not close it.** CI was declined on 2026-09-06, removing the only remaining closing path other than hardware |
| 11 — Package managers | Scheduled as [WP-1.1 – WP-1.4](m1-release.md) |
| 12 — Reference dashboards | Scheduled as [WP-5.6 – WP-5.7](m5-github.md) |

**2. The "After closure" section** currently ends by observing that if Phase 2 reintroduces automation, the macOS check is the first thing worth automating. That observation MUST be updated to record that Phase 2 was asked to reintroduce automation and declined, so the check remains manual and unscheduled, and that it is the single item standing between the project and a public announcement.

**3. A new note** MUST record that Phase 2 will publish release artifacts including macOS binaries before criterion 1 is met, and that [WP-1.3](m1-release.md) therefore tags a pre-release rather than a stable release.

### Normative rules

1. This package MUST NOT mark criterion 1 as met, deferred, waived, or accepted. It stays open and visible.
2. The `-race` item in "Known gaps worth revisiting" is unaffected and MUST be left in place. It remains an opportunistic errand, and M2 adds concurrent code paths that make it slightly more valuable than before.

### Acceptance criteria

- A reader of `phase-1-detailed.md` alone can determine that criterion 1 is open, why it is open, and what would close it.
- No text in `phase-1-detailed.md` implies that Phase 2 verifies macOS.
