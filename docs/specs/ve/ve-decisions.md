# Virtual Equipment decisions

Status: current (2026-10-09) — append-only owner decision log

Owner decisions for the Virtual Equipment (VE), newest last.
Each entry gives its date (from its round heading or inline), its ID, the decision and its consequence.
The log is append-only: an entry is never edited or removed after it is recorded.
A superseded entry keeps its text and gains "(superseded by <ID>)".
IDs never change.
Entries marked "(also in the tracepack decision log)" are recorded in both logs.

## 2026-09-25 (after review v1)

- D-Q1 Host-side clock: deferred (nice-to-have).
  VE-internal virtual clock only.
- D-Q2 Target hosts: both EAP flows and FDC flows.
- D-Q3 Vendor GEM manuals available;
  heterogeneous formats;
  LLM converts to Profile schema.
- D-Q4 Separate repo;
  Go preferred, Python allowed for offline tooling;
  no GPU required.
- D-LOG Traffic-log storage format deferred;
  assume a complete canonical capture;
  adapters later.

## 2026-09-26 grilling round 1 (owner accepted all recommendations)

- G1-1 Pilot fixtures (frozen):
  EAP (a) S2F41 START rejected with HCACK≠0, (b) host reconnect without report/enable rebuild → missed events;
  FDC (c) START without END / duplicate START → dangling context, (d) S6F11 report position vs seq_path mismatch.
  (e) late S1F3 vs sampling period and (f) S7F19 directory consistency go to batch 2.
  Fact: the equipment gateway's auto-reply to S6F5 is always B 0x00 (GRANT6=0),
  so grant-refusal paths need the interactive harness to play host.
- G1-2 Transport: first accepted release is HSMS-SS only;
  SECS-I binding deferred;
  core transport interface abstract from day one.
- G1-3 Unseen-input default: `unsupported` (E30 default reply + diagnostic);
  `default` opt-in per Profile.
- G1-4 Instance State survives VE runtime restart (persisted);
  checkpoint format doubles as evaluation-episode initial state.
- G1-5 Under acceleration, assertions allowed to be invalid:
  START→END wall duration, event-timestamp vs host-receive skew, host T3/T6 behaviour.
  Must hold: trace sample count, S1F3 jitter.
  Compression applies only to idle intervals with no polling and no trace running.
- G1-6 Quirk/conflict authority: auto-accept when evidence count ≥ 3 across ≥ 2 captures, plus human review;
  owner has final say.
  Thresholds revisited after pilot.
- G1-7 Quirk scope v1: observed semantic-layer deviations + authored fault overlays.
  Transport-layer deviations (Select/Linktest/Separate behaviour) deferred to v2 (needs conn-wrapper interception).
- G1-8 External services: vendor manuals may go to an external LLM API;
  traffic logs never leave the fab;
  miner is local CPU only.
- G1-9 Profile key = (model, firmware range);
  site configuration is an instance overlay.
  Concurrency target: tens of VE instances per test host;
  design must not preclude hundreds.
- G1-10 Initial state: host's own connect-time setup (S2F33/35/37, S5F3) is the warm-up prefix;
  un-inferable pre-capture state is marked `unknown`, never guessed.
- G1-11 Semantic scope v1: full Instance State for S7 recipes (store, directory, PPGNT);
  S3/S4/S14/S16 replay + templated only.
  Evaluator hosts v1 = the equipment gateway + the FDC service (EAP oracle unavailable locally).
  Fact: the equipment gateway passes S3/S4/S7/S14/S16 through to the EAP opaquely;
  SECS-I supported in code but unused in any example/e2e config.
  (2026-09-27, G5-73: VE reads unmasked recordings as a privileged consumer;
  its derived models and profiles hold recipe content and inherit the same access control as the recordings.)
  (G5-73 superseded by V-26)

## 2026-09-26 grilling round 2 (owner accepted all recommendations)

- G2-12 Evidence source: EAP side already has traffic logs.
  Plan: design a searchable, indexable "traffic data pack" format, then write an EAP-log converter.
  Pilot capture recorder is not the first step any more.
- G2-13 EAP host for evaluation: Go host harness (HSMS active, direct to VE) first;
  a NATS client acting as the EAP, into the equipment gateway, second;
  real EAP if the environment allows.
- G2-14 Pilot tools: ≥1 of the tool type that already has FDC fixtures, ≥1 with S7 recipe flow, optional third = quirkiest vendor.
  Owner to name them.
- G2-15 Manual extraction: Claude API, per-page PDF with page citations.
  Floor: 50-row human sample, ≤2 errors;
  RCMD parameters and processing-state table 100 % human-reviewed.
- G2-16 Budgets (frozen for G4): onboarding ≤ 2 person-days/tool;
  VE instance ≤ 0.5 core, ≤ 256 MB steady;
  miner ≤ 30 min/GB single machine;
  ≤ 2 extension hooks per Profile.
- G2-17 Deployment: runtime is natively multi-instance in one process;
  containers are packaging only.
- G2-18 Log volume assumption: ≤ 1 GB/tool/day;
  Profile window 2–4 weeks;
  miner incremental by transport epoch.
- G2-19 Repo: `github.com/arloliu/veq`;
  Python offline tools in `tools/miner/` (uv).
- G2-20 Data-pack requirements (owner):
  space-efficient (compressed), indexable and searchable, suitable for long-term blob storage (e.g. S3), troubleshooting-friendly.
  Placement decision pending (round 3).

## 2026-09-26 grilling round 3 (owner accepted all recommendations)

- G3-20 EAP log (assumed until sample seen): SML text;
  has direction, µs timestamps, SystemBytes, DeviceID, W-bit;
  has transport events (connect, Select, T3 timeout).
  (superseded by V-3)
  (also in the tracepack decision log)
- G3-PLACE Data-pack format + codec live in go-secs as a nested module (own go.mod), e.g. `github.com/arloliu/go-secs/tracepack`;
  reference CLI in the same nested module (`tracepack/cmd/tracepack`), exempt from go-secs "no binary" rule.
  VE repo holds the EAP converter.
  (also in the tracepack decision log)

## 2026-09-26 grilling round 4 (owner accepted all recommendations) — frontier empty

- G4-28 Evidence grades:
  `reconstructed` (from EAP SML via converter) is accepted for semantic-layer learning (catalog, reply rules, timing, semantic quirks);
  transport-layer and malformed-frame quirks require `observed`.
  Pack header field `evidence: observed | reconstructed`
  (this field superseded by V-7).
  Owner note: raw-byte capture can be added inside the equipment gateway —
  either in its NATS-facing EAP adapter or as a new `secs-recorder` device on a shadow channel (gRPC etc.).
  Caveat recorded:
  a shadow-channel recorder sees decoded messages (byte-exact re-serialisation for well-formed frames only);
  truly observed malformed frames need a connection-level interceptor (go-secs WithListener wrapper).
  Both writers emit tracepack.
  (also in the tracepack decision log)
- G4-30 Next steps in parallel: (a) tracepack format spec → external review;
  (b) the v3 synthesis (since removed) integrating rounds 1–4 and review v2 P1s → external review.
  Specs stay in the local working area until accepted,
  then move to go-secs docs/specs/ (tracepack) or the veq repo (VE).
- G4-31 Profile schema: YAML + JSON Schema for structure;
  semantic validation in the Go Spec Compiler.

## 2026-09-26 after review round 3 (owner)

- R3-1 Fixture (c) oracle: ask the FDC service side for actual missing-END / duplicate-START behaviour;
  fixture expected outcome is whatever the FDC service does today, frozen as `known`.
- R3-2 Fixture (d) oracle: owner unsure → design default:
  one report with three sentinel SVIDs in a deliberately permuted order vs the configured seq_path;
  expected outcome = the FDC service attributes values by position (documented behaviour),
  so the harness asserts the mis-attribution is visible in the uploaded value names.
  Revisit when the FDC service side confirms.
  (superseded by V-9)
- R3-3 Generic-onboarding acceptance: design default K = 10 tool types after the pilot, pass rate ≥ 80 % with ≤ 2 hooks each.
  Placeholder for design purposes, not an implementation commitment.
- R3-6 Pilot tools and EAP sample: provided later.
  **Owner emphasis: the current goal is to clarify the HIGH-LEVEL design direction, not to implement.**
  Byte-level grammar, exact algorithms and test vectors belong to a later implementation-spec phase;
  the design docs should state contracts and invariants, and explicitly defer encodings.
  (also in the tracepack decision log)

## 2026-09-26 stopping rule (owner agreed)

- STOP-1 The synthesis/review loop ends
  when a review round's remaining P1s are all implementation-level (encodings, queue sizes, API shapes, algorithms).
  At that point the high-level design is settled;
  next phase = `veq` repo skeleton + implementation spec, not another synthesis.
- STOP-2 Two decisions surfaced with v4 for the owner's awareness:
  (a) all VE output via ForwardDataMessage (VE owns System Bytes, reply correlation, T3, S9F9);
  (b) v1 evaluator includes the equipment gateway + FDC service e2e stack,
  because fixtures (c)(d) are oracled on the FDC service's gRPC responses.

## 2026-09-26 STOP-1 reached

- `ve-architecture.md` (then 04-synthesis.md) v4.2:
  confirmatory review found no design-level P1 → HIGH-LEVEL VE DESIGN SETTLED.
- tracepack-spec.md v2.2: confirmatory review found no P0 and one residual P1 (lifecycle-event epoch attribution)
  → fixed by writing epoch=0 + correlation-incomplete unless the originating socket is independently established
  → HIGH-LEVEL FORMAT DESIGN SETTLED.
- Next phase (STOP-1): `veq` repo skeleton + implementation spec;
  tracepack moves to go-secs `docs/specs/` and nested module `tracepack/` when implementation starts.

## 2026-09-27 redaction (question Q3, proposal P6; owner agreed, grilling rounds 1–3)

- G5-73 VE reads unmasked data as a privileged consumer (G1-11 unchanged);
  its derived models and profiles inherit the same access control.
  Recorded at G1-11, not in the tracepack spec.
  (superseded by V-26)
  (also in the tracepack decision log)

## 2026-10-09 pilot sample and re-derivation (owner)

- V-1 Terminology: the host-side automation program is called EAP (equipment automation program) in all veq documents and code;
  the earlier site-specific name is retired.
  The converter is `cmd/eapconv`.
- V-2 De-identification: veq documents carry no company, site, tool-ID, internal-service or person names and no local paths;
  vendor manual content is not quoted;
  sensitive data lives only in the gitignored `data/` tree.
- V-3 The pilot EAP log sample contradicts the G3-20 assumption:
  it has no transport events (connect, Select, T3 timeout);
  timestamps resolve to 0.1 ms and are the EAP's log times, not wire times;
  the EAP's transaction id equals the System Bytes only for equipment-initiated primaries;
  a logged reply header can carry its primary's stream and function;
  bodies of some messages are logged encrypted;
  the EAP also logs non-wire messages it builds itself.
  G3-20 is superseded by V-3.
  (The transaction-id statement is superseded by V-8.)
- V-4 Re-derivation: the owner re-opened the whole VE design, including the pilot gates G0–G4,
  whose definitions were lost with the removed v3 synthesis.
  All earlier entries stand as a record but bind nothing until re-confirmed by a later entry;
  `ve-architecture.md` returns to `draft`.
- V-5 Document lifecycle: veq follows `.agents/rules/450-doc-lifecycle.md`, adopted from go-secs with additions:
  one-clause Status lines, only the current phase of a plan detailed,
  an `active` plan untouched for more than 21 days fails `make docs-check`,
  a `done` phase needs a commit or PR reference,
  and `docs/ROADMAP.md` is the single index of active plans.
- V-6 tracepack dependency: veq pins `github.com/arloliu/go-secs/tracepack` at a pseudo-version of go-secs main (currently 146846a), not v0.1.0;
  (the pin is superseded by V-14)
  System Bytes, Session ID and W are taken only from a logged header the dialect proves to be the wire header;
  non-wire and unparsable entries become annotations, never dropped.
- V-7 Evidence provenance: veq reuses the tracepack provenance fields instead of a VE-specific enum:
  `capture_method` and `vantage` per pack, `fidelity` per record.
  The grades `observed` (`wire-exact` or `re-encoded`) and `reconstructed` are views derived from `fidelity`, not stored.
  Transport-layer and malformed-frame evidence requires `wire-exact` records, whatever the pack's capture method.
  The pack header field `evidence` of G4-28 is superseded; G4-28's grade rules stand.
  Field names confirmed with the go-secs session; they are identical at 146846a and on its next branch.

## 2026-10-10 log dialect corrections

- V-8 Whether the EAP's transaction id equals the logged System Bytes depends on the EAP version, not on the initiating side:
  on one pilot tool it does for every message, on the other for none.
  The equipment gateway keeps System Bytes and rewrites only the session ID.
  Event and alarm reports carry no W-bit because the tools are configured not to ask for replies, not because the gateway acknowledges them.
  One EAP log can carry several equipment connections, so `cmd/logcat` separates them.
  Details are in `docs/specs/eapsml/eapsml-dialect.md`.

## 2026-10-10 FDC fixture oracles (answers from the FDC service and equipment gateway maintainers)

- V-9 The FDC service never sees raw SECS: the equipment gateway decodes each S6F11,
  resolves every configured value by position within the report of the configured RPTID,
  and sends named values, which the FDC service accepts by name without checking RPTID, value count or format.
  Fixture (d) therefore targets the gateway's report-value resolution, observed at the FDC service;
  its expected outcome is unchanged: the permuted values arrive under the configured names.
  R3-2's attribution of positional mapping to the FDC service is superseded.
  Fixture (c), answering R3-1: duplicate START opens a parallel context; END without START is ignored;
  nothing in the FDC service ends a START without END,
  and the gateway aborts it as a client timeout at its first sample tick after the task deadline.

## 2026-10-10 generation service (grilling rounds 1–4; owner accepted all recommendations)

- V-10 The VE generation service is designed as two learning loops,
  written up in `docs/specs/ve/proposals/p1-generation-service.md`,
  which feeds phases 2 and 3 of the re-derivation;
  pilot gates and budgets are re-derived in phase 2 (G2-16 figures as reference only).
  The VE model becomes concurrent timed state machines with synchronising transitions,
  and E87, E40, E94, E39 and E90 object machines become skeleton capabilities, reversing the architecture's non-goal.
  Profiles learn in four prior layers (standard, vendor/family, tool type, tool delta);
  the outer loop minimises onboarding cost and selects automatically only among declared alternatives.
  G1-8 is re-confirmed: logs stay in the fab, LLMs are advisory, promotion is deterministic, no probing of production tools.

## 2026-10-10 generation service review (owner)

- V-11 Two choices made while revising the generation-service proposal after its first external review:
  - Onboarding cost has two dimensions, evidence and person-hours, and no weighting between them:
    the outer loop adopts an alternative automatically
    only if neither dimension gets worse beyond its tolerance and at least one improves;
    every other outcome goes to a person.
  - The release rule for log-derived content leaving the fab is set in the proposal now, not deferred:
    released and withheld categories, unclassified content withheld,
    a person the owner names approves new categories and templates, and every release is recorded.
- V-12 Storage for the generation service:
  artifacts (Profiles, tool deltas, suites, lineage manifests) are content-addressed files
  in the fab's S3-compatible object storage (MinIO);
  registry records go to a transactional database already run in the fab, MariaDB by default,
  made append-only by denying the service updates and deletes.
  The lasting evidence source is a traffic log service that receives traffic from the equipment gateway
  and packs it as tracepack captures into the same object storage, for learning and troubleshooting.
- V-13 Proposal P1 (`docs/specs/ve/proposals/p1-generation-service.md`) is accepted
  after four rounds of external review, the last with no remaining finding.
  It becomes the input to phases 2 and 3 of the re-derivation:
  its §8 changes are applied to the living documents, and phase 2 sets the numbers it leaves open.

## 2026-10-10 tracepack v0.2.0 (owner)

- V-14 veq pins `github.com/arloliu/go-secs/tracepack` at v0.2.0 (format 1.0), the first release that fits veq;
  tracepack is pre-1.0, so the pin is exact and every upgrade reads its changelog.
  tracepack depends on go-secs v2, so veq moved from go-secs v1 to v2.6.0.
  V-6's rules on header-derived identities and annotations stand.
  - The EAP log converter writes one capture per equipment connection found in a log,
    each with its own tool and device identity; the rule that maps a log's device token or session ID to a capture is part of the dialect.
  - The traffic log service that packs gateway traffic into object storage lives in its own repository;
    veq only reads its captures.

## 2026-10-10 generation service placement (owner)

- V-15 The parts of P1 that its §8 gave no home in the architecture
  (evidence intake, onboarding a new type, thin evidence, the outer loop's objective and autonomy,
  the registries and their storage, the initial hypotheses, governance and the release rule)
  move to a new living spec, `docs/specs/ve/ve-generation-service.md`, so they outlive P1 when it closes.
  The architecture keeps the Profile model, prior layers, learner contract, scores, suites and promotion gate.

## 2026-10-10 multi-capture conversion (owner, via the tracepack maintainers)

- V-16 A converter run over a log of several equipment connections follows tracepack specification 2.29 (go-secs G5-186),
  which extends V-14's one capture per equipment connection:
  each capture holds only its own connection's lines;
  lines of no connection go into one more capture whose tool ID is the EAP's configured identity, distinct from every equipment's;
  coverage is counted over the whole run, with no byte in two captures.
  The pin stays at v0.2.0 until a release ships 2.29.
  Details are in `docs/specs/eapsml/eapsml-dialect.md` §5.

## 2026-10-10 EAP log converter rules (owner)

- V-17 The owner confirms the rules `cmd/eapconv` fixed where neither tracepack nor the dialect decided them,
  as `docs/specs/eapsml/eapsml-dialect.md` §5 states them:
  - tool IDs are operator configuration, and a connection without one fails the run before anything is written;
  - a message whose connection is unknown goes to the EAP's capture as a data record;
  - an invalid W in the body line's text is `parse-failed`, an invalid W from the header alone is `build-rejected`;
  - a source reference is the file's base name and the SHA-256 of its content, never a local path;
  - an entry without a parseable time takes the previous entry's time,
    the first entries of a run the first parseable time, and both are marked `ordering-uncertain`.

## 2026-10-10 pilot connections (owner)

- V-18 Of the connections in the pilot logs, the VE simulates the tool and its load-port controller.
  The third peer, which exchanges stream 99 with the EAP, is labelled an FDC connection and left out for now;
  the converter still writes its capture, as it writes every connection's.
  Details are in `docs/specs/eapsml/eapsml-dialect.md` §3.

## 2026-10-10 converter output and identities (owner, via the tracepack maintainers)

- V-19 `cmd/eapconv` follows tracepack specification 2.31 (go-secs G5-192 and G5-193), which answers the two questions V-16 left open:
  - a converter writes generation-0 segment packs, one capture and one UTC hour each, outside the recorder's flush and recovery contract;
  - a re-run over the same files with the same settings reuses its capture IDs,
    derived from each source file's content and every setting that interprets it,
    and reuses a pack ID only for byte-identical output, derived from everything the pack's bytes depend on;
  - a pack whose key already holds different bytes fails the run and is never overwritten.

  G5-192 replaces G5-186's one capture ID per tool per run with one per tool per source file set per logical conversion.
  Details are in `docs/specs/eapsml/eapsml-dialect.md` §5.

## 2026-10-10 pending decisions (owner accepted the recommendations)

- V-20 A `correlation-incomplete` record can carry a semantic quirk.
  A converted EAP log has no transport events, so tracepack writes every record in epoch 0 and marks it `correlation-incomplete`,
  which under the architecture's earlier quality-flag rule left no log record eligible as quirk evidence, contrary to G4-28.
  The flag bars only a deviation whose claim rests on pairing a reply with its primary or on the transport epoch;
  a deviation read from single records or their order stays a candidate, under G1-6's human review.
  An inferred-epoch extension to tracepack is reconsidered once multi-day logs show whether connection starts are visible;
  it would be a separate field, since tracepack's `epoch` names an observed socket.
- V-21 Traffic of another peer on the tool's connection is excluded by the Profile, not by the converter.
  On one EAP version the FDC peer's stream 99 messages carry the tool's session ID (V-18),
  and the converter keeps every byte in the capture of the connection that carried it.
  The tool-type layer of a Profile declares such traffic foreign;
  learners skip it and the Evaluator does not score it.
- V-22 Where the standards behind the skeleton disagree or leave a value undefined
  (e.g. E40 STOPPED and ABORTED codes, the E90 SKIPPED code, E87 CARRIERACTION strings),
  the rule starts `unresolved` at the narrowest prior layer its evidence supports:
  standard, vendor or family, tool type, or tool.
  The vendor manual and captures resolve it; an `unresolved` rule that affects scored behaviour blocks promotion.

## 2026-10-10 tracepack v0.3.0 (owner)

- V-23 veq pins `github.com/arloliu/go-secs/tracepack` at v0.3.0, which implements specification 2.31,
  so the version the converter's packs declare matches the rules it follows (V-16, V-19).
  V-14's rules on exact pins and changelog reading stand; the v0.3.0 changelog has no change veq must adapt to.
  `cmd/eapconv` drops its own no-overwrite wrapper for the directory sink's `DirSinkNoReplace` option (go-secs G5-197),
  whose hard-link publication is safe against a concurrent writer, as the wrapper's check-then-rename was not.
  Capture and pack IDs hash the converter and tracepack versions, so converted output is regenerated with the new build.

## 2026-10-10 converter output and standard revisions (owner accepted the recommendations)

- V-24 During the pilot, converted output stays as generation-0 segment packs in a local directory, and nothing merges it.
  Learners read segments through listing views, which tracepack allows, and the pilot's few segments per capture gain nothing from an archive.
  Merging, its catalog and the deletion of merged segments come with the traffic log service and the fab's object storage (V-12).
- V-25 The standards corpus the skeleton was distilled from is enough for now;
  E157 (module process tracking) and the older object-standard revisions a tool's manual claims are not acquired yet.
  The pilot tool type's manual documents its own E157 implementation (state model, events and variables),
  and the pilot logs carry none of those events or variables.
  E157 is acquired when an EAP enables its events or a tool type depends on it with a thinner manual;
  an older revision when a behaviour cannot be resolved without it.

## 2026-10-10 sensitive content and FDC polling in captures (owner, after an external review and the FDC service maintainers' answers)

- V-26 The VE trains on masked captures with keyed digests, not on sensitive content in the clear; G5-73 is superseded.
  Masked captures keep structure, item types, lengths and, through digests, equality across messages,
  which serve message shapes, machines, timing and identity relations.
  They do not serve value-dependent rules or the return of a recorded sensitive value, so the decision holds under these conditions:
  - every sensitive field is classed as equality-only, grammar-projected (e.g. a PPID's character classes and length),
    numeric or predicate-bearing, template-bearing, or deployment-provisioned;
  - values compared across message types share one digest domain;
  - the Instance Overlay provisions every initial sensitive value the VE must return,
    such as the recipe directory and bodies a tool holds before a test,
    and ranges and predicates come from the manual or the overlay, not from captures;
  - a protected scorer that takes no part in learning compares the VE's sensitive outputs with recorded digests and returns only the results;
    learners never hold the digest key;
  - capture-time encryption of sensitive fields with a digest (the tracepack's deferred G5-74), replacing store-in-full,
    is the tracepack maintainers' design to make; veq asks for it but does not depend on it.
- V-27 The FDC service's interval polling (`S1F3`/`S1F4`) is thinned at the equipment gateway, not dropped.
  The FDC service's own sample store cannot replace it:
  it keeps only named values converted to numbers or text, one gateway timestamp per sample, inside processing contexts,
  with no variable IDs, item types, failed polls or raw event reports, and its offline copy is best-effort.
  The gateway keeps every poll's header and time, every abnormal reply in full (`S9Fx`, timeouts, empty or zero-length items),
  and a declared sample of normal reply bodies; the sample store is a secondary source of value trajectories.
  This needs a tracepack record that omits its body and a capture-level declaration of deliberately filtered or sampled traffic,
  which learners treat as a known unobserved source (architecture §6).
  The sampling rule is set when the FDC configuration (polling interval, variable count) is known.
