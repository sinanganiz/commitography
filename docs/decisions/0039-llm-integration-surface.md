# ADR-0039: One compatible HTTP interface, and full function without it

**Status:** Accepted
**Note:** This record adds constraints to ADR-0014 and ADR-0030. It does not
supersede either.

## Context
ADR-0014 confines language models to prose describing an already-assigned
archetype, disabled by default, never required. What remained undecided was the
integration shape, and the guarantee that an unconfigured deployment is not a
degraded one.

## Decision
1. Integration MUST be a single OpenAI-compatible HTTP interface configured by
   base URL, model name and key. This covers commercial providers, compatible
   proxies and local inference servers with one implementation.
2. Provider SDKs MUST NOT be added as dependencies. Plain HTTP only.
3. **Absence of configuration is not an error state.** No startup warning, no
   error log, no configuration prompt in the interface.
4. **Without a configured model, no metric, archetype, badge, visualisation,
   Wrapped card or report field is missing.** The only difference is that
   archetype prose is the default text rather than personalised text.
5. Every archetype MUST carry a written default description in the taxonomy
   definition file. Archetype prose MUST NOT be empty in any configuration, and
   MUST NOT render as an empty state.
6. Generated text MUST be stored in its own report namespace and MUST be marked
   as model-generated. The interface MUST show that marking.
7. Timeout and maximum output length MUST be enforced and configurable.
8. A failed call MUST NOT fail the interpret stage. The affected field is marked
   `skipped` per ADR-0032 and the default text is used.
9. **Only the assigned archetype, badges and numeric summaries may be sent.**
   Raw commit messages, file paths, code content, identities and email addresses
   MUST NOT be sent.
10. Generated text MUST NOT be produced in public mode (ADR-0029).

## Consequences
- One implementation covers local and remote inference, including deployments
  where nothing leaves the machine.
- The identity privacy boundary in ADR-0033 is not bypassed by the model call,
  which would otherwise be its most likely leak.
- Removing the model changes tone, never structure.

## Out of scope
- Per-provider adapters, embedded models, model-driven classification.

## Assumption
The compatible request schema remains the common denominator of targeted
endpoints.

## Acceptance criteria
- Removing all model configuration produces a report byte-identical outside the
  generated-text namespace.
- Every archetype has a non-empty default description.
- A payload inspection test confirms no raw commit message, path, identity or
  email is sent.
- No provider SDK is present in dependencies.

## Dependencies
ADR-0014, ADR-0030, ADR-0032 and ADR-0033 must be implemented before this one.
