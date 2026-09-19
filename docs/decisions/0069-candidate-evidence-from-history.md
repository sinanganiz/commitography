# ADR-0069: Merge candidates are evidence from the analysed history

**Status:** Accepted

## Context
ADR-0068 stores identifying values in the embedded configuration as digests, and
its own out-of-scope section states that a matcher needing more than an exact
comparison requires its own decision. Merge candidate signals are such a
matcher: they compare address local parts, hosting provider account names and
normalised display names, none of which survive a digest.

The collision surfaces a question that predates it. The signals currently draw
on addresses and names the **configuration** supplies, including addresses no
analysed commit uses. ADR-0007 makes the local clone the only data source; the
configuration is a resolution instruction, not a source. An address that exists
only because an operator typed it is not evidence about this repository.

## Decision
1. Merge candidate signals MUST be computed **only from values the analysed
   commits record**: author and committer names and addresses appearing in the
   analysed population.
2. A value present only in the configuration MUST NOT produce a candidate.
   Configured identity merges still merge; they simply supply no evidence for
   suggesting further merges.
3. Signals MUST be computed in the internal layer from raw values (ADR-0033
   clause 1), before anonymisation, and are therefore identical with
   anonymisation on or off.
4. A candidate names the other identity by its digest and its signal, never by
   an address or a name.
5. Because the evidence comes from history alone, the round trip required by
   ADR-0026 clause 4 and ADR-0068 clause 6 holds: rerunning with the embedded
   configuration reproduces the same candidates, whether that configuration
   names addresses or digests, and with anonymisation on or off.
6. A future signal that cannot be computed from recorded values alone is not
   added. Widening the evidence base to the configuration requires a superseding
   record, and would reopen the reproduction guarantee it exists to protect.

## Consequences
- An address that appears only in the configuration stops producing
  suggestions. This is a narrowing, and it is the correct one: the suggestion
  meant "the history suggests these may be one person", and that address is not
  in the history.
- Signals are independent of the configuration, so the same repository yields
  the same suggestions regardless of how identities were declared.
- Anonymisation cannot change a suggestion, because suggestions are computed
  before it and reference digests.

## Out of scope
- How suggestions are presented, and who acts on them. Nothing applies a
  candidate; a person decides (ADR-0010 clause 4).
- Hashed forms of derived values such as local parts or account names. A short
  value's digest is guessable, which would weaken anonymisation rather than
  preserve it.

## Assumption
Every signal worth having is computable from values the commits record. The one
case this removes — a configured address no commit uses linking a third
identity — is narrow and was never evidence about the repository.

## Acceptance criteria
- A configured address that no analysed commit uses produces no candidate.
- The same repository produces the same candidates whether identities are
  declared in the configuration or not.
- Candidates are byte-identical with anonymisation on and off.
- Rerunning from the embedded configuration reproduces every candidate.

## Dependencies
ADR-0010, ADR-0033 and ADR-0068 must be implemented before this one.
