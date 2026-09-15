# Commitography — project overview

Commitography analyses the commit history of a git repository and describes
the repository.

This document is an orientation. It is not binding and states no rule of its
own. Every rule lives in the documents listed below; where this document and
one of them disagree, this document is wrong.

---

## Where the rules live

| Document | Status | Contents |
|---|---|---|
| `docs/decisions/INDEX.md` | **Binding** | The accepted architecture decision records. This is the source of every product, architecture and process rule. |
| `docs/metrics.md` | **Binding** | The metric catalogue (ADR-0062). |
| `docs/conventions.md` | Guidance | Patterns that are not enforced by tooling (ADR-0058). |
| `docs/work/INDEX.md` | Work | Numbered work packages and their implementation dependencies. |
| `AGENTS.md` | Instructions | How to work in this repository. |

---

## What the decision set covers

The records are grouped here by subject so that the relevant ones can be found.
The grouping says what each group is about, not what it decides; read the
records for that.

| Subject | Records |
|---|---|
| How decisions, rules and work are recorded and enforced | ADR-0001, ADR-0004, ADR-0055, ADR-0056, ADR-0058, ADR-0059 |
| Licence, goals, distribution, unit of analysis and data sources | ADR-0002, ADR-0003, ADR-0005, ADR-0006, ADR-0007, ADR-0012, ADR-0022 |
| Presentation, interpretation, Wrapped and sharing | ADR-0008, ADR-0009, ADR-0013, ADR-0014, ADR-0015, ADR-0023, ADR-0028, ADR-0030, ADR-0039 |
| Identity and privacy | ADR-0010, ADR-0025, ADR-0033 |
| Server mode, persistence, configuration and job handling | ADR-0011, ADR-0016, ADR-0017, ADR-0026, ADR-0027, ADR-0029, ADR-0035 |
| Analysis pipeline and metric families | ADR-0018, ADR-0020, ADR-0024, ADR-0051, ADR-0052, ADR-0062 |
| The report document and the command-line output | ADR-0021, ADR-0031, ADR-0032, ADR-0034 |
| Frontend delivery, visualisation and styling | ADR-0036, ADR-0037, ADR-0038, ADR-0053 |
| Code structure, errors, wiring, HTTP API and concurrency | ADR-0040, ADR-0041, ADR-0042, ADR-0043, ADR-0044, ADR-0060, ADR-0061 |
| Threat model, filesystem boundary, subprocesses and resource limits | ADR-0045, ADR-0046, ADR-0047, ADR-0048 |
| Verification, performance budgets, supply chain and CI gates | ADR-0019, ADR-0049, ADR-0050, ADR-0054, ADR-0057 |

The index also collects every prohibition stated in the records into one list.

---

## The tracked tree and the decision set

The code in this repository was written before the current decision set and
does not conform to it. `docs/work/audit.md` classifies every tracked file and
names the record each non-conforming file contradicts. The work that brings the
tree into conformance is defined in `docs/work/INDEX.md`.

`docs/legacy/report-schema-v0.json` is the schema of the report document the
decision set replaces. It is kept because existing tests validate against it.
It is not a contract.
