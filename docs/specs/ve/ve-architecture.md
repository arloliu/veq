# Virtual Equipment architecture

Status: draft (2026-10-09) — v4.2 baseline plus the accepted generation-service changes; under re-derivation

The owner re-opened the whole design on 2026-10-09 because v4.2 rested on assumptions made without pilot data.
This text is the starting point for that re-derivation, not a settled design.
Parts whose source was a removed revision are marked **open** with the re-derivation phase that settles them;
the phases are tracked from the [roadmap](../../ROADMAP.md).
The changes of the accepted generation-service proposal are applied ([ve-decisions.md](ve-decisions.md) V-13).

Scope of this document: what the VE is, which objects exist, which contracts hold between them, and how success is measured.
Encodings, sizes, algorithms and API shapes are named as open implementation items, not specified.

Terminology: `H->E` host to equipment; `E->H` equipment to host; VE = Virtual Equipment.

---

## 1. Purpose, goals, non-goals

Produce and run a Passive-side simulator of a semiconductor tool (SEMI E5/E30/E37.1 in v1)
so that host software (the EAP through the equipment gateway, and the FDC pipeline) can be developed and regression-tested without a production tool.

Goals as testable targets:
1. **Fidelity that improves with evidence**:
   a promoted Profile passes all applicable mandatory fixtures, meets absolute floors,
   and does not regress any scorecard dimension beyond tolerance (§8, §9).
2. **Latency, instability, quirks**: observed per-tool timing distributions; observed semantic-layer deviations;
   authored fault overlays.
   Transport-layer deviations are v2.
3. **Virtual clock**: VE-internal logical time; host-facing timing is wall time in live mode.
   With a continuously polling host there is no compression in live mode (§10).
4. **Generic generation**: tool types beyond the pilot onboard mostly with Profile data and few extension hooks;
   the population target is open (phase 2);
   R3-3's K = 10 / 80 % binds nothing until re-confirmed ([ve-decisions.md](ve-decisions.md) V-4).

Non-goals: process physics; conformance certification; host-side virtual time;
SECS-I binding in v1 (interface abstract).
The object standards (E87, E40, E94, E39, E90) are skeleton capabilities, not non-goals ([ve-decisions.md](ve-decisions.md) V-10):
the standard is their prior, and the vendor manual and logs supply the delta.

---

## 2. Architecture

```
 vendor GEM manual ─► Spec Compiler ─► cited assertions ──┐
 EAP SML logs ─► converter ─► tracepack (reconstructed) ──┤ Evidence Store (§6)
 traffic log service ─► tracepack (observed) ─────────────┘        │
                                                                   ▼
  outer loop ── chooses learners and priors ───────► learners (§7) ─► candidate artifact
  experiment registry                                              │
        ▲               ┌──────────────────────────────────────────┘
        │               ▼
        └── results ── Evaluator (§8) ─► promote ──────────────────────► Profile Registry (§9)
                        ▲                                                         │
                ┌───────┴─────────────────────────────────────────────────────────▼─────────────┐
                │  VE Runtime (Go, many instances per process)                                  │
                │  capability library + reference model (§5)                                    │
                │  Profile · tool delta · Instance Overlay · Instance State · Scenario (§3)     │
                │  event core with slots (§4.10) → outbound adapter → transport adapter (§4)    │
                │  clock modes (§10)                                                            │
                └──────┬──────────────────────────┬────────────────────────┬────────────────────┘
                       ▼                          ▼                        ▼
               go-secs Passive HSMS-SS     in-process fake endpoint    gateway plugin (later)
```

Two governing ideas:
**skeleton is code, tool delta is data** (the skeleton is an optional capability library plus a reference model, never an enforcer);
**Profile is the unit of evolution**, distinct from Instance State and Scenario, promoted only through a gate.

The generation service that produces Profiles runs two learning loops ([ve-decisions.md](ve-decisions.md) V-10, V-13):

- The **inner loop** learns one tool type:
  exchangeable learners (§7) turn evidence into a candidate artifact,
  the Evaluator (§8) scores it,
  and the gate (§9) promotes it into the **Profile registry** or rejects it.
- The **outer loop** learns which learners, priors and parameters work across tool types.
  It records its experiments in the **experiment registry**
  and selects automatically only among alternatives already declared there.

Both registries are append-only and stay in the fab.
[ve-generation-service.md](ve-generation-service.md) specifies the outer loop and the registries' storage.
It also covers onboarding new tool types and the release of log-derived content.

---

## 3. Core objects and their lifetimes

| Object | Contents | Owner |
|---|---|---|
| **Profile** (immutable per version) | identity; capability set; catalogs (SVID/ECID/DVID/CEID/RPTID/ALID/RCMD with format, units, range, standard-name mapping, DVVAL-valid-at-CEID); machines with guards and synchronising transitions, and slot components (§3.4); default reports/links/enables; reply rules and value generators; timing distributions; accepted quirk rules; pinned standard editions; layer and provenance per rule (§3.5); applicability (model, firmware range) | learners (§7), promoted by the gate (§9) |
| **Tool delta** (immutable per version, learned) | the rules of one tool over its tool-type Profile: enabled options, constant values, the effects of its EAP configuration; provenance per rule | learners (§7), promoted on its own tool's suites (§9) |
| **Instance Overlay** | site configuration: DeviceID, port, timers, timing scale, site EC defaults | test-env operator |
| **Instance State** | see the lifetime matrix below | VE Runtime |
| **Scenario** | external stimuli and *proposed* progression: operator actions, material, process-phase proposals, alarm triggers, fault overlays, trajectory references, clock mode | test author / replayer |
| **Run Journal** (append-only) | every core admission in admission order: inputs, transaction outcomes, lifecycle observations, outbound outcomes, diagnostics, with scheduler time and (live) wall time | VE Runtime |
| **Reproduction Bundle** (immutable after run) | launch inputs (Profile, tool delta, overlay, runtime, seed, initial checkpoint, Scenario, clock mode) + Run Journal + final checkpoint | VE Runtime |

### 3.1 Instance State lifetime matrix

| State group | Host reconnect | Equipment restart (Scenario) | VE process restart | Factory reset (Scenario) | Profile migration |
|---|---|---|---|---|---|
| Durable configuration: reports, links, enables, ECs, limits, alarm enables, recipe store, spool context | kept | kept; the Profile quirk `reports-volatile-on-restart` clears **only reports, links and enables** | kept (checkpoint) | reset to Profile defaults | kept if `state-compatible-with` else new instance |
| Processing execution: processing state, Scenario cursor, trajectory position, pending progression | kept | reset to Profile initial state | restored from checkpoint | reset | reset |
| Alarm set (current) | kept | reset | restored | reset | reset |
| Trace producers (definitions + sample counters) | kept; sampling continues (§4.3) | removed | restored (definitions + counter); a partial group in progress at shutdown is discarded (`trace-group-lost`) and the schedule restarts at the next DSPER boundary after restore, so no group spans an unrepresented gap | removed | removed |
| Event conversations in flight, outstanding transactions, T-timers | cancelled per conversation rule | cancelled | cancelled | cancelled | cancelled |
| Communications state | → NOT COMMUNICATING | → configured initial | → NOT COMMUNICATING | → configured initial | → configured initial |
| Control state (OFF-LINE / ON-LINE LOCAL / REMOTE) | kept | → Profile initial control state (E30 §3.3 transition 1) | restored | → Profile initial | → Profile initial |
| Random stream | continues | continues | restored from checkpoint (stream position) | continues | restarts |
| Object machines (carriers, process jobs, control jobs, substrates) | kept | open (phase 3) | restored from checkpoint | open (phase 3) | open (phase 3) |
| Slot component state (§3.4) | continues | open (phase 3) | restored from checkpoint | open (phase 3) | restarts |

Checkpoint = durable configuration + processing execution + alarm set + trace producers + object machines + slot component state + random-stream position + scheduler time + variable-history window.
Transport transactions are never checkpointed.
The configuration-commit boundary is "reply written" (§4.5): a host change is durable once its ACK reply is on the wire.

### 3.2 Authority rule
Scenario proposes; Runtime decides.
Every stimulus (Scenario or host) passes the Profile guards of the target machine.
Guards are sensitive to **origin**: operator stimuli are not subject to LOCAL-mode host restrictions; host commands are.
Outcomes are one of:
- `performed` (state changed now),
- `accepted-pending` (state will change later; HCACK=4 is this outcome, E30 §4.4),
- `rejected` (HCACK 1/2/3/5/6, or guard failure for Scenario stimuli → `scenario-rejected`),
- `stalled` (Scenario waits for a guard with a timeout → `scenario-stalled`).

A trajectory starts only on `performed` or on the completion event of an `accepted-pending` command.
STOP/ABORT end the trajectory and cancel dependent scheduled work (§4.7).

### 3.3 Variable history and trajectories
Per-variable timeline so S1F3, S6F1, S6F11 and S6F19 agree.
Values come from a Scenario trajectory (conditioning input, labelled as such) when present, else from generators (§7.3).

### 3.4 Machines and slots
A Profile is a network of **concurrent extended timed state machines** joined by **synchronising transitions**,
over a **shared variable store** (status variables, data variables, equipment constants),
plus reply rules, value generators and timing distributions ([ve-decisions.md](ve-decisions.md) V-10).

- **Machines**:
  the GEM machines (communications, control, alarm, limit monitoring, spooling; [ve-gem-skeleton.md](ve-gem-skeleton.md)),
  the processing machine, whose states and transitions are tool-type data with E30's example model as the standard prior,
  one machine per equipment module,
  and **object machines** instantiated per object:
  carriers (E87), process jobs (E40), control jobs (E94) and substrates (E90), with E39 object services.
- **Synchronisation**:
  a transition in one machine can require or trigger a transition in another,
  e.g. a job step that moves a substrate and fires events in two modules.
- **Slots**:
  two parts may be filled by components that are not state machines:
  the choice among concurrently enabled transitions (interleaving) and value generation.
  A slot component chooses only among transitions the guards allow (§3.2),
  writes values only through the variable history (§3.3),
  and keeps its state in Instance State, so it is checkpointed (§3.1) and replayed (§10) like every other part.
  Everything outside the slots stays an inspectable machine, so conformance can still be reported.

Each message type has a place in this model:
`S1F3` and `S2F13` read the variable store;
`S6F11` is the output of a transition, labelled by its event ID and carrying a snapshot of variables;
host commands (`S2F41`, `S3F17`, `S16F15`) are inputs that a guard accepts or rejects before later events follow;
process progress is a spontaneous transition driven by the Scenario and the clock;
alarm text is a template filled with live values.
Splitting events per object (which substrate, which chamber) needs the report definitions,
to know which report value is the object ID.

### 3.5 Prior layers
Every rule belongs to one layer and records its provenance:

| Layer | Holds | Learned from |
|---|---|---|
| Standard | GEM and object-standard behaviour | SEMI standards, distilled |
| Vendor / family | conventions shared by one vendor's tools, e.g. ID numbering, manual structure, common quirks | several tool types of that vendor |
| Tool type | catalogs, machines, reports, timing (key: model, firmware range) | the vendor manual and logs of the type's tools |
| Tool | the delta of one tool: enabled options, constant values, its EAP configuration's effects | that tool's logs |

A lower layer overrides a higher one only with evidence.
Logs of a type's tools train its tool-type layer together.
The tool layer is the **tool delta**, a separate versioned artifact beside the Instance Overlay;
the runtime loads a Profile, an optional tool delta and the overlay,
and resolves them by the precedence in §5: the overlay first, then the most specific active layer.

---

## 4. Event core, outbound adapter, transport adapter

go-secs citations in §4 are to v2.6.0, the version veq pins;
the passive-side spike verified them ([ve-decisions.md](ve-decisions.md) V-28).

### 4.1 Inputs and their observation source

| Input | Source | Reliability |
|---|---|---|
| Host primaries and orphan secondaries | go-secs data channel | complete for decodable frames |
| Secondaries to VE primaries | **VE-owned correlation** on the data channel (§4.5) | complete |
| Connection transitions (accept, Selected, Separate, T7, I/O error, local close) | lifecycle subscription (go-secs `hsms/lifecycle.go`) | latest state guaranteed; intermediate transitions may coalesce; each event carries `Socket` and `Generation` (go-secs `hsms/lifecycle.go:92,99`) |
| Epoch identity | go-secs `Generation`, stamped on every wire, socket, lifecycle and transaction event | complete; the epoch id is the selected session's `Generation` |
| T6 on passive side | not applicable in v1: VE-initiated Linktest is disabled (§4.5) | — |
| T8, read errors | lifecycle `CauseIOError` | not distinguishable |
| Linktest/Select/Reject frames | **observed** by the wire observer (§4.4) as evidence, for the Select.rsp gate (§4.5) and for peer rejections (§4.2); not intercepted (interception is v2) | observed, not consumed by protocol logic except the gate and rejection correlation |
| Scenario stimuli, scheduler timers | in-core | complete |

### 4.2 Outputs and completion signals

| Output | Path | Completion |
|---|---|---|
| Any VE frame: replies (function+1 or Sx,F0), VE primaries with W=1 or W=0, quirk frames | **`ForwardDataMessage` only** (go-secs `hsms/session.go:191`; synchronous write, returns once on the wire, registers no local reply wait) | `written` on return; `failed(err)` otherwise |
| Diagnostics, state changes | in-core | immediate |

Design decision: the VE never uses `SendDataMessage`, `SendDataMessageAsync` or `ReplyDataMessage`.
`ReplyDataMessage` is bound to the connection its primary arrived on
and fails after a reconnect (go-secs `hsms/session.go:242`),
which would make the epoch fence unnecessary for plain replies;
it was considered and not adopted for v1, because it is asynchronous, so it has no `written` signal,
and it builds only function+1 with no W-bit, so it cannot carry Sx,F0, quirk or S9Fx replies.
Known limitations of this path:
go-secs still counts forwarded frames in its data-send/error counters
and reports `TxSent` with `ReplyWaited=false` to a transaction observer, but never the VE's reply/T3 outcome,
so transaction-level metrics are VE-owned;
the VE's pending T3 does not feed go-secs's inflight-based linktest suppression;
forwarded data outside SELECTED is refused (`ErrNotSelectedState`)
and inbound data outside SELECTED never reaches the core,
so §4.1 completeness holds only within SELECTED;
a peer Reject.req with no registry entry is counted and discarded by go-secs (go-secs `hsmsss/transport_recv.go:201,231`),
but the wire observer (§4.4) sees it,
so the core correlates it by System Bytes and ends the VE transaction as `rejected` instead of at T3.
Consequences:
(a) every output has a real `written` signal;
(b) the VE core owns reply correlation and T3 for its own primaries (a scheduler timer, virtual in offline mode);
(c) go-secs automatic S9F9 on T3 (`WithEquipRole`) never fires for VE transactions,
so S9Fx are VE outputs governed by the Profile;
(d) go-secs transaction metrics do not see VE transactions, which is acceptable.
Malformed quirk frames are limited to what `DecodeHSMSMessage` accepts (length, PType, SType checked, go-secs `hsms/decode.go:41`);
Sx,F0 is built normally with W=0.

### 4.3 Conversations
Instance State objects with state machine, timers, cancellation and dependencies.
- **Event report**: optional `S6F5 → S6F6` per Profile `event-inquire-grant: always | size-based | never` (default `never`).
  GRANT6 per E5 Table 3: `0` proceed; `1` busy → retry after `grant-retry-delay` up to `grant-retry-max`, then terminal;
  `2` and `>2` terminal.
  The event owns an **immutable snapshot** (layout, order, values) taken at event time;
  report redefinition affects future events only, never a created one.
- **Trace**: the producer samples at DSPER **regardless of communications state** (E30 §4.2.3.3);
  sampling never pauses, so SMPLN/STIME always denote real sample times.
  A completed group is a primary handed to the **common output policy**:
  the communications, control and spooling state machines decide whether it is transmitted (COMMUNICATING, not SPOOL ACTIVE),
  spooled (SPOOL ACTIVE and S6F1 spool-enabled and not OFF-LINE, E30 §4.11 and §3.3)
  or discarded (journal `trace-group-lost`).
  Trace has no private spool rule.
  `suspend` exists only as a labelled vendor quirk with journal `trace-discontinuity`.
  TOTSMP=0 cancels and removes the producer and its scheduled work.
- **Recipe transfer**, **establish communications** (both E30 §3.2 AND-substates; Profile `comm-init: host | equipment | both`),
  **spool unload**, **control state** (E30 Table 2 note 3 order chosen by Profile).
  Their GEM behaviour is in [ve-gem-skeleton.md](ve-gem-skeleton.md);
  their conversation rules (timers, cancellation, failure outcomes) are open (phase 3).

### 4.4 Transport adapter
Passive endpoint via go-secs; `WithListener` (go-secs `hsmsss/config.go:188`) stays only as the listener factory.
The VE observes the transport through go-secs's own observers, which replace the earlier VE conn wrapper:
- `WithWireObserver` (go-secs `hsms/connection_config.go:735`) reports every complete frame in both directions,
  byte-exact, with wall time, `Socket` and `Generation`;
- `WithSocketObserver` (go-secs `hsms/connection_config.go:793`) reports socket accept, refusal and close.

Observer contract: the callback runs synchronously on the I/O goroutine, so the VE copies the frame and returns;
the frame is valid only during the call;
the callback never sends on the connection;
an outbound frame is reported only after its write succeeded, with a time taken just before the write was issued.
The observers do not intercept or inject.
go-secs owns automatic transport responses
(Select.rsp, unconditional Linktest.rsp go-secs `hsmsss/transport_control.go:508`, Reject, T7 close,
and Deselect.rsp with status 0, go-secs `hsmsss/transport_control.go:570`, a documented deviation from E37.1);
the Profile records `transport-policy: go-secs-default`, and transport-layer deviations are `unsupported` in v1.
After a connection drops, the passive side listens again only after the reconnect backoff (100 ms by default),
so the VE sets `WithReconnectBackoff` deliberately; the gap is a transport-fidelity item.
Transport options: `WithSessionIDValidation` and `WithStrictReplyMatching` stay at their defaults (off);
the VE validates DeviceID itself
so that a wrong-DeviceID host message reaches the core and can be answered with S9F1 per Profile.

### 4.5 Outbound adapter
- One adapter goroutine per instance; a bounded intent queue;
  the scheduler never blocks on it (full queue → `failed(overload)` → §4.6).
- Intents carry: id, epoch id at creation, dependencies (intent ids that must be `written`), frame.
  Release order respects dependencies;
  a `failed` prerequisite fails its dependants with `failed(prerequisite)`,
  and the owning conversation applies its failure rule.
- Mandatory orderings are expressed as dependencies on `written`:
  S1F14 before COMMUNICATING is entered; S5F1 before its S6F11; S1F16/S1F18 before the control-state CE.
- **Select.rsp before unsolicited data**:
  go-secs commits Selected before it enqueues Select.rsp (go-secs `hsmsss/transport_control.go:299,321`),
  so the Selected lifecycle event is not proof the response is on the wire.
  Without a gate, an S1F13 sent on the Selected event
  reached the wire before Select.rsp in about 10 % of handshakes (V-28),
  and comparing the two events' times cannot order them.
  Invariant: after passive selection, the adapter releases unsolicited VE data (S1F13 in `comm-init: equipment | both`, events, alarms)
  only after the wire observer has reported the outbound Select.rsp whose `Generation` equals the Selected event's.
  Observation of control frames is v1 (the observer already sees every frame); interception stays v2.
- **Active session epoch vs sockets**: go-secs assigns a `Socket` id to every accepted connection,
  but only the socket it selects for the session becomes the **active epoch**.
  While a socket holds the session, selected or not, the passive listener accepts each extra socket,
  answers its Select.req with status 1 and closes it;
  the socket observer reports it as refused, and its frames carry `Generation` 0
  (go-secs `hsmsss/transport_passive.go:125,192,319`).
  Every admitted input carries the active epoch it was admitted under; intents inherit it.
- **Epoch fencing**: before each write the adapter compares the intent's epoch with the current active epoch;
  mismatch → `failed(epoch-ended)`.
  On any observed epoch change the adapter first fails every queued intent from older epochs, then resumes.
  `ForwardDataMessage` is not bound to the intent's origin:
  an intent built before a reconnect and forwarded after it goes out on the new connection.
  go-secs binds the write to the epoch it loaded at call time (go-secs `hsms/connection_send.go:632,108,125`),
  so a reconnect during the call fails the write rather than crossing sockets,
  except in the window between the adapter's check and go-secs loading the epoch.
  **Residual**: in that window an old VE primary or secondary can reach the new host connection as a valid frame.
  The adapter detects the window by re-reading the epoch after the write;
  on detection the run is journaled `possible-cross-epoch-write`
  and the Evaluator classifies it `invalid(infra)`, never as tool behaviour.
  The window only opens on a reconnect; its duration is not bounded by the design.
  An origin-bound write contract would need a go-secs change and is deferred.
- VE-initiated transactions: the core owns a **per-instance System Bytes generator** satisfying E37 §7.10.3
  (unique among the instance's open transactions and different from the last completed one),
  independent of go-secs's own generator (which only serves control messages once the VE never calls SendDataMessage).
  The core registers (epoch, DeviceID, SystemBytes, S/F) with a T3 scheduler timer;
  the peer's secondary arrives on the data channel as an orphan
  (go-secs's reply registry has no entry for it, so it falls through to the data handlers regardless of `WithStrictReplyMatching`,
  go-secs `hsms/connection_runtime.go:299,305`)
  and the core correlates it;
  T3 expiry → conversation rule and Profile-governed S9F9.
- **Connection-wide transaction identity**: go-secs's reply registry keys on System Bytes only,
  so a go-secs-initiated control transaction (automatic Linktest) that happens to share System Bytes with an open VE transaction would consume the VE's secondary
  (go-secs `hsms/reply_registry.go:112`, go-secs `hsmsss/transport_procedures.go:192`).
  v1 rule: **the VE never enables go-secs-initiated control transactions**
  (automatic Linktest stays at its default, disabled; Linktest from the host is answered by go-secs and needs no registry entry).
  The collision is reproduced in V-28's spike:
  with automatic Linktest on, a VE primary sharing the probe's System Bytes lost its reply to it.
  The VE's System Bytes generator also stays disjoint from go-secs's own, which starts at 1
  and still serves control messages such as a courtesy Separate on close.
  This is a documented v1 limitation; a coordinated generator is the v2 alternative.

### 4.6 Admission and overload
- Bounded admission queue (default sized so that it never fills under the pilot's host load; measured in O4).
  If it fills, this is a **VE resource failure, not tool behaviour**:
  the instance journals `overload`, closes the connection, and the Evaluator marks the run `invalid(infra)`.
  No S9F11, no spool interaction.
- Deferred events (beyond `max-open-events`) are queued in the event conversation queue;
  E30 spooling activates only on its own conditions (E30 §4.11).

### 4.7 Scheduled work versioning
Scheduled actions reference the *object instance* they serve (event with its snapshot, trace producer generation, processing-state generation).
Object removal drops its actions at dispatch.

### 4.8 Unseen-input policy
Default `unsupported`: reply per E30/E5 defaults by state and cause —
NOT COMMUNICATING discards all but S1F13/S1F14;
OFF-LINE answers Sx,F0 except S1F13/S1F17;
otherwise unknown stream S9F3, unknown function S9F5, illegal data S9F7 (E5 §8.3) —
and journal `unsupported(S,F,reason)`.
`default` opt-in per message type; `hypothesis` stage G only.

### 4.9 Worked example (compact, normative at the level of ordering and state)
Full timestamped tables move to the worked-examples document (not in this repository)
with the implementation spec ([ve-decisions.md](ve-decisions.md) R3-6);
earlier timestamped tables were withdrawn because their trace timeline and completion rows contradicted §4.2/§4.3.
The compact example below fixes the orderings that earlier rounds got wrong.

Profile: `comm-init: host`, `event-inquire-grant: never`,
trace policy default (continuous sampling, groups lost when not communicating, S6F1 not spool-enabled).
Initial: COMMUNICATING, ON-LINE/REMOTE, EXECUTING; trace T1 {DSPER 1 s, REPGSZ 5, count 3}; CEID 901 enabled → RPTID 10.

| # | Input | Core decision | Output (all via ForwardDataMessage) |
|---|---|---|---|
| 1 | Scenario "wafer 2 start" | guard ok → `performed`; snapshot report 10 now; EV-901 opens, System Bytes minted by the core, T3 scheduler timer armed | S6F11 written |
| 2 | trace tick | count 4 | — |
| 3 | lifecycle → NotConnected | comm failure; EV-901 outcome `conn-closed` → cancel (journal `event-lost`); T1 keeps sampling | — |
| 4 | trace tick | count 5 → group complete while NOT COMMUNICATING → `trace-group-lost`; count continues | — |
| 5 | lifecycle → Selected with a new `Generation`: new active epoch; wire observer reports Select.rsp written for it | core: WAIT CR FROM HOST; unsolicited data gate opens only after the Select.rsp observation | — |
| 6 | S1F13 | intent R1 (S1F14) | S1F14 written → **then** COMMUNICATING |
| 7 | trace ticks ×5 | next group; SMPLN/STIME are real sample times, contiguous with pre-disconnect ticks | S6F1 written; its S6F2 arrives as an orphan secondary and is correlated by the core |
| 8 | S6F15 CEID 901 | report 10 persisted; DVVAL outside its event → zero-length item | S6F16 written |
| 9 | S2F41 START while EXECUTING | guard → `rejected` HCACK=2; trajectory unchanged | S2F42 written |
| 10 | S2F41 START after completion, Profile says completion later | `accepted-pending` HCACK=4; trajectory starts on the completion CE | S2F42 written, later S6F11 |

### 4.10 Slots in the event core
The core evaluates guards first and hands a slot component (§3.4) only the transitions they enable;
the component returns one of them and never adds a transition.
A value slot is consulted where §3.3 falls back to generators, and its value enters the variable history like any other.
A slot component draws randomness only from the instance's random stream,
and its choices are core decisions, so offline replay recomputes and compares them (§10).

---

## 5. Skeleton: capability library plus reference model
Known rules:
switchable capabilities;
reference model only for conformance reporting;
rule states `candidate | accepted | override | unresolved`;
precedence composed with the prior layers (§3.5) as below;
validation exception path for accepted nonconformance;
conformance and fidelity reported separately;
≤ 2 extension hooks;
`gem: false` path.
The capability boundaries and the validation exception path are open (phase 3).

**Which rule applies.**
A rule has a layer (its scope, §3.5) and a state (its authority).
They compose in this order:

1. Only rules in state `accepted` or `override` are active.
   `candidate` and `unresolved` rules never are,
   and an `unresolved` rule that affects scored behaviour still blocks promotion (§9).
2. The Instance Overlay comes first; it stays the operator's site configuration (§3).
3. Among active rules for the same behaviour, the most specific layer wins:
   tool delta, then tool type, vendor, standard.
   A lower-layer rule that contradicts a higher layer is a quirk, so it is active only once accepted on evidence.
4. Within one layer: override, accepted quirk, manual assertion, library default.

---

## 6. Evidence model
- **Provenance** uses the tracepack fields, not a VE enum ([ve-decisions.md](ve-decisions.md) V-7):
  `capture_method` per pack (`raw-stream | decoded-message | log | generator`),
  `vantage` per pack (`host | equipment | intermediary | network | none`),
  and `fidelity` per record (`wire-exact | re-encoded | reconstructed | synthesized | not-applicable`);
  each also has `unknown`.
- **Grades** are views derived from `fidelity`, not stored:
  `observed` is `wire-exact` or `re-encoded`; `reconstructed` is `reconstructed`.
- **Sources**:
  a raw recorder on the EAP's connection, in a relay or on the network writes `raw-stream` packs of `wire-exact` records;
  the equipment gateway after decoding writes `decoded-message` packs from vantage `intermediary`, records `re-encoded`;
  the EAP SML converter writes `log` packs from the vantage where the EAP logged (usually `host`), records `reconstructed`.
  The lasting source is the traffic log service, kept in its own repository ([ve-decisions.md](ve-decisions.md) V-12, V-14):
  the equipment gateway sends it the traffic it carries,
  and it packs that traffic as tracepack captures into the fab's object storage.
  Converted EAP logs are the source until it runs, and stay a second vantage afterwards.
- **Known unobserved sources**: every capture declares, beside its vantage,
  the traffic it cannot see, e.g. "traffic the gateway originates toward the tool".
  A learner must not read absence from a capture with a declared blind spot as absence of behaviour.
  Traffic a recorder filters or samples on purpose, such as the FDC service's polling bodies (V-27), is declared the same way.
- **Sensitive content**: learners read masked captures with keyed digests, never sensitive content in the clear (V-26);
  initial sensitive values the VE must return come from the Instance Overlay.
- Transport-layer and malformed-frame evidence requires `wire-exact` records, whatever the pack's capture method;
  a `decoded-message` recorder that keeps the raw bytes of frames it failed to decode also writes `wire-exact` records.
- **Scopes and where they live**: tracepack holds capture id, transport epoch and transaction identity
  (capture id, epoch, initiating direction, DeviceID, SystemBytes, primary record reference).
  Conversation, process-activity and evaluation-episode scopes live in a VE **Evidence Index** sidecar keyed by (capture id, record seq),
  the tracepack record identity
  (a pack id names a file, never a record, and segment packs are deleted after the hourly merge).
- **Quality flags** are structured record bits in tracepack:
  `capture-boundary`, `ordering-uncertain`, `correlation-incomplete`, `direction-inferred`;
  "malformed" is derived from `decode_status`
  (the tracepack specification, go-secs `docs/specs/tracepack/`, P9 §3.8 retired the `decode-failed` mirror bit).
  A deviation becomes a quirk `candidate` only from records without `capture-boundary` and `ordering-uncertain`
  that are not malformed;
  a deviation whose claim rests on pairing a reply with its primary or on the transport epoch
  also needs records without `correlation-incomplete` ([ve-decisions.md](ve-decisions.md) V-20).
- **Known foreign traffic**: traffic of another peer carried on the tool's connection,
  e.g. the FDC peer's stream 99 on one EAP version, stays in the tool's capture;
  the tool-type layer of the Profile declares it foreign, learners skip it, and the Evaluator does not score it (V-21).

---

## 7. Learners

### 7.1 Contract
Each part of a Profile or tool delta is produced by an exchangeable **learner** with one contract:

- **Input**: an evidence slice and the priors of every layer above the target layer (§3.5).
- **Output**: a fragment for one layer, each rule with its provenance, layer and a confidence.

The outer loop chooses which learner fills each part, per layer ([ve-generation-service.md](ve-generation-service.md));
that choice is possible only because learners are exchangeable and compared on the same suites (§8.5).

### 7.2 Learners per Profile part

| Profile part | Example learners |
|---|---|
| Catalogs (IDs, formats, names, units) | log catalog (`cmd/logcat`), manual extraction, identifier exports |
| Reply rules | per-message templates with echo and lookup |
| Value generators | the generator ladder (§7.3) |
| Machine structure within a module | passive automata learning with the standard and manual state models as priors |
| Synchronisation and interleaving | event-keyed module choice, object-machine products, sequence models in the slot (§3.4) |
| Timing | per-transition delay distributions |
| Quirks | deviation mining against the higher layers |

Rules recorded for two learners before the pilot,
which bind nothing until re-confirmed ([ve-decisions.md](ve-decisions.md) V-4):
the Spec Compiler (manual → cited assertions → validated candidate; floor: 50-row sample ≤ 2 errors, RCMD and processing tables 100 % reviewed);
the log miner (local CPU, incremental by epoch; candidates → accepted at ≥ 3 evidence across ≥ 2 captures plus human).

### 7.3 Value generators
Ladder: trajectory → state-derived → request-ordered lookup → persistent-state lookup → producer-inherited ids → literal echo → enum → counter → constant → bounded numeric.

### 7.4 Conflicts and provenance
Conflict resolution: `unresolved` blocks promotion when it affects scored behaviour (§9).
Every rule carries its provenance and layer.
Learner algorithms, the miner's stages and the candidate-to-accepted workflow beyond this summary are open (phase 3).

---

## 8. Evaluator

### 8.1 Scorecard
Responses, events, stateful behaviour, timing, quirks, coverage, conformance (separate).
Frozen scorable population per suite version;
first supported-path divergence is a failure;
coverage non-regression;
negative controls (a deliberately broken Profile must fail);
**absolute floors** per dimension fixed at G0 (design defaults: responses ≥ 95 % matched on the frozen population; mandatory fixtures 100 %).

### 8.2 Evaluators and execution paths
- Historical replay against recorded primary/secondary transaction pairs on a closed input tape (§10);
  its matching and scoring rules are open (phase 3).
- Host harness (Go, HSMS active) for EAP fixtures, directly against the VE.
- **FDC fixtures run through the equipment gateway and the FDC service in the end-to-end container setup from v1**,
  because their oracles are FDC service outputs;
  the "harness only" path is not sufficient for (c) and (d).

### 8.3 Mandatory fixtures with oracles

| Fixture | Stimulus | Target | Oracle (`known`) | Negative control |
|---|---|---|---|---|
| (a) START rejected | harness sends S2F41 START while Profile guard yields HCACK=**2** (cannot perform now) | EAP logic | host reports failure upstream and does not enter RUNNING within 10 s; VE processing state unchanged; a later START after the guard clears → HCACK=0 and trajectory starts | Profile that answers HCACK=0 must make the host enter RUNNING |
| (b) reconnect without rebuild | Scenario `equipment-restart` on a Profile with `reports-volatile-on-restart`; host reconnects | EAP logic | host variant that rebuilds S2F33/35/37 receives the next S6F11; variant that does not receives none; post-reset report/link/enable state frozen in the fixture | Profile without the quirk delivers the event to both variants |
| (c) missing END / duplicate START | Scenario emits START, START (same DC config), END; then END alone; then START with no END | the FDC service via the equipment gateway | per the FDC service's data-collection task handling ([ve-decisions.md](ve-decisions.md) V-9): duplicate START opens a **parallel** context (no deduplication; two ready-to-run tasks with distinct context ids); one END completes every active context whose STOP condition it matches, so both unless the condition separates them by context fields; END without START is **ignored** (empty ready sets, status OK, active set unchanged); START without END stays active (the service has no sweeper) until the **equipment gateway aborts it as a client timeout at its first sample tick after the task deadline** → a timeout-end data-quality value. A START whose values lack the material-ID attribute opens no context. Oracle: the FDC service's exact actionable-task-set responses and active-task membership at each step, and the gateway's timeout abort | a correct START/END pair yields one context and one completed-end result |
| (d) report position vs configured value order | one report with three sentinel SVIDs in permuted order vs the configured positions (`seq_path`) | the equipment gateway's report-value resolution, observed at the FDC service | the gateway resolves each configured value by position within the report of the configured RPTID and hands the FDC service named values, which the service accepts by name without checking RPTID, count or format (V-9); so the uploaded value names carry the permuted (wrong) values | correctly ordered report uploads correct values |

Applicability comes from a **frozen pilot capability matrix**, not from the candidate Profile;
each fixture family must execute at least once in the pilot.

### 8.4 Two scores and the scored population
- **Predictive score.**
  The learned model is asked for the probability of each held-out observable event,
  given the recorded history up to it (teacher forcing).
  It measures how well the model explains the evidence, and it is cheap; learner selection uses it.
- **Generated-run score.**
  The executable artifact runs in the deterministic runtime against a closed input tape built from the held-out capture
  (recorded host inputs and recorded transaction outcomes, §10),
  choosing its own transitions, and its outputs are compared with what the tool recorded (the historical replay of §8.2).
  It catches drift after the VE's first wrong choice, which teacher forcing hides.
  Promotion requires both (§9).
- **Scored population**: fixed by the suite, independent of any candidate:
  the observable events (`E->H` primaries with their stream, function and event ID) and replies in the held-out window,
  conditioned only on what precedes them in the capture and on the declared starting state.
- **Coverage**: a candidate must give every scored event a probability above zero:
  unseen IDs get probability through an explicit unknown-event allowance,
  so a model cannot score well by declining to cover hard events.
- **Timing** is scored per transition category the suite defines from observables (from one event ID to the next within a module),
  not per edge a candidate learned.
  Its acceptance rule is an effect size (the KS statistic within a tolerance), not a significance test;
  and it is always labelled with its vantage: EAP-log timing is host-side timing, never wire timing.
- **Metrics**:
  mean log-likelihood per held-out event (primary, predictive score),
  top-3 next-event accuracy,
  the timing statistic above,
  exact-match rate of replies and of generated events (generated-run score),
  and the rate of unseen IDs, as a signal of missing data rather than a quality score.
  They join the dimensions of §8.1, whose safeguards stay; no dimensions are summed into one score.

### 8.5 Suites and their roles
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
    at scoring time the runtime conditions only on its recorded host inputs and the declared starting state (§8.4).
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

### 8.6 Suite maintenance
- A suite version is frozen once used and is never edited.
- Each suite declares its applicability: the model, firmware range, options and EAP configuration it represents.
- A new version is cut when applicability changes (new firmware, options, configuration),
  when an assessment suite has been used up,
  or when new evidence makes the old one unrepresentative.
- Old versions keep their results.
  A suite whose scope is still supported keeps its regression obligation.
  Moving a trend to a new version requires running both versions on the same artifacts (an overlap), or recording a trend break.
- Who approves a new suite version is independent of the candidate it would judge.

---

## 9. Profile Registry and promotion gate
Known rules:
promotion requires absolute floors;
`unresolved` rules affecting scored behaviour block;
≥ 3 seeded runs for stochastic comparison;
every promoted version runnable.
Registry contents (versioning, rollback, applicability lookup) are open (phase 3).

- **What is promoted.**
  Every deployed effective artifact is immutable and reproducible:
  its layer versions are pinned or materialised,
  and a promotion never changes the behaviour of a version already promoted.
  This covers tool-type Profiles, learned tool deltas and the combination the runtime loads.
- **Gate.**
  Promotion is deterministic and reproducible and needs both scores of §8.4 above their floors.
  A learned tool delta is promoted on tool-split suites of its own tool (§8.5), against the tool-type Profile without it.
  A change to a higher layer is an outer-loop change:
  it produces new versions of the affected Profiles, must not regress any of them, and a person approves it.
- **Divergence sources.**
  Offline replay comes first; host test reports (failed fixtures, EAP developers) feed the same queue;
  live shadow comparison through the gateway is a later option.

The Profile registry and its storage are specified in [ve-generation-service.md](ve-generation-service.md).

---

## 10. Time model
- Clocks: scheduler time; equipment civil time (scheduler + skew/drift);
  transport wall time (go-secs timers; the VE's own T3 is a scheduler timer since §4.2).
- **Live-host mode**: scheduler time follows wall time.
  A logical jump is allowed only when the idle policy holds
  (no trace producer, no open conversation, no VE T3, no host primary in the last `idle-guard`, default 2 × max observed poll interval).
  **With a continuously polling host or an active trace there is no compression; this is stated, not worked around.**
  Reproducibility: seed controls VE choices;
  the Run Journal records admission order, which may differ from wire order
  (go-secs `hsms/connection_runtime.go:279`, go-secs `hsms/session.go:425`).
- **Offline/core mode**: the fake endpoint feeds a **closed input tape**
  (Scenario + recorded host inputs + recorded transaction outcomes);
  recorded timers and outcomes are injected from the tape, never regenerated;
  no real timers run.
  Determinism: identical launch inputs (Profile version, Instance Overlay, runtime version, seed, initial checkpoint, clock anchors)
  plus an identical ordered tape (host inputs, transaction outcomes, lifecycle observations, outbound write results, logical time jumps)
  → identical **canonical journal** (wall timestamps excluded).
  Recorded admissions are injected; derived decisions are recomputed and compared.
  Replaying a live run's journal in offline mode reproduces its core decisions.
- **Slot components** (§3.4, §4.10) run under the same closed-tape determinism:
  their state is in the checkpoint, their randomness comes from the seeded stream,
  and their choices are derived decisions, recomputed and compared like any other.

---

## 11. Provenance stages and measured fidelity
A rule's stage names where it comes from:
R replay (baseline) → T templated + Instance State → M manual-derived → C calibrated timing and accepted quirks
→ S learned structure (only on measured need) → G hypotheses.
A stage is provenance, never a fidelity claim.
Fidelity is only ever a measured result of the Evaluator (§8):
a Profile built from the manual alone is labelled *manual-derived* and carries no measured-fidelity claim
until it passes the gate of §9.

## 12. Tooling and repositories
`github.com/arloliu/veq` (runtime, schema, evaluator, registry, harness, converter at `cmd/eapconv`, `tools/miner/` Python);
`github.com/arloliu/go-secs/tracepack`, a nested module of go-secs (format, codec, CLI; [ve-decisions.md](ve-decisions.md) R3-4);
the traffic log service in its own repository (V-14).
No go-secs core change is a pilot prerequisite.

## 13. Pilot gates
The definitions of gates G0–G4 were lost with a removed revision
and are open (phase 2, [ve-decisions.md](ve-decisions.md) V-4).
The constraints below were recorded against them;
they are inputs to phase 2 and bind nothing until re-confirmed:
G0 freezes fixture stimulus/target/oracle/deadline/applicability/negative control and absolute floors;
G3 predeclares a ceiling case
(if the R baseline already meets the timing target, G3 passes on quirk applicability or is N/A with justification);
**G4 requires a successful promotion** and onboarding of one unseen type within budget;
the K = 10 / 80 % population is evaluated after the pilot as a separate milestone.

## 14. Open implementation items (deferred by R3-6)
O1 sync-write throughput at tens of instances;
O2 wire-observer timestamp semantics;
O3 checkpoint encoding;
O4 queue sizes;
O5 worked-example tables (the worked-examples document, not in this repository);
O6 Evidence Index sidecar schema.
