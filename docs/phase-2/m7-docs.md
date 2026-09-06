# M7 — Documentation

**Depends on:** M5 and M6.

[`phase-2.md`](../phase-2.md) deliverable 8: a documentation page covering CI integration, with the shallow-clone warning first. Everything else in this milestone exists because Phase 2 introduces two things a user cannot infer from a flag description — a cache with an invalidation contract, and a release procedure with steps that are easy to forget.

Per the convention in [`phase-2-detailed-work-packages.md`](../phase-2-detailed-work-packages.md) §9, each earlier package already updated `README.md` as it landed. M7 does not repeat that work.

| Package | Status |
|---|---|
| WP-7.1 `docs/ci.md` | ☐ |
| WP-7.2 The cache contract | ☐ |
| WP-7.3 Release procedure | ☐ |

---

## WP-7.1 — `docs/ci.md`

### Required order

The shallow-clone warning comes first, before any prose about what CI integration is for. `README.md` already establishes this ordering for the same warning and for the same reason: it is the single most common cause of incorrect output, and a reader who stops after the first screen must have read it.

```
1. Shallow clones produce wrong numbers        <- first, before anything else
2. Choosing a publishing target
3. GitHub Actions
4. GitLab CI
5. Bitbucket Pipelines
6. Azure Pipelines and Jenkins
7. Incremental caching
8. Privacy when publishing
9. Failing a build on warnings
10. Troubleshooting
```

### Normative content per section

**1. Shallow clones.** The five-row table from [`phase-2.md`](../phase-2.md) §3, plus a statement that Commitography refuses to run on a shallow clone and that `--allow-shallow` exists to accept wrong numbers deliberately, not to work around the check.

**2. Choosing a publishing target.** Pages versus artifact, and the fact that Pages is unavailable on private repositories outside paid plans. A reader on a private repository must reach the right template without first following the wrong one.

**3. GitHub Actions.** The Action's inputs and outputs, a minimal example, and a pointer to both templates. It MUST state that `fetch-depth: 0` is the caller's responsibility because the Action does not check out.

**4 and 5. GitLab and Bitbucket.** Each carries the WP-6.3 verification notice. Each states its own caching semantics, since they differ from GitHub's in the way [WP-6.1](m6-gitlab-bitbucket.md) describes.

**6. Azure Pipelines and Jenkins.** Decision A4: the checkout setting only, with no template. This section MUST say that no template ships for these systems rather than leaving a reader to search for one.

**7. Incremental caching.** Points at WP-7.2 rather than restating it.

**8. Privacy when publishing.** [`phase-2.md`](../phase-2.md) §3 requires the privacy defaults to be explicit at the point of decision. This section MUST state: that emails are hashed by default; that `--per-author` publishes named individuals; that `--anonymize` is recommended for a public dashboard of a repository whose contributors were not asked; and that publishing to Pages makes all of it world-readable and indexable. The generated page carries `<meta name="robots" content="noindex">`, which asks search engines not to index it and does not stop anyone from reading it — this section MUST NOT let that meta tag read as a privacy control.

**9. Failing a build on warnings.** `--strict` and exit code 3, and the fact that output is still written before the non-zero exit.

**10. Troubleshooting.** At minimum: wrong numbers from a shallow clone; a cache that never seems to help, with the `actions/cache` fixed-key trap named explicitly; an Action version that resolves to a 404 after a tag is deleted; and per-author data appearing on a public page unintentionally.

### Acceptance criteria

- The page exists at `docs/ci.md` and follows the section order above.
- The shallow-clone warning is the first content on the page.
- `README.md` links to it and does not duplicate it.
- Every template shipped in `templates/` is reachable from this page.

---

## WP-7.2 — The cache contract

A section of `docs/ci.md` or a sibling page it links. It MUST be written for a user deciding whether to trust the cache, not for a contributor reading the code.

### Required content

1. **What the cache is.** A directory holding the collected history, the previous report, blame results and a manifest. No database, no new dependency.
2. **Where it lives.** The per-user default location and the `--cache` override, and the rule that it is never placed inside the analyzed repository.
3. **The guarantee.** Deleting it at any moment changes nothing but runtime. It is never a source of truth. Any inconsistency causes a full re-read.
4. **What invalidates it,** as a table a user can check their own situation against:

| Change | Effect |
|---|---|
| New commits | Only the new commits are read |
| Rewritten history, force-push | Vanished commits are dropped; whatever remains is reused; the blame cache is discarded entirely |
| Branch deleted | Its unreachable commits are dropped; the rest is reused |
| `exclude_paths`, `identities`, `exclude_authors` changed | History reused, report recomputed |
| `.mailmap` changed while `use_mailmap` is on | Full re-read |
| `--since` or `--until` changed | Full re-read |
| Commitography upgraded to a version with a new cache format | Full re-read |
| Cache damaged, truncated, or partially written | Full re-read |

5. **Size.** That the history artifact scales with history and reaches hundreds of megabytes on very large repositories, that CI cache storage is finite, and that not caching is a legitimate choice for a very large repository whose analysis is infrequent.
6. **Concurrency.** That no lock is taken, that concurrent runs sharing one cache path can leave a mismatched set, that the result is a discarded cache rather than a wrong answer, and that a job matrix should therefore not share one path.
7. **Privacy.** That the cache contains commit messages, author names and email addresses in plaintext, that it is created with owner-only permissions, and that a CI cache is readable by anyone who can read that repository's caches. A user publishing anonymized output while caching un-anonymized history should be told, not left to discover it.

### Acceptance criteria

- Every row of the invalidation table corresponds to a passing test from [WP-2.7](m2-cache.md).
- The privacy paragraph exists and is not buried.
- No claim in this document is stronger than what the tests verify.

---

## WP-7.3 — Release procedure

Create `docs/releasing.md`. Without the project's own CI, every release step is manual, and two of them are easy to forget in a way nothing else catches.

### Required content

An ordered checklist covering at minimum:

1. `go vet`, `gofmt`, `go test` all clean.
2. **Rebuild the frontend and confirm `internal/render/assets/` is unchanged in `git status`.** The bundle is committed so that `go build` works without Node, which means a stale bundle produces a release whose dashboard does not match its source and nothing in the pipeline notices.
3. `goreleaser check`.
4. Decide the tag, including whether the pre-release suffix still applies. It applies while Phase 1 exit criterion 1 is open.
5. Export `GITHUB_TOKEN` and `TAP_GITHUB_TOKEN`; authenticate to GHCR.
6. `goreleaser release --clean`, with the arm64 image fallback from [WP-1.3](m1-release.md).
7. **Update the `version` input default in `action.yml`** and commit it. [WP-5.1](m5-github.md) forbids `latest` for reproducibility, which makes this a manual step in every release.
8. Verify installation from Homebrew and Scoop.
9. Pull and run the container image.
10. Confirm the dogfood workflow's next run succeeds against the new tag.

### Normative rules

1. Steps 2 and 7 MUST be marked as the two that nothing else catches. Everything else on the list announces its own failure.
2. The checklist MUST state that macOS remains unverified while Phase 1 exit criterion 1 is open, and that the pre-release suffix is what communicates this to users.
3. When the CI decision is revisited, this document is where the manual steps that automation would replace are already enumerated.

### Acceptance criteria

- The checklist exists and was followed for the release produced by [WP-1.3](m1-release.md).
- Steps 2 and 7 are called out as the silent-failure steps.
- The macOS caveat is present.
