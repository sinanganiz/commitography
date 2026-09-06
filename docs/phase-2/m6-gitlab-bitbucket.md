# M6 — GitLab CI and Bitbucket Pipelines Templates

**Depends on:** M4. **Blocks:** M7.

Decision A3: all three templates ship, but the Action, Pages plumbing and dogfooding go to GitHub only. GitLab and Bitbucket get working templates that **have never been executed in a real pipeline**, and they say so on their first line.

That labelling is the point of this milestone as much as the templates are. Phase 1's most expensive lesson was that an unverified claim costs more than an absent feature: the `LC_UUID` defect survived because a platform nobody ran was described as supported. Shipping two templates while claiming three verified integrations would repeat it in a place that is harder to check.

| Package | Status |
|---|---|
| WP-6.1 GitLab CI template | ☐ |
| WP-6.2 Bitbucket Pipelines template | ☐ |
| WP-6.3 Verification status and how it gets cleared | ☐ |

---

## WP-6.1 — GitLab CI template

Create `templates/gitlab-ci/.gitlab-ci.yml`.

Neither GitLab nor Bitbucket has an equivalent of a composite action, so both templates run the GHCR container image from decision C3. This is what the image is for, and it means neither template needs a download-and-verify step of its own.

### Normative contents

1. **`GIT_DEPTH: 0`** as a job or global variable, with an inline comment stating that GitLab shallow-clones by default and that removing this silently produces wrong numbers.
2. `image:` pinned to the GHCR image **by tag, not by `latest`**, for the same reproducibility reason WP-5.1 pins the Action's version.
3. A job named exactly `pages`, writing its output into `public/` and declaring `artifacts: paths: [public]`. GitLab Pages recognizes that job name and that directory and nothing else.
4. `rules` limiting publication to the default branch, plus a `schedule` entry, so the template works when a pipeline schedule is added without further edits.
5. A `cache:` block covering the Commitography cache directory.
6. The same three privacy comments WP-5.3 requires, at the same position: immediately above the run step, covering hashed emails, what `--per-author` publishes, and `--anonymize` as the recommendation for a public dashboard.

### GitLab caching differs from GitHub's and the comment must say so

`actions/cache` never updates an existing key, which is why [WP-5.5](m5-github.md) requires a unique key with a restore prefix. GitLab's cache **does** update on job completion, so a stable `key` is correct there and the GitHub pattern would be wrong.

This difference MUST be stated as a comment in each template. A user copying between the two systems will otherwise carry the wrong pattern across, and the failure is silent: the cache appears to work while never advancing.

### Acceptance criteria

- The file passes GitLab's CI lint.
- Every required element above is present.
- The caching comment states the difference from GitHub explicitly.
- The header carries the WP-6.3 verification notice.

---

## WP-6.2 — Bitbucket Pipelines template

Create `templates/bitbucket/bitbucket-pipelines.yml`.

Bitbucket has no static-hosting equivalent to Pages, so this template produces a downloadable artifact. [`phase-2.md`](../phase-2.md) deliverable 3 specifies exactly that.

### Normative contents

1. **`clone: depth: full`**, with the same inline comment about silently wrong numbers.
2. The GHCR image, pinned by tag.
3. A `schedules`-compatible pipeline definition, so that adding a schedule in the Bitbucket interface requires no edit to the file.
4. `artifacts:` covering the output directory.
5. A `caches:` definition with a custom cache pointing at the Commitography cache directory.
6. The same three privacy comments.
7. A header comment stating that the artifact is a single self-contained file, downloaded and opened locally with no server and no network — the same explanation WP-5.4 gives for the GitHub artifact variant.

### Acceptance criteria

- The file passes Bitbucket's pipeline validator.
- Every required element above is present.
- The header carries the WP-6.3 verification notice.

---

## WP-6.3 — Verification status and how it gets cleared

### The notice

Both templates MUST open with a comment block, before any YAML, stating in plain language:

- That the template has not been run in a real GitLab or Bitbucket pipeline.
- That it was written against those systems' documentation and validated only by their linters.
- That the shallow-clone setting is the one line most likely to matter and the one most likely to be dropped when adapting it.
- Where to report a problem.

The wording MUST NOT be softened into "should work" or "experimental". It states what was and was not done.

### The same notice in three more places

1. `README.md`, wherever CI support is described.
2. `docs/ci.md` from [WP-7.1](m7-docs.md), in each system's section.
3. [`phase-2.md`](../phase-2.md) §2, whose in-scope bullet currently promises three sets of pipeline definitions without qualification.

### How the notice is removed

A template's notice MAY be removed only when someone has run it end to end on that system and produced a dashboard, and has recorded here: the date, the system, and the URL or artifact produced. Passing a linter is not verification and MUST NOT clear the notice.

Until then, Phase 2 exit criterion 10 is satisfied by GitHub alone, and the criterion's wording is accurate about that: it names copying one template into a repository on GitHub.

### Acceptance criteria

- Both templates carry the notice.
- The notice appears in all three additional locations.
- This package records what would be needed to clear each notice.
