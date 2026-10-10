# What the VE requires of the traffic log service

Status: draft (2026-10-10) — the VE's requirements as a reader; open items wait on the gateway owners and the owner

The traffic log service is the lasting evidence source of [ve-architecture.md](ve-architecture.md) §6:
the equipment gateway publishes the traffic it carries,
and the service packs it as tracepack captures into the fab's object storage.
It lives in its own repository, `tracekeep` for now, and is designed there ([ve-decisions.md](ve-decisions.md) V-12, V-14, V-29);
this document is its input from the VE side, not its design.
What its captures supply and cannot supply as evidence is in [ve-evidence.md](ve-evidence.md) §3.

Decisions `G5-n` and proposal P8 belong to the tracepack specification (go-secs `docs/specs/tracepack/`),
and `[STO §n]` is a section of its storage profile.
veq pins tracepack v0.3.0 (V-23);
the rulings cited below come with v0.4.0 (G5-201) and v0.5.0 (G5-200, P8, and format 1.1 for G5-207 and G5-208).

---

## 1. Who owns what

| Part | Owner | Settled in |
|---|---|---|
| Producer: observes the tool connection, assigns capture ID and seq, stamps times and epochs, publishes one record per bus message (NATS JetStream) | the equipment gateway's project | G5-200; the bus message in P8 |
| Bus record, capture descriptor, consumer contract, segments, archives, merge and the deletion condition | tracepack | [STO §2]–[STO §4] |
| Consumers that write segments, merging, catalog, retention, deletion, recipe-body encryption and its keys | the traffic log service | G5-201, G5-207 |
| Reading captures, the Evidence Index, evidence lineage | the VE generation service | architecture §6; [ve-generation-service.md](ve-generation-service.md) §1.1, §2.3 |

## 2. Requirements

Each requirement states what the VE needs, why, and whether it is settled or open.

### 2.1 What a capture holds

- **LS-1 Gateway captures.**
  One capture per tool connection and gateway leadership term,
  with `capture_method` `decoded-message`, `vantage` `intermediary`, and records `re-encoded` (architecture §6).
  *Settled*: G5-200.
- **LS-2 The tool's side of the connection.**
  The VE plays the tool, so its evidence is the conversation as the tool saw it, session ID included;
  the gateway keeps System Bytes and rewrites the session ID (V-8).
  *Open*: which side's header the gateway records, and whether a record can carry both.
- **LS-3 Transport events and epochs.**
  Connect, Select, Deselect, Linktest, Separate, Reject, T3 and T7 timeouts and socket close, each socket an epoch.
  Without them every record is `correlation-incomplete` (V-20),
  and the transport adapter of architecture §4 has no evidence.
  *Open*: G5-200 assigns the wire and socket observers to the gateway's project.
- **LS-4 Malformed frames as received.**
  A frame the gateway fails to decode keeps its raw bytes as a `wire-exact` record,
  the only evidence for malformed-frame quirks (architecture §6).
  *Open*: the gateway's project.
- **LS-5 Traffic the gateway originates or answers itself.**
  Its `S1F1` ping, the FDC service's polling, and every reply it sends the tool without the EAP,
  recorded as exchanged with the tool, because the VE must answer them ([ve-evidence.md](ve-evidence.md) §3).
  *Open*: whether the gateway's observers see its own traffic.
- **LS-6 Capture descriptor.**
  `equipment_model`, `equipment_sw_rev`, `device_id`, `hsms_timers` and `equipment_connect_mode` filled when the gateway knows them:
  the software revision selects the manual version ([ve-evidence.md](ve-evidence.md) §5),
  and the timers and connect mode configure the VE's transport.
  *Open*: which of them the gateway knows.

### 2.2 What the pipeline does to records

- **LS-7 FDC polling thinned, not dropped.**
  Every poll's header and time, every abnormal reply in full, and a declared sample of normal reply bodies (V-27).
  *Settled*: G5-208 adds a record whose body is omitted by design and a capture-level declaration of filtered or sampled traffic.
  *Open*: the sampling rule, set from the FDC configuration ([ve-evidence.md](ve-evidence.md) §8).
- **LS-8 Sensitive content masked at write.**
  Learners read masked packs with keyed digests and never hold the key;
  values compared across message types share one digest domain (V-26).
  *Settled*: G5-207 masks recipe bodies with a keyed digest
  when the service writes a pack and keeps the original encrypted.
  *Open*: which fields beyond recipe bodies V-26's classes add to the policy (the owner);
  how the protected scorer of V-26 obtains the digest of a VE output without holding the key,
  and how a faithful replay restores an original (key custody is designed with P8).
- **LS-9 No other filtering.**
  Traffic of another peer on the tool's connection stays in the tool's capture;
  the Profile, not the pipeline, excludes it (V-21).
  *Open*: the gateway publishes frames unchanged (G5-207), and V-27's thinned polling is its only declared exception;
  the gateway owners confirm that nothing else is filtered.

### 2.3 Reading

- **LS-10 Read by tool and time.**
  The VE learns per batch, incrementally by epoch ([ve-generation-service.md](ve-generation-service.md) §1.1),
  and tracepack defines no view: the caller chooses which packs to read ([STO §4]).
  The service's catalog answers with the packs of a tool and a time window,
  together with what makes the window incomplete: crash gaps, seq gaps and declared losses.
  *Open*: the catalog's interface (G5-201).
- **LS-11 Settled hours.**
  The VE reads an hour once the service has merged it,
  and learns when a late segment reopens one ([STO §4] Readiness).
  *Open*: how the catalog shows it.
- **LS-12 Record identity outlives packs.**
  Lineage and the Evidence Index name records by (capture ID, seq) and evidence sets by capture and UTC hour, never by pack ID,
  because merged segments are deleted (architecture §6).
  *Settled*: a merge never alters record bytes ([STO §4]).

### 2.4 Retention

- **LS-13 Evidence of a promoted artifact outlives retention.**
  The service keeps one month by default (G5-200),
  while the artifact store keeps every dependency of a promoted artifact and the registry records its evidence lineage
  ([ve-generation-service.md](ve-generation-service.md) §2.3).
  Whether the captures a lineage names are dependencies to keep is undecided.
  Two ways meet both:
  - (a) the service holds every hour a lineage names, past its retention;
  - (b) the generation service copies the archives a promoted artifact depends on into its own content-addressed store.

  Recommended: (b).
  The service stays unaware of VE lineage, the artifact store already refuses deletion,
  and an archive is an immutable file.
  *Open*: the owner's decision.

## 3. Questions outstanding

| Question | For | Requirement |
|---|---|---|
| Which side's header a record carries; both if possible | gateway owners | LS-2 |
| Transport events and epochs from the observers | gateway owners | LS-3 |
| Raw bytes of frames that fail to decode | gateway owners | LS-4 |
| Whether the gateway's own traffic toward the tool is observed | gateway owners | LS-5 |
| Which descriptor tags the gateway knows | gateway owners | LS-6 |
| The FDC polling sampling rule | FDC maintainers, owner | LS-7 |
| Fields beyond recipe bodies under masking | owner | LS-8 |
| A digest of a VE output for the scorer; restoring an original for replay | tracepack maintainers (P8), service | LS-8 |
| The catalog's read interface and how it marks a settled hour | service | LS-10, LS-11 |
| Retention against evidence lineage | owner | LS-13 |
