# M4 - React and MUI Frontend

**Depends on:** M0 and M3. **Blocks:** M5 and M6.

M4 replaces the vanilla DOM dashboard with one React application while
preserving the static CLI mode, report schema and custom SVG visualizations.

| Package | Status |
|---|---|
| WP-4.1 React/MUI toolchain | Complete |
| WP-4.2 Theme and visual language | Complete |
| WP-4.3 Application shell and navigation | Complete |
| WP-4.4 Repository form and advanced options | Complete |
| WP-4.5 Progress and job controls | Complete |
| WP-4.6 Report and Wrapped components | Not started |
| WP-4.7 Job history and terminal states | Not started |
| WP-4.8 Accessibility, responsiveness and static compatibility | Not started |

---

## WP-4.1 - React/MUI toolchain

### Deliverables

Update `web/package.json`, `web/package-lock.json`, `web/tsconfig.json` and
`web/vite.config.ts` for a bundled React application using React, React DOM,
MUI and its required styling dependencies.

### Normative rules

- All packages are pinned through the lockfile.
- The build produces assets that can still be embedded by `internal/render`.
- No CDN, remote font, image host or runtime module import is used.
- No router, global state library or second component library is added unless a
  later work package explicitly changes this contract.

### Acceptance criteria

- `npm.cmd run typecheck` passes.
- `npm.cmd run build` passes.
- The resulting JavaScript and CSS are available under the existing embed path
  or a documented replacement path.
- A clean Go build does not require Node.js when committed assets are current.

## WP-4.2 - Theme and visual language

### Deliverables

Create a custom MUI theme that preserves Commitography's existing visual
identity while making the local runner feel like a focused developer tool.

The theme must provide light and dark modes, semantic colors for progress,
warning, failure, stale and success states, and consistent focus treatment.

### Acceptance criteria

- MUI default Material branding is not visible in the application.
- Existing report charts retain their custom visual language.
- Both themes meet the existing accessibility contrast requirements.
- Theme switching does not use storage or network APIs.

## WP-4.3 - Application shell and navigation

### Deliverables

Implement the Start, Progress, Result and Recent jobs views using local React
state and the browser History API or an equally small local navigation layer.

### Acceptance criteria

- Refreshing a report view produces a useful recovery state rather than a blank
  page.
- Browser back/forward navigation does not duplicate jobs or lose active state.
- The shell clearly distinguishes an active analysis from a completed report.
- Keyboard navigation order is logical and visible.

## WP-4.4 - Repository form and advanced options

### Deliverables

Implement path entry, validation feedback, Start action and an Advanced section
for `noBlame`, `perAuthor`, `anonymize`, `allowShallow`, `countMerges`, `since`
and `until`.

### Acceptance criteria

- The form never suggests that the browser can browse the host filesystem.
- Invalid paths and shallow clone errors are actionable.
- `perAuthor` displays a privacy warning before submission.
- The form prevents duplicate submission while a job is active.
- Form state is not persisted outside the current page/session.

## WP-4.5 - Progress and job controls

### Deliverables

Implement polling, progress display, elapsed time, stage details, warning count
and cancellation.

### Acceptance criteria

- Polling stops at every terminal state.
- An estimated fraction is labeled or visually distinguished as estimated.
- A slow or indeterminate stage still shows activity.
- Cancel requires no page reload and reports the terminal cancellation state.
- Network/API failures offer a retry or refresh action without creating a
  duplicate job.

## WP-4.6 - Report and Wrapped components

### Deliverables

Rewrite the current dashboard sections and Wrapped components as React
components. Keep custom SVG chart generation where it provides the existing
visualizations.

The static renderer must bootstrap the same report components from embedded
JSON. The server renderer must bootstrap them from `/api/v1/jobs/{id}/report`.

### Acceptance criteria

- Every existing report section remains available when its data exists.
- Missing data still omits sections rather than rendering meaningless zeroes.
- The report schema remains version 1.
- Wrapped CLI output remains functional.
- Server mode does not require a separate duplicate report implementation.

## WP-4.7 - Job history and terminal states

### Deliverables

Render the ten-job history with succeeded, failed, cancelled and stale states.
Provide delete and reopen actions where the API permits them.

### Acceptance criteria

- The oldest job eviction behavior is visible and understandable.
- A stale report cannot be mistaken for a current report.
- Error messages distinguish usage, analysis and cancellation failures.
- Empty history and server restart states have intentional empty screens.

## WP-4.8 - Accessibility, responsiveness and static compatibility

### Deliverables

Audit the application at narrow mobile, tablet and desktop widths. Preserve the
existing chart accessibility model with titles, descriptions and data tables or
equivalent disclosures.

### Acceptance criteria

- No horizontal page overflow at 360, 768 and 1440 CSS pixels.
- All controls are keyboard reachable in logical order.
- Progress and terminal status changes are announced appropriately.
- Charts retain an accessible textual representation.
- Opening the generated static HTML with network disabled still works.
