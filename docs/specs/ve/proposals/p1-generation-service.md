# P1 — A self-learning VE generation service

Status: active (2026-10-10) — accepted; its §8 changes and phase 2's numbers are pending

This proposal turns the first pilot evidence into a design for the service that generates Virtual Equipment (VE).
The service learns each tool type's behaviour from evidence (the inner loop)
and learns which of its own learners, priors and parameters work across tool types (the outer loop).
Its own design choices are hypotheses with metrics, so the service's design evolves by evidence too.

It feeds phases 2 and 3 of the design re-derivation (see the [roadmap](../../../ROADMAP.md)).
It does not replace phase 1, the evidence baseline.
This proposal fixes the measurement contracts; phase 2 fills in their numbers (floors, budgets, tolerances, thresholds)
before any experiment that depends on them runs.
The owner decisions behind this text are recorded as V-10 and V-11 in [ve-decisions.md](../ve-decisions.md).

Terminology: `H->E` host to equipment, `E->H` equipment to host.
A *Profile* is the learned, versioned description of one tool type
that the VE runtime executes ([ve-architecture.md](../ve-architecture.md) §3).

---

## 1. Evidence and what it does and does not show

The pilot sample is the EAP logs of two etch tools of one model: 69 minutes and 6 hours of one day.
The measurement record (task, splits, denominators, all models compared) is kept with the prototype that produced it,
in `_proto/ceidpredict/README.md` on the branch `proto/ceid-predict`.
Two tools of one model are too few for any general claim;
each item below says what was observed, how it is read, and what it motivates.

- **Module conditioning helps prediction.**
  *Observed*: collection-event IDs group into modules by a vendor numbering convention.
  On the 6-hour log (4 hours training, 3,019 held-out events), the next event within a known module is predicted 77 % top-1;
  the next event of the whole stream, 58 % top-1.
  *Reading*: the two numbers answer different questions (the first is told the module),
  so they motivate a decomposition into module machines;
  they do not prove it.
- **Which module comes next depends on recent events.**
  *Observed*: choosing the next module from the previous module alone loses to a global trigram;
  choosing it from the previous events gives the best mean log-likelihood on both logs
  (−1.68 against the trigram's −1.73 nats on the 6-hour log)
  and the best top-3 there, but not the best top-1 (56 % against 58 %).
  *Reading*: a predictive dependence between modules in this sample.
  It does not tell synchronising transitions apart from common causes such as host commands or process stages;
  hypothesis H1 tests the object-machine explanation.
- **These captures are too short and too narrow.**
  *Observed*: on the 6-hour log, accuracy levels off after 2–3 hours of training
  while the event vocabulary still grows at 4 hours;
  on the 69-minute log, 21 % of held-out events carry IDs never seen in training.
  Neither log holds a connection start or a rejected command.
  *Reading*: these captures miss behaviour the VE needs; how much log a tool type needs is measured per type, not assumed.
- **What an EAP log cannot show.**
  Report definitions (`S2F33`/`S2F35`/`S2F37`) were sent before the sample, so report values carry no variable names.
  The log has no transport events, and its timestamps are log times, not wire times.
  In one data-collection mode the equipment gateway itself polls the tool and pings it every minute,
  and that traffic never reaches the EAP log.
  One log can hold several equipment connections.
- **Tools of one model share most of their observed catalogs.**
  *Observed*: of the events both tools reported, 89 of 93 link to identical reports,
  and about 96 % of shared equipment constants have the same format.
  *Reading*: enough to motivate a shared tool-type layer;
  it says nothing about guards, timing, values or quirks, which hypothesis H2 tests.

---

## 2. What a Profile describes

A Profile is a network of **concurrent extended timed state machines** joined by **synchronising transitions**,
over a **shared variable store** (status variables, data variables, equipment constants),
plus reply rules, value generators and timing distributions.

- **Machines.**
  The GEM machines (communications, control, processing, alarm, spooling; [ve-gem-skeleton.md](../ve-gem-skeleton.md)),
  one machine per equipment module,
  and **object machines** instantiated per object:
  carriers (SEMI E87), process jobs (E40), control jobs (E94) and substrates (E90), with E39 object services.
  The object standards move from non-goal to skeleton capabilities;
  the standard is their prior and the vendor manual and logs supply the delta.
- **Synchronisation.**
  A transition in one machine can require or trigger a transition in another,
  e.g. a job step that moves a substrate and fires events in two modules.
- **Slots.**
  Two parts may be filled by components that are not state machines:
  the choice among concurrently enabled transitions (interleaving) and value generation.
  A slot component chooses only among transitions the guards allow
  (the runtime keeps its authority rule, [ve-architecture.md](../ve-architecture.md) §3.2),
  writes values only through the variable history (§3.3),
  and keeps its state in Instance State, so it is checkpointed and replayed like every other part (§3.1, §10).
  Everything outside the slots stays an inspectable machine, so conformance can still be reported.

Each message type has a place in this model:
`S1F3` and `S2F13` read the variable store;
`S6F11` is the output of a transition, labelled by its event ID and carrying a snapshot of variables;
host commands (`S2F41`, `S3F17`, `S16F15`) are inputs that a guard accepts or rejects before later events follow
(the sample shows only acceptances);
process progress is a spontaneous transition driven by the scenario and the clock;
alarm text is a template filled with live values.

Splitting events per object (which wafer, which chamber) needs the report definitions,
to know which report value is the object ID.

---

## 3. Prior layers

Every rule in a Profile belongs to one layer and records its provenance:

| Layer | Holds | Learned from |
|---|---|---|
| Standard | GEM and object-standard behaviour | SEMI standards, distilled |
| Vendor / family | conventions shared by one vendor's tools, e.g. ID numbering, manual structure, common quirks | several tool types of that vendor |
| Tool type | catalogs, machines, reports, timing (key: model, firmware range) | the vendor manual and logs of the type's tools |
| Tool | the delta of one tool: enabled options, constant values, its EAP configuration's effects | that tool's logs |

A lower layer overrides a higher one only with evidence.
Logs of a type's tools train its tool-type layer together, which eases the shortage of log data.

**Which rule applies.**
A rule has a layer (its scope) and a state (its authority, [ve-architecture.md](../ve-architecture.md) §5).
They compose in this order:

1. Only rules in state `accepted` or `override` are active.
   `candidate` and `unresolved` rules never are,
   and an `unresolved` rule that affects scored behaviour still blocks promotion (architecture §9).
2. The operator's Instance Overlay comes first; it stays the operator's site configuration (architecture §3).
3. Among active rules for the same behaviour, the most specific layer wins:
   tool delta, then tool type, vendor, standard.
   A lower-layer rule that contradicts a higher layer is a quirk, so it is active only once accepted on evidence.
4. Within one layer, the architecture's order applies:
   override, accepted quirk, manual assertion, library default.

The learned tool delta is a separate, versioned artifact.

---

## 4. Inner loop: learning one tool type

```
 evidence ─► learners ─► candidate artifact ─► evaluation ─► promote or reject ─► registry ─► VE runtime
    ▲                                                                                             │
    └──── data requests ◄──── divergences (offline replay, host test reports) ◄───────────────────┘
```

### 4.1 Evidence intake

- Evidence arrives as tracepack captures, in batches, and the loop runs per batch, incrementally by transport epoch.
- The lasting source is a **traffic log service**:
  the equipment gateway sends the traffic it carries to it,
  and it packs the traffic as tracepack captures into the fab's object storage,
  where they serve learning and troubleshooting alike (one of tracepack's design goals).
  EAP logs converted by `cmd/eapconv` are the source until that service runs, and stay a second vantage afterwards.
- Every capture declares its vantage (tracepack `vantage`)
  and a list of **known unobserved sources**, e.g. "traffic the gateway originates toward the tool".
  A learner must not read absence from a capture with a declared blind spot as absence of behaviour.
- The evidence model of [ve-architecture.md](../ve-architecture.md) §6 still holds:
  per-record fidelity, the quality flags that bar a record from quirk candidacy,
  and `wire-exact` records for transport-layer and malformed-frame evidence (V-7).

### 4.2 Learners

Each part of a Profile is produced by an exchangeable **learner** with one contract:

- **Input**: an evidence slice and the priors of every layer above the target layer.
- **Output**: a fragment for one layer, each rule with its provenance, layer and a confidence.

| Profile part | Example learners |
|---|---|
| Catalogs (IDs, formats, names, units) | log catalog (`cmd/logcat`), manual extraction, identifier exports |
| Reply rules | per-message templates with echo and lookup |
| Value generators | the generator ladder of [ve-architecture.md](../ve-architecture.md) §7 |
| Machine structure within a module | passive automata learning with the standard and manual state models as priors |
| Synchronisation and interleaving | event-keyed module choice (the pilot's best), object-machine products, sequence models in the slot |
| Timing | per-transition delay distributions |
| Quirks | deviation mining against the higher layers |

The outer loop chooses which learner fills each part, per layer.
That choice is only possible because learners are exchangeable and compared on the same suites.

### 4.3 A new tool type

A tool type enters with its manual alone.
The manual and the higher layers yield a **provisional** Profile:
it passes the checks that need no logs (schema and semantic validation, mandatory fixtures that apply),
is labelled *manual-derived* by provenance, and carries no measured-fidelity claim.
"Manual-derived" names where the rules come from; fidelity is only ever a measured result (§5).
The service then estimates, from learning curves of comparable types, how much log it needs,
and issues **data requests**: each names the missing input and the evidence that it is missing.
When data arrives, the Profile is re-learned and must pass the full gate of §5.5 to lose its provisional label.

### 4.4 Thin evidence

Where evidence is thin, the service issues data requests and recommends which recordings to enable,
e.g. gateway-side wire logging.
It never probes a production tool.
One hook is reserved for probing during an offline or maintenance window, each use approved by a person.

---

## 5. Evaluation and promotion

### 5.1 Two kinds of score

- **Predictive score.**
  The learned model is asked for the probability of each held-out observable event,
  given the recorded history up to it (teacher forcing).
  It measures how well the model explains the evidence, and it is cheap; learner selection uses it.
- **Generated-run score.**
  The executable artifact runs in the deterministic runtime against a closed input tape built from the held-out capture
  (recorded host inputs and recorded transaction outcomes, [ve-architecture.md](../ve-architecture.md) §10),
  choosing its own transitions, and its outputs are compared with what the tool recorded.
  It catches drift after the VE's first wrong choice, which teacher forcing hides.
  Promotion requires both.

### 5.2 What is scored

- The scoring population is fixed by the suite, independent of any candidate:
  the observable events (`E->H` primaries with their stream, function and event ID) and replies in the held-out window,
  conditioned only on what precedes them in the capture and on the declared starting state.
- A candidate must give every scored event a probability above zero:
  unseen IDs get probability through an explicit unknown-event allowance,
  so a model cannot score well by declining to cover hard events.
- Timing is scored per transition category the suite defines from observables (from one event ID to the next within a module),
  not per edge a candidate learned.
  Its acceptance rule is an effect size (the KS statistic within a tolerance), not a significance test;
  and it is always labelled with its vantage: EAP-log timing is host-side timing, never wire timing.
- Metrics:
  mean log-likelihood per held-out event (primary, predictive score),
  top-3 next-event accuracy,
  the timing statistic above,
  exact-match rate of replies and of generated events (generated-run score),
  and the rate of unseen IDs, as a signal of missing data rather than a quality score.
- The architecture's safeguards stay ([ve-architecture.md](../ve-architecture.md) §8.1):
  a frozen scorable population per suite version, the first divergence on a supported path counted as a failure,
  coverage non-regression, negative controls, absolute floors, and dimensions never summed into one score.

### 5.3 Suites and their roles

- **Kinds.**
  *Time-split*: train on the earlier part of a capture set, test on a later, disjoint part.
  *Tool-split*: train on some tools of a type and test on another; when a tool delta is evaluated,
  both alternatives get the same prefix of the held-out tool's capture and are scored on a disjoint suffix.
  *Type-split*: train every layer without one tool type, then onboard it; claims about new types need this kind.
- **Three partitions.**
  Every experiment and onboarding episode splits its evidence three ways;
  the registry records each artifact's evidence lineage, so the split can be checked.
  - *Shared training*: evidence that may shape shared assets (vendor and standard priors, learners, mined features).
    The target tool or type of an evaluation is never in it.
  - *Adaptation*: what the evaluated artifact may learn about its target:
    the manual, identifier exports, and the declared training prefix or evidence schedule.
  - *Assessment*: the held-out part.
    It is excluded transitively from learning, from shared training and from the inputs of any proposal;
    at scoring time the runtime conditions only on its recorded host inputs and the declared starting state (§5.1, §5.2).
- **Roles.**
  A *selection* suite is used freely by the outer loop.
  An *assessment* suite only confirms a selection already made,
  and is never shown to learners, people or models proposing alternatives.
- **Assessment use.**
  - One *use* is one scoring of one artifact on an assessment partition;
    uses are counted per partition, across all candidates, batches and experiments.
  - A use returns only the pre-registered verdict and the scorecard, never per-event detail.
  - Nothing learns from assessment results.
    An alternative proposed after its proposer saw an assessment result is marked *assessment-informed*
    and must be confirmed on a fresh partition.
  - A partition is used up after its declared number of uses.
    Its replacement must be evidence never used before for any purpose (new captures, or tools or types not yet seen);
    re-slicing consumed evidence does not count.
- **Independent unit.**
  Each suite declares its independent unit (a tool, or a type).
  Results are reported with their spread across those units,
  and claims are scoped to the units tested: many events from two tools are two replications, not thousands.

### 5.4 Suite maintenance

- A suite version is frozen once used and is never edited.
- Each suite declares its applicability: the model, firmware range, options and EAP configuration it represents.
- A new version is cut when applicability changes (new firmware, options, configuration),
  when an assessment suite has been used up,
  or when new evidence makes the old one unrepresentative.
- Old versions keep their results.
  A suite whose scope is still supported keeps its regression obligation.
  Moving a trend to a new version requires running both versions on the same artifacts (an overlap), or recording a trend break.
- Who approves a new suite version is independent of the candidate it would judge.

### 5.5 Promotion

- **What is promoted.**
  Every deployed effective artifact is immutable and reproducible:
  its layer versions are pinned or materialised, and a promotion never changes the behaviour of a version already promoted.
  This covers tool-type Profiles, learned tool deltas and the combination the runtime loads.
- **Gate.**
  Promotion is deterministic and reproducible ([ve-architecture.md](../ve-architecture.md) §9),
  needs both scores of §5.1 above their floors, and keeps every promoted version runnable.
  A learned tool delta is promoted on tool-split suites of its own tool, against the tool-type Profile without it.
  A change to a higher layer is an outer-loop change:
  it produces new versions of the affected Profiles, must not regress any of them, and a person approves it.
- **Divergence sources.**
  Offline replay comes first; host test reports (failed fixtures, EAP developers) feed the same queue;
  live shadow comparison through the gateway is a later option.

---

## 6. Outer loop: the service learns

### 6.1 Objective: the cost of an onboarding episode

The outer loop minimises the cost of **onboarding episodes**.
An episode onboards one tool type and is declared before it starts:

- **Start**: the starting package (manual, identifier exports, any logs) and the pinned versions of every prior layer.
- **Target**: a capability scope, a suite version, the scorecard floors and the mandatory fixtures, all fixed for the episode.
- **Success**: the first artifact that passes the target on the episode's assessment partition,
  within the episode's declared number of assessment uses (§5.3); every use counts toward its cost.
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

### 6.2 Autonomy

- The service selects automatically among alternatives already declared in the registry
  (learner per part and layer, hyperparameters, prior weights),
  on selection suites, and confirms on an assessment suite before adoption.
- New alternatives, a new learner or feature, may be proposed by the service, an LLM included,
  but enter the registry as hypotheses and run only after a person approves them.
- Human review is a budgeted resource:
  review time is recorded per item,
  and the service stops raising proposals of a kind
  whose validated yield per review hour falls below the phase-2 threshold.

### 6.3 Registries

The service keeps two registries, both inside the fab and both append-only:
an entry is never edited or deleted, only superseded by a later one.

- The **Profile registry** holds every promoted artifact (§5.5):
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
  It also records every release of log-derived content (§7.1).

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
  an export that leaves the fab follows §7.1.

### 6.4 Initial hypotheses

Each hypothesis is pre-registered as in §6.3 before it runs.
Pre-registration sets the numbers (the margin δ, tolerances, the uncertainty method); it cannot change the rules below.
Every comparison gives both alternatives the same evidence access, unless the hypothesis is about that access.

**Verdict rule**, the same for every hypothesis.
The gain is measured per independent unit (§5.3) and summarised by an uncertainty interval.

- **Adopted**: no veto fires, and the whole interval lies at or above δ.
- **Rejected**: a veto fires, or the whole interval lies below δ; the report says whether the gain is small or negative.
- **Inconclusive**: any other result; the current design stays in place.

Vetoes for every hypothesis: a generated-run regression beyond tolerance, a broken scorecard floor, a failed mandatory fixture.

| ID | Hypothesis | Cohort, split and unit | Gain | Extra veto |
|---|---|---|---|---|
| H1 | Object machines explain interleaving better than event-keyed module choice | types with report definitions; time-split; unit = type; both alternatives get the report definitions | mean log-likelihood per held-out event | — |
| H2 | A tool-type prior plus a tool delta beats learning each tool alone | types with ≥ 3 tools; tool-split with an equal adaptation prefix; unit = tool; the target tool in neither alternative's shared training | two metrics kept apart: likelihood and reply match (rule below) | — |
| H3 | Vendor-layer priors cut the evidence a new type needs | types of one vendor; type-split; unit = type; the same evidence schedule for both | evidence saved to reach the target (§6.1) | a lower success share or more person-hours, beyond tolerance |
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
  while the result still passes the promotion gate (§5.5).
  Passing the gate alone validates nothing.
  The arms never share a queue, so no finding is won by arriving first;
  within an arm a finding counts once;
  proposals still pending at the end of the period count as not validated and are reported separately.

---

## 7. Governance and data boundaries

- Traffic logs never leave the fab; mining runs on local CPU.
  Vendor manuals may be sent to an external LLM for extraction.
- LLMs and hosted decision models are advisory:
  priors from manuals (as cited assertions) and triage of divergences, with a confidence threshold and a person as fallback.
  Scoring and promotion stay deterministic.
- One service instance runs per fab, on one node.
  Profiles never leave the fab, since they inherit the access control of the recordings they come from.

### 7.1 Release of log-derived content (V-11)

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

## 8. Changes if accepted

- [ve-architecture.md](../ve-architecture.md):
  §1 drops E87/E40/E94 from the non-goals;
  §2 adds the learners, the two registries and the outer loop;
  §3 gives the Profile its layers, adds the learned tool delta beside the operator overlay with its precedence,
  and puts object-machine and slot state into Instance State, its lifetime matrix and the checkpoint;
  §4 adds the slot's place in the event core;
  §5 composes rule authority with the layers (§3 of this proposal) and keeps the unresolved-rule gate;
  §6 adds vantage and known unobserved sources;
  §7 becomes the learner contract;
  §8 takes the two scores, the scored population and the suites of §5, keeping its safeguards;
  §9 promotes effective artifacts and learned tool deltas;
  §10 states that slot components run under the same closed-tape determinism;
  §11 separates provenance stages (replay, templates, manual-derived, learned) from measured fidelity.
- [ve-gem-skeleton.md](../ve-gem-skeleton.md): extended with E39, E87, E40, E94 and E90 behaviour, distilled without quotation.
- Phase 2 sets the numbers this proposal leaves open: floors, budgets, margins, tolerances, the review-yield threshold and *k*.

## 9. Left to implementation

Learner algorithms and their parameters;
the registries' schemas;
the suite format;
the sanitising code for releases;
the review queue's tooling;
how a slot component is serialised into a Profile.
