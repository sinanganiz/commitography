# ADR-0028: Three levels, and lenses are states rather than pages

**Status:** Accepted

## Context
ADR-0006 makes the repository the subject and the person a lens. If the person
view becomes its own page, the person becomes the subject in practice, and the
repository-scope metrics that distinguish this product move to the background.
The same applies to historical comparison.

## Decision
1. The interface MUST have exactly three navigation levels:
   - the repository registry;
   - the repository view;
   - the Wrapped presentation, which is full screen and separate.
2. The repository view MUST consist of a narrative overview followed by
   in-depth sections.
3. **Person lens** and **time lens** MUST be implemented as states applied over
   the repository view. They MUST NOT be separate pages or separate reports.
4. With a lens active, a visualisation MUST remain the same visualisation with
   its reading changed — for example the coupling graph highlights the selected
   person's edges rather than being replaced by a different chart.
5. The repository view MUST be complete and coherent with no lens applied.
6. Lens state MUST be representable in the URL.
7. Wrapped MUST have its own visual language and MUST NOT be a tab within the
   repository view.
8. A self-service view builder in which the reader assembles metrics MUST NOT
   be built. The product presents an opinionated narrative.

## Consequences
- Repository-scope findings stay primary even when a reader is looking at
  themselves.
- Lens state is shareable and reloadable without accounts.
- Two visual languages exist and are maintained deliberately, rather than one
  blurred compromise.

## Out of scope
- Dashboard builders, custom metric selection, saved custom views.
- A person page that is not anchored to a repository.

## Assumption
An anonymous reader with no lens applied sees a complete experience.

## Acceptance criteria
- No route renders person-scoped or comparison content outside the repository
  view.
- Applying a lens changes existing visualisations rather than replacing the
  section set.
- The repository view renders fully with no lens applied.

## Dependencies
ADR-0006 must be implemented before this one.
