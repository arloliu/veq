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

