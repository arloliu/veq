# VE design re-derivation

Status: draft (2026-10-09) — awaiting the owner's approval of phase 1

The first VE design (`docs/specs/ve/ve-architecture.md`, v4.2) was written before any pilot data existed.
The owner re-opened all of it, including the pilot gates (decision V-4 in `docs/specs/ve/ve-decisions.md`).
This plan re-derives the design from evidence, one phase at a time.
Design content goes to the specs and rationale to the decision log; this file only tracks the work.

## Phases

| Phase | Status |
|---|---|
| 1 — Evidence baseline | pending |
| 2 — Goals, scope and pilot gates | pending |
| 3 — Architecture, contract by contract | pending |
| 4 — Implementation spec and plan for the first milestone | pending |

## Phase 1 — Evidence baseline

Goal: know, per source, what the VE can learn from it, before any design question is reopened.

Inputs to obtain (owner):

1. The tools' identifier exports (status variables, data variables, equipment constants, alarms, collection events), or the EAP's copy of them.
2. The EAP's report and link definitions (`S2F33`, `S2F35`, `S2F37`): none appear in the current sample, yet 41 report IDs fire.
3. EAP logs over several days that include a connection start and full carrier cycles, alarms and command rejections.
4. The vendor manual's change notes between the tools' software version and the manual's.

Work (veq):

- Read the equipment gateway's code to learn whether it acknowledges event and alarm reports itself (`eapsml-dialect.md` §2).
- Run `cmd/logcat` over the new logs and confirm or correct `eapsml-dialect.md`.
- Write `docs/specs/ve/ve-evidence.md` (living):
  per source — EAP logs, gateway capture, manual, identifier exports, standards —
  what it supplies, what it cannot, and at what fidelity.
- Record each finding that changes an earlier assumption as a decision entry.

Done when `ve-evidence.md` is `current` and every input above is obtained or recorded as unavailable.

Later phases are detailed when they start.
