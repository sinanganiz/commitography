# Taxonomy

`taxonomy.yml` holds the archetype and badge definitions for both subjects
(ADR-0023). `axes.md` defines the axis names those definitions are written
against and where each axis comes from.

Changing any threshold, identifier or description in `taxonomy.yml` increments
`version` in that file and is recorded in the report (ADR-0030 clause 6).
Archetype identifiers appear in exported images, so an identifier whose meaning
changes invalidates previously shared artifacts; add a new identifier instead of
repurposing one.

Display names and descriptions here are English. They are a translatable layer;
identifiers are not (ADR-0030 clause 5).
