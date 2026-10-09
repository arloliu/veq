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
| `ve-architecture.md` | living | draft | v4.2 architecture written before pilot data; the starting point of the re-derivation |
| `ve-gem-skeleton.md` | living | current | GEM behaviour a passive VE must exhibit, distilled from SEMI E5, E30, E37 and E37.1 |

Next proposal ID: P1.
