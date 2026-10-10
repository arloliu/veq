# VE design re-derivation

Status: active (2026-10-10) — phase 1 approved; collecting its inputs

The first VE design (`docs/specs/ve/ve-architecture.md`, v4.2) was written before any pilot data existed.
The owner re-opened all of it, including the pilot gates (decision V-4 in `docs/specs/ve/ve-decisions.md`).
This plan re-derives the design from evidence, one phase at a time.
Design content goes to the specs and rationale to the decision log; this file only tracks the work.

## Phases

| Phase | Status |
|---|---|
| 1 — Evidence baseline | in-progress |
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

- Done: the equipment gateway does not acknowledge event and alarm reports; the tools are configured not to ask (decision V-8).
- Done: the inputs above were requested from their owners; replies are expected 2026-10-12.
- Run `cmd/logcat` over the new logs and confirm or correct `eapsml-dialect.md`.
- Write `docs/specs/ve/ve-evidence.md` (living):
  per source — EAP logs, gateway capture, manual, identifier exports, standards —
  what it supplies, what it cannot, and at what fidelity.
  Started as `draft` with the pilot evidence, which moved there from P1 §1.
- Record each finding that changes an earlier assumption as a decision entry.

Done when `ve-evidence.md` is `current` and every input above is obtained or recorded as unavailable.

Later phases are detailed when they start.
Proposal P1 (`docs/specs/ve/proposals/p1-generation-service.md`, accepted) is an input to phases 2 and 3.
