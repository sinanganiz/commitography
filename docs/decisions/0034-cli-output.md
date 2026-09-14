# ADR-0034: The CLI emits `report.json` only

**Status:** Accepted

## Context
The product's presentation surface is the web application (ADR-0028); the
previously generated self-contained static HTML file is no longer the product.
The CLI still has two clear jobs: producing evidence for continuous
integration, and producing the deterministic artifacts the golden regime
compares (ADR-0019).

## Decision
1. A single CLI run MUST produce `report.json` and nothing else.
2. The CLI MUST NOT render HTML, images or any presentation output.
3. The CLI MUST be deterministic under ADR-0021 clause 4 and MUST produce a
   report identical to the one server mode produces for the same inputs.
4. Progress and diagnostics MUST be written to standard error; standard output
   MUST stay clean for scripting.
5. Exit codes MUST be: `0` success; `1` unexpected internal error; `2` usage,
   configuration or repository validation error, including a refused shallow
   clone.
6. A shallow clone MUST be detected and MUST cause the run to fail with exit
   code `2` unless an explicit override is given, in which case the report MUST
   record the degraded condition under ADR-0032.
7. The web application MAY accept a `report.json` file supplied by the reader
   and render it. That is a frontend capability and MUST NOT require any
   additional CLI output.

## Consequences
- The CLI is a small, testable, deterministic surface suitable for continuous
  integration and for the golden regime.
- Anyone can analyse a repository on one machine and read the result on
  another, by carrying one file.

## Out of scope
- Static HTML generation.
- Terminal-rendered dashboards and interactive CLI output.

## Assumption
Readers who want a visual result will use the web application.

## Acceptance criteria
- A CLI run writes exactly one file.
- CLI output and server output for identical inputs are byte-identical apart
  from generation metadata.
- A shallow clone without an override exits with code `2`.

## Dependencies
ADR-0021 must be implemented before this one.
