# VE

Virtual Equipment: a passive-side simulator of a semiconductor tool,
derived from recorded host–equipment traffic and vendor GEM manuals,
against which host software (EAP, FDC) can be developed and regression-tested.
The design is being re-derived from evidence (`docs/plans/ve-design-rederivation.md`);
read the decision log first, then the architecture draft.
Documents here follow `.agents/rules/450-doc-lifecycle.md`.

| File | Class | Status | Purpose |
|---|---|---|---|
| `ve-decisions.md` | living | current | owner decisions, append-only |
| `ve-architecture.md` | living | draft | v4.2 architecture with P1's changes applied; the starting point of the re-derivation |
| `ve-generation-service.md` | living | draft | the generation service: learning loops, onboarding, registries, hypotheses, release of log-derived content |
| `ve-gem-skeleton.md` | living | current | GEM behaviour a passive VE must exhibit, distilled from SEMI E5, E30, E37 and E37.1, plus the object machines (E39, E87, E40, E94, E90) |
| `ve-evidence.md` | living | draft | the evidence baseline: per source, what it supplies, what it cannot, and at what fidelity |
| `proposals/p1-generation-service.md` | finite | active | a self-learning VE generation service: two learning loops, prior layers, experiment registry |

Next proposal ID: P2.
