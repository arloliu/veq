# VE generation service

Status: draft (2026-10-10) — from the accepted proposal P1; phase 2 sets its open numbers

The generation service produces Virtual Equipment (VE) Profiles.
It learns each tool type's behaviour from evidence (the inner loop)
and learns which of its own learners, priors and parameters work across tool types (the outer loop).
Its own design choices are hypotheses with metrics, so the service's design evolves by evidence too
([ve-decisions.md](ve-decisions.md) V-10, V-11, V-13).

What a Profile is, its prior layers, the learner contract, the Evaluator and the promotion gate are in [ve-architecture.md](ve-architecture.md) §3, §5, §7, §8 and §9.
This document holds the rest: the loops, onboarding, the outer loop's objective and autonomy, the registries,
the initial hypotheses and the release of log-derived content.
Phase 2 of the re-derivation sets the numbers left open here:
floors, budgets, margins, tolerances, the review-yield threshold and *k*.

Terminology: `H->E` host to equipment, `E->H` equipment to host.

---

## 1. Inner loop: learning one tool type

```
 evidence ─► learners ─► candidate artifact ─► evaluation ─► promote or reject ─► registry ─► VE runtime
    ▲                                                                                             │
    └──── data requests ◄──── divergences (offline replay, host test reports) ◄───────────────────┘
```

### 1.1 Evidence intake

- Evidence arrives as tracepack captures, in batches, and the loop runs per batch, incrementally by transport epoch.
- The lasting source is a **traffic log service**, kept in its own repository (V-14):
  the equipment gateway sends the traffic it carries to it,
  and it packs the traffic as tracepack captures into the fab's object storage,
  where they serve learning and troubleshooting alike (one of tracepack's design goals).
  EAP logs converted by `cmd/eapconv` are the source until that service runs, and stay a second vantage afterwards.
- The evidence model is that of [ve-architecture.md](ve-architecture.md) §6:
  vantage and known unobserved sources per capture,
  per-record fidelity, the quality flags that bar a record from quirk candidacy,
  and `wire-exact` records for transport-layer and malformed-frame evidence.

### 1.2 A new tool type

A tool type enters with its manual alone.
The manual and the higher layers yield a **provisional** Profile:
it passes the checks that need no logs (schema and semantic validation, mandatory fixtures that apply),
is labelled *manual-derived* by provenance, and carries no measured-fidelity claim.
"Manual-derived" names where the rules come from; fidelity is only ever a measured result (architecture §8).
The service then estimates, from learning curves of comparable types, how much log it needs,
and issues **data requests**: each names the missing input and the evidence that it is missing.
When data arrives, the Profile is re-learned and must pass the full gate of architecture §9 to lose its provisional label.

### 1.3 Thin evidence

Where evidence is thin, the service issues data requests and recommends which recordings to enable,
e.g. gateway-side wire logging.
It never probes a production tool.
One hook is reserved for probing during an offline or maintenance window, each use approved by a person.

---

## 2. Outer loop: the service learns

### 2.1 Objective: the cost of an onboarding episode

The outer loop minimises the cost of **onboarding episodes**.
An episode onboards one tool type and is declared before it starts:

- **Start**: the starting package (manual, identifier exports, any logs) and the pinned versions of every prior layer.
- **Target**: a capability scope, a suite version, the scorecard floors and the mandatory fixtures, all fixed for the episode.
- **Success**: the first artifact that passes the target on the episode's assessment partition,
  within the episode's declared number of assessment uses (architecture §8.5); every use counts toward its cost.
- **Cost**, counted in two dimensions:
  - *evidence*: tool-hours of capture used, also reported as message counts, because traffic density varies;
  - *person-hours*, by category: manual review, configuration, fixture preparation, hypothesis review, failed attempts.
- **Failure**: an episode that exhausts its budget or its assessment uses without passing is recorded with its cost;
  it is never dropped.
- **Comparison** (V-11), on the same declared episodes:
  first, an alternative must not lower the share of episodes that reach the target beyond its tolerance,
  and a failed episode never counts as cheaper than a successful one;
  then, on the episodes both alternatives completed,
  neither cost dimension may get worse beyond its tolerance and at least one must improve.
  Any other outcome goes to a person.
- **Shared work**: an episode's cost is its marginal cost, given the shared assets pinned at its start.
  Building a shared asset (a vendor prior, a new learner) is an outer-loop change,
  with its own budget and a person's approval;
  its cost is recorded, and its benefit counts only over a declared cohort of episodes,
  whose total cost (shared plus episodes) must beat the baseline cohort under the same rule.
- **People**: in a paired comparison nobody onboards the same type twice;
  alternatives are assigned to people in a balanced order, and each person's earlier exposure to the type is recorded.
- **Over time and across types**: comparisons hold the target fixed, or record a rebaseline;
  across types they use declared groups of comparable capability scope, never one pooled number.

### 2.2 Autonomy

- The service selects automatically among alternatives already declared in the registry
  (learner per part and layer, hyperparameters, prior weights),
  on selection suites, and confirms on an assessment suite before adoption.
- New alternatives, a new learner or feature, may be proposed by the service, an LLM included,
  but enter the registry as hypotheses and run only after a person approves them.
- Human review is a budgeted resource:
  review time is recorded per item,
  and the service stops raising proposals of a kind
  whose validated yield per review hour falls below the phase-2 threshold.

### 2.3 Registries

The service keeps two registries, both inside the fab and both append-only:
an entry is never edited or deleted, only superseded by a later one.

- The **Profile registry** holds every promoted artifact (architecture §9):
  tool-type Profiles, learned tool deltas, the combinations the runtime loads,
  and frozen suite versions, each with its evidence lineage.
- The **experiment registry** holds the outer loop's experiments.
  An experiment has two entries:
  - **Pre-registration**, written before any result is seen:
    the design question; the baseline and the alternatives; who proposed it (a person, the service, an LLM);
    the cohort (types, tools, captures) and each alternative's evidence and compute access;
    the suites and their roles; the metrics; the uncertainty treatment (seeds, independent units);
    and the decision rule, with its margins: adopt, reject, or inconclusive.
  - **Result**: the run record (code version, configuration, seeds, evidence lineage), the results,
    and the decision under the pre-registered rule, with the reason.
    The store rejects a result whose pre-registration is missing or later than the run.
  It also records every release of log-derived content (§3.1).

**Storage.**

- **Artifacts** (Profiles, deltas, suites, lineage manifests)
  are immutable files named by the hash of their content,
  in the fab's S3-compatible object storage (MinIO), beside the tracepack captures.
  The store must refuse to overwrite or delete them and must keep every dependency of a promoted artifact;
  the content hash names each artifact and lets every reader verify it.
- **Records** (versions, lineage, pre-registrations, results, decisions, releases)
  go to a transactional database already run in the fab;
  MariaDB is the default.
  The service's database account has no right to update or delete them, which enforces append-only;
  a record refers to artifacts by their content hash.
  A store without transactions or joins does not fit:
  the order of pre-registration and result must be enforced, and lineage queries join records.
- Records are exported periodically as JSON Lines for audit;
  an export that leaves the fab follows §3.1.

### 2.4 Initial hypotheses

Each hypothesis is pre-registered as in §2.3 before it runs.
Pre-registration sets the numbers (the margin δ, tolerances, the uncertainty method); it cannot change the rules below.
Every comparison gives both alternatives the same evidence access, unless the hypothesis is about that access.

**Verdict rule**, the same for every hypothesis.
The gain is measured per independent unit (architecture §8.5) and summarised by an uncertainty interval.

- **Adopted**: no veto fires, and the whole interval lies at or above δ.
- **Rejected**: a veto fires, or the whole interval lies below δ; the report says whether the gain is small or negative.
- **Inconclusive**: any other result; the current design stays in place.

Vetoes for every hypothesis: a generated-run regression beyond tolerance, a broken scorecard floor, a failed mandatory fixture.

| ID | Hypothesis | Cohort, split and unit | Gain | Extra veto |
|---|---|---|---|---|
| H1 | Object machines explain interleaving better than event-keyed module choice | types with report definitions; time-split; unit = type; both alternatives get the report definitions | mean log-likelihood per held-out event | — |
| H2 | A tool-type prior plus a tool delta beats learning each tool alone | types with ≥ 3 tools; tool-split with an equal adaptation prefix; unit = tool; the target tool in neither alternative's shared training | two metrics kept apart: likelihood and reply match (rule below) | — |
| H3 | Vendor-layer priors cut the evidence a new type needs | types of one vendor; type-split; unit = type; the same evidence schedule for both | evidence saved to reach the target (§2.1) | a lower success share or more person-hours, beyond tolerance |
| H4 | Manual behavioural assertions lessen the penalty for events unseen in training | time-split; unit = type; unseen = absent from all training captures; both alternatives get the manual's catalog, only one its behavioural assertions | log-likelihood on unseen events the manual names | a loss on all other events beyond tolerance |
| H5 | LLM-proposed quirk and divergence hypotheses are worth their review time | one period; unit = review batch; each arm gets its own randomly assigned share of the period's divergences | validated findings per review hour | — |

- H2 compares two metrics, each with its own margin δ and non-inferiority tolerance, never on one scale.
  *Adopted*: no veto fires, neither metric's interval reaches below minus its tolerance,
  and at least one metric's interval lies at or above its δ.
  *Rejected*: a veto fires, either metric's interval lies wholly below minus its tolerance,
  or both metrics' intervals lie wholly below their δ.
  *Inconclusive*: any other result.
  Pre-registration states how the two chances of superiority are corrected for.
- H4 also reports catalog against no manual, so a gain from knowing the vocabulary is not credited to the assertions.
  A cohort with no eligible unseen events gives no evidence either way.
- H5 counts *validated findings*.
  A finding is one divergence class: one observable behaviour at one scope (tool, type or layer).
  It is validated only when both hold:
  the divergence it names reproduces on evidence not used to raise it,
  and the rule or fix it led to removes that divergence against the baseline artifact on that evidence,
  while the result still passes the promotion gate (architecture §9).
  Passing the gate alone validates nothing.
  The arms never share a queue, so no finding is won by arriving first;
  within an arm a finding counts once;
  proposals still pending at the end of the period count as not validated and are reported separately.

---

## 3. Governance and data boundaries

- Traffic logs never leave the fab; mining runs on local CPU.
  Vendor manuals may be sent to an external LLM for extraction.
- LLMs and hosted decision models are advisory:
  priors from manuals (as cited assertions) and triage of divergences, with a confidence threshold and a person as fallback.
  Scoring and promotion stay deterministic.
- One service instance runs per fab, on one node.
  Profiles never leave the fab, since they inherit the access control of the recordings they come from.

### 3.1 Release of log-derived content (V-11)

Everything log-derived that leaves the fab follows one release rule:
prompts and examples sent to an external model, diagnostics, hypotheses, priors, registry exports,
and exchanges between fabs.

| Released | Withheld |
|---|---|
| Aggregate metrics: scorecard values, rates, counts, distributions summarised by quantiles | raw messages and SML bodies, in whole or in part |
| Structure with opaque tokens: IDs replaced by tokens issued per release, the token map kept in the fab | real IDs (SVID, ECID, CEID, RPTID, ALID, RCMD), their names and units |
| Message types (stream and function) and codes the SEMI standards define (e.g. HCACK, ALCD values) | variable values and value ranges, recipe names and content |
| Descriptions of learners, parameters and model families | tool, site, lot, carrier and wafer identifiers; absolute dates and times |
| Patterns seen at least *k* times and on at least two tools | any pattern below that support (rare sequences can identify a process) |

- **Unclassified content is withheld.**
- **Approval**: a person the owner names approves each new release category and each new template;
  a release that uses an approved template goes out automatically and is recorded in the registry.
- **Replies** from an external model are advisory and enter the registry as proposals, never as rules.
- Phase 2 sets *k*; the sanitising code is implementation work.

---

## 4. Left to implementation

Learner algorithms and their parameters;
the registries' schemas;
the suite format;
the sanitising code for releases;
the review queue's tooling;
how a slot component is serialised into a Profile.
