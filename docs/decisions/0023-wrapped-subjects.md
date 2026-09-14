# ADR-0023: Wrapped exists for two subjects — the repository and the person within it

**Status:** Accepted

## Context
A repository-only year in review has no personal hook and is unlikely to be
shared by an individual. A person-only year in review discards the highest
leverage distribution path available to this project, which is a maintainer
sharing their own project's year with that project's audience.

## Decision
1. Two Wrapped subjects MUST exist, both produced from a single analysis:
   - **repository Wrapped**, describing the repository's year;
   - **person-within-repository Wrapped**, describing one contributor identity
     set's year inside that repository.
2. Repository Wrapped MUST be publishable in public mode.
3. Person Wrapped in public mode MUST follow ADR-0009 clause 2 and ADR-0033:
   session-scoped, not addressable, not indexable.
4. Both subjects MUST receive archetypes and badges under ADR-0014.
5. Cross-repository person Wrapped is **not decided by this ADR and MUST NOT be
   implemented under it**. The data model MUST NOT prevent it: identity sets
   belonging to one person across several repositories MUST be representable
   (ADR-0025).

## Consequences
- One analysis produces both card decks, at no additional analysis cost.
- Maintainers have something to share that is about their project rather than
  about themselves.

## Out of scope
- Team or organisation Wrapped.
- Cross-repository Wrapped, pending a separate decision.

## Assumption
Repository Wrapped is interesting enough that maintainers will share it. This
depends on content quality rather than on architecture.

## Acceptance criteria
- A single analysis yields both a repository Wrapped and a person Wrapped.
- In public mode, no route returns a person Wrapped for a named identity.

## Dependencies
ADR-0014 and ADR-0025 must be implemented before this one.
