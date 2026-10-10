# Roadmap

Status: current (2026-10-10) — design re-derivation phase 1 under way

veq builds Virtual Equipment (VE): passive-side simulators of semiconductor tools,
derived from recorded host–equipment traffic and vendor GEM manuals.
This file lists the milestones and links every finite document that is `active` (`.agents/rules/450-doc-lifecycle.md`).

## Milestones

| Milestone | State | Plan |
|---|---|---|
| Repository bootstrap: rules, docs check, first EAP log tooling | done | — |
| Design re-derived from evidence: goals, pilot gates, architecture | in-progress | [ve-design-rederivation](plans/ve-design-rederivation.md) (active); input: [P1 generation service](specs/ve/proposals/p1-generation-service.md) (active) |
| EAP log to tracepack converter (`cmd/eapconv`) | done | — |

Milestones after these are set by the re-derivation.

## Subjects

- [VE](specs/ve/README.md): architecture (draft), generation service (draft), evidence baseline (draft), log service requirements (draft), decisions, GEM behavioural skeleton.
- [eapsml](specs/eapsml/README.md): EAP SML log dialect and its tracepack mapping.
