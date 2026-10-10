# GEM behavioural skeleton

Status: current (2026-10-10) — reference distilled from SEMI E5, E30, E37, E37.1 and the object standards (§6)

This document describes the GEM behavioural skeleton a passive Virtual Equipment (VE) must exhibit:
its state models, the standard scenarios, the transport behaviour underneath them,
and the object machines a tool adds on top (§6).

Sources: SEMI E30-0611 (GEM), E5-0813 (SECS-II), E37-0413 (HSMS), E37.1-0702 (HSMS-SS), E4-0699 (SECS-I);
§6 adds E39, E87, E40, E94, E90 and their `.1` SECS-II mappings, with revisions listed there.
Notation: `H->E` is host to equipment, `E->H` is equipment to host, `W` is the W-bit, `CE` is a collection event (CEID).

Caveats:
- E30 has no communications transitions #11–13.
  Figure 4 and Table 1 skip from #10 to #14 and #15.
- The S2F49 header reads "M,H->E" without ",reply".
  Treated as W=1 because S2F50 is defined as its reply (inference, marked below).
- Two passive-side behaviours in §3.2 are inferences and are marked as such.

---

## 1. State models

### 1.1 Communications State Model (E30 §3.2, Figure 4, Table 1)

Hierarchy:
- DISABLED
- ENABLED, which contains:
  - NOT COMMUNICATING, which has two AND substates:
    - HOST-INITIATED CONNECT: {WAIT CR FROM HOST}
    - EQUIPMENT-INITIATED CONNECT: {WAIT CRA, WAIT DELAY}
  - COMMUNICATING

| # | From | Trigger | Guard | To | Action / messages |
|---|---|---|---|---|---|
| 1 | (entry) | System initialization | configured default (non-volatile) | DISABLED or ENABLED | none |
| 2 | DISABLED | Operator switches to ENABLED | — | ENABLED | none; **no CE** (§3.2.5) |
| 3 | ENABLED | Operator switches to DISABLED | — | DISABLED | stop all SECS-II now; discard the send queue; drop open transactions and conversations |
| 4 | (entry ENABLED) | any entry | — | NOT COMMUNICATING | none |
| 5 | (entry EQUIP-INIT CONNECT) | any entry to NOT COMMUNICATING | — | WAIT CRA | set CommDelay to "expired"; **send S1F13** |
| 6 | WAIT CRA | connection transaction failure | — | WAIT DELAY | start CommDelay = `EstablishCommunicationsTimeout` (s); dequeue queued messages, which may go to the spool (see 1.6 #1) |
| 7 | WAIT DELAY | CommDelay expired | — | WAIT CRA | **send S1F13** |
| 8 | WAIT DELAY | any message other than S1F13 received | — | WAIT CRA | discard it with no reply; set CommDelay "expired"; **send S1F13** |
| 9 | WAIT CRA | S1F14 received | COMMACK = 0 | COMMUNICATING | none |
| 10 | (entry HOST-INIT CONNECT) | any entry to NOT COMMUNICATING | — | WAIT CR FROM HOST | none |
| 14 | COMMUNICATING | communication failure | — | NOT COMMUNICATING | dequeue queued messages, which may go to the spool |
| 15 | WAIT CR FROM HOST | S1F13 received | — | COMMUNICATING | **send S1F14 COMMACK=0** |

Rules the table does not show (E30 §3.2.1, §3.2.4):
- **Connection transaction failure** means one of:
  - a communication failure;
  - no S1F14 within the reply timeout (T3);
  - a malformed S1F14;
  - COMMACK ≠ 0.
- **NOT COMMUNICATING:**
  - The equipment sends only S1F13, S1F14 and S9Fx.
  - It discards every received message except S1F13 and S1F14.
  - At most one equipment-initiated S1F13 may be open.
- **COMMUNICATING:**
  - A received S1F13 gets S1F14 with COMMACK=0.
  - A late S1F14 answering our own S1F13 requires no action.
  - If our own S1F13 is still open when COMMUNICATING is entered and then times out, send S9F9.
- **Simultaneous S1F13 from both sides:** whichever transaction completes first establishes communications.
  The other transaction stays open until it closes normally (§4.1.5.3).
- **"Communication failure" is defined by the protocol** (E30 §2.1.4):
  - HSMS: the transition SELECTED → NOT CONNECTED (E37 §4.1.2).
  - SECS-I: a failed send after RTY retries (E4 §4.1.8).
- **Constant:** `EstablishCommunicationsTimeout` is an ECV, format U2, in seconds (E30 §4.1.2; E5 Table 4).
- **Collection events:** the communications model defines none (§3.2.5).

### 1.2 Control State Model (E30 §3.3, Figure 5, Table 2, §4.12)

Hierarchy:
- OFF-LINE: {EQUIPMENT OFF-LINE, ATTEMPT ON-LINE, HOST OFF-LINE}
- ON-LINE: {LOCAL, REMOTE}

| # | From | Trigger | Guard / config | To | Action / messages |
|---|---|---|---|---|---|
| 1 | (undefined) | system init | configured default (one of EQ OFF-LINE / ATTEMPT / HOST OFF-LINE / ON-LINE) | per config | none |
| 2 | (undefined) | entry to OFF-LINE | configured substate | per config | none |
| 3 | EQUIPMENT OFF-LINE | operator ON-LINE switch | — | ATTEMPT ON-LINE | **send S1F1** on entry |
| 4 | ATTEMPT ON-LINE | S1F0 received, T3 reply timeout, or comm failure | config "ON-LINE fail target" | EQUIPMENT OFF-LINE or HOST OFF-LINE | none. A timeout counts as a denial (§4.12 fn 23) |
| 5 | ATTEMPT ON-LINE | S1F2 received | — | ON-LINE | CE via #7 |
| 6 | ON-LINE | operator OFF-LINE switch | — | EQUIPMENT OFF-LINE | CE "Equipment OFF-LINE" (S6F11); its reply is discarded (#3 note) |
| 7 | (undefined) | entry to ON-LINE | REMOTE/LOCAL switch | LOCAL or REMOTE | CE "Control State LOCAL" or "…REMOTE" |
| 8 | LOCAL | operator switch to REMOTE | — | REMOTE | CE "Control State REMOTE" |
| 9 | REMOTE | operator switch to LOCAL | — | LOCAL | CE "Control State LOCAL" |
| 10 | ON-LINE | S1F15 accepted | — | HOST OFF-LINE | **S1F16 OFLACK=0**, then CE "Equipment OFF-LINE" |
| 11 | HOST OFF-LINE | S1F17 accepted | — | ON-LINE | **S1F18 ONLACK=0**, then CE via #7 |
| 12 | HOST OFF-LINE | operator OFF-LINE switch | — | EQUIPMENT OFF-LINE | CE "Equipment OFF-LINE" |

Table note #3 (E30 Table 2 note 3):
an open host transaction is resolved either by sending the appropriate reply *before* the event,
or by sending Sx,F0 *after* the event transaction.
Both orders are permitted; the equipment must pick one.

Per-state message rules (§3.3):
- **OFF-LINE, inbound:** reply **Sx,F0** to every host primary except S1F13 and S1F17.
- **OFF-LINE, outbound:** send only S1F13, S1F1, and S9Fx.
  S9Fx is allowed only in response to S1F13 or S1F17.
- **OFF-LINE, replies:** discard received replies other than S1F14 and S1F2.
  Nothing is spooled.
- **S1F17:** positive only in HOST OFF-LINE.
  Otherwise reply ONLACK=1 (not allowed), or 2 if already ON-LINE (E5 ONLACK).
- **S1F15 while OFF-LINE:** reply S1F0 (§4.12.5.2).
- **ATTEMPT ON-LINE:** the ON-LINE and OFF-LINE switches are ignored.
- **ON-LINE:** Sx,F0 is discouraged.
  It is allowed only to close a transaction after a message fault.
- **LOCAL:** the host may not:
  - send remote commands that cause physical movement or start processing;
  - send, during processing, any command or EC change that affects the process.
- **LOCAL:** everything else stays available to the host:
  data collection, alarm and event configuration, recipe upload/download, recipe select that does not affect the running recipe, and terminal services.
- **REMOTE:** there are no host restrictions.
  An operator command issued in REMOTE produces CE "Operator Command Issued" (§4.12.4).
- **SV `ControlState` (E5 Table 4):** 1=OFF/EQ OFF-LINE, 2=OFF/ATTEMPT, 3=OFF/HOST OFF-LINE, 4=ON/LOCAL, 5=ON/REMOTE.

### 1.3 Processing State Model (E30 §3.4, Figure 6, Table 3)

This is an **example** model.
The tool-specific model replaces it.
The generic requirement is:
- one CE per transition;
- the SVs `ProcessState` and `PreviousProcessState`;
- the Table 8 CEs Processing Started, Completed, Stopped and State Change.

| # | From | Trigger | To | Notes |
|---|---|---|---|---|
| 1 | INIT | initialization complete | IDLE | — |
| 2 | IDLE | commit to set up | SETUP | — |
| 3 | SETUP | setup done, ready for START | READY | equipment-specific |
| 4 | READY | START (host RCMD or operator) | EXECUTING | CE "Processing Started" |
| 5 | EXECUTING | processing complete | IDLE | CE "Processing Completed" |
| 6 | PROCESSING ACTIVE | STOP | IDLE | CE "Processing Stopped" |
| 7 | PROCESSING ACTIVE | ABORT | IDLE | equipment-specific |
| 8 | PROCESS | equipment decides to PAUSE (e.g. alarm) | PAUSE | operator assist usually needed |
| 9 | PROCESS | PAUSE command | PAUSE | — |
| 10 | PAUSE | RESUME command | history (H*) PROCESS substate | — |

### 1.4 Alarm State Model, per ALID (E30 §4.3, Table 5)

| # | From | Trigger | To | Action |
|---|---|---|---|---|
| 1 | ALARMn CLEAR | alarm detected | ALARMn SET | local safety actions; update `AlarmsSet` and ALCD (bit 8 = 1); **S5F1** if the ALID is enabled; then **S6F11** CE "AlarmnDetected" if that CEID is enabled |
| 2 | ALARMn SET | alarm no longer detected | ALARMn CLEAR | update `AlarmsSet` and ALCD (bit 8 = 0); **S5F1** if enabled; then **S6F11** CE "AlarmnCleared" if enabled |

Rules (§4.3.3, §4.3.4):
- **S5F1 goes before its S6F11.**
- **Two CEIDs per ALID**, one for set and one for clear.
  Each CEID is enabled independently via S2F37.
- **S5F3 enable applies to both set and clear together.**
- **Enables are non-volatile** and must be reflected in the SV `AlarmsEnabled`.
- **ALCD bits 1–7 (category) are not used by GEM** (§5.1).

### 1.5 Limit Monitoring, per VID and per LIMITID (E30 §4.2.4, Figure 9, Table 4)

| # | From | Trigger | To | Action |
|---|---|---|---|---|
| 1 | DISABLED | S2F45 defines UPPERDB/LOWERDB | ENABLED (substate chosen from the current value) | none |
| 2 | ENABLED | S2F45 sets the limit undefined | DISABLED | none |
| 3 | BELOW LIMIT | value ≥ UPPERDB | ABOVE LIMIT | zone-transition CE |
| 4 | ABOVE LIMIT | value ≤ LOWERDB | BELOW LIMIT | zone-transition CE |
| 5 | NO ZONE | value ≤ LOWERDB | BELOW LIMIT | zone-transition CE |
| 6 | NO ZONE | value ≥ UPPERDB | ABOVE LIMIT | zone-transition CE |

Requirements (§4.2.4.3–§4.2.4.4):
- **At least 7 limits per monitored VID.**
- **One CEID per monitored VID**, carrying the DVVALs `LimitVariable`, `EventLimit` (a list of LIMITIDs) and `TransitionType` (0 = up, 1 = down).
- **Reject any S2F45 that breaks these rules:**
  - LIMITMAX ≥ UPPERDB ≥ LOWERDB ≥ LIMITMIN;
  - UPPERDB and LOWERDB must be both defined or both undefined.
- **S2F46 error codes:** VLAACK / LVACK / LIMITACK.
- **Allowed types:** integer, float, or boolean.
  Binary and lists are not allowed; ASCII must be converted before comparison.
- **Non-volatile:** limit definitions persist.

### 1.6 Spooling State Model (E30 §4.11, Figure 11, Table 7)

Hierarchy:
- POWER ON:
  - SPOOL INACTIVE
  - SPOOL ACTIVE, which has two AND substates:
    - SPOOL LOAD: {SPOOL NOT FULL, SPOOL FULL}
    - SPOOL UNLOAD: {NO SPOOL OUTPUT, SPOOL OUTPUT: {TRANSMIT, PURGE}}
- POWER OFF

| # | From | Trigger | Guard | To | Action |
|---|---|---|---|---|---|
| 1 | SPOOL INACTIVE | Comm #14 (COMMUNICATING→NOT COMM) or Comm #6 (WAIT CRA→WAIT DELAY) | `EnableSpooling` = TRUE | SPOOL ACTIVE (NOT FULL, NO SPOOL OUTPUT) | zero SpoolCountActual and SpoolCountTotal; abort open host transactions; SpoolStartTime = now; alert operator; CE "Spooling Activated" (queued) |
| 2 | SPOOL NOT FULL | message does not fit | — | SPOOL FULL | SpoolFullTime = now; alert operator; no CE |
| 3 | SPOOL OUTPUT | spool emptied | — | SPOOL INACTIVE | CE "Spooling Deactivated" |
| 4 | NO SPOOL OUTPUT | S6F23 RSDC=1 | — | PURGE SPOOL | discard all; SpoolCountActual = 0 |
| 5 | NO SPOOL OUTPUT | S6F23 RSDC=0 | — | TRANSMIT SPOOL | send oldest first; one open transaction at a time |
| 6 | TRANSMIT SPOOL | comm failure, or MaxSpoolTransmit reached | — | NO SPOOL OUTPUT | CE "Spool Transmit Failure" on comm failure only |
| 7 | POWER ON | power lost | — | POWER OFF | context already in non-volatile storage |
| 8 | POWER OFF | power restored | — | POWER ON (H*) | restore context; spooling continues |

Rules (§4.11.3–§4.11.4):
- **What is spooled:** only primaries enabled via S2F43.
  Other primaries except Stream 1 are discarded.
- **Never spooled:** Stream 1 (S1F1, S1F13).
  The equipment must reject an S2F43 that asks to spool S1.
- **Secondaries are never spooled.**
  They are attempted, then discarded if the send fails.
- **SPOOL FULL:**
  - OverWriteSpool=TRUE drops the oldest message.
  - OverWriteSpool=FALSE discards new messages; SpoolCountTotal still increments.
  - A full spool leaves FULL only via SPOOL INACTIVE.
- **MaxSpoolTransmit** (ECV, U4): 0 means unlimited.
  Inquire/grant sequences are not counted.
- **Multi-block spooled messages** are spooled first.
  Their inquire/grant is resolved during transmit;
  a denied grant discards the message, which counts once.

---

## 2. Standard scenarios (E30 §4; message structures and codes from E5 §10)

Establish Communications (§4.1.5):
- **Host-initiated:** H->E S1F13 `L,0` / E->H S1F14 `L,2 {COMMACK=0, L,2 {MDLN, SOFTREV}}`.
- **Equipment-initiated:** E->H S1F13 `L,2 {MDLN, SOFTREV}` / H->E S1F14 COMMACK.
  COMMACK: 0 accept, 1 denied.
- **Simultaneous:** both transactions complete independently (§4.1.5.3).

On-line Identification (§4.2.6):
- H->E S1F1 (header only) / E->H S1F2 `L,2 {MDLN, SOFTREV}`.
  The host's own S1F2 is `L,0`.
- SOFTREV must change with every software change.

Control:
- **Operator ON-LINE:** E->H S1F1 / H->E S1F2 (accept), then E->H S6F11 "Control State LOCAL/REMOTE".
  - Instead of S1F2, the host may send H->E S1F0 (deny) or let T3 time out;
    either leads to transition #4.
- **Operator OFF-LINE / LOCAL / REMOTE:** E->H S6F11 with the matching CE, then H->E S6F12.
- **Host OFF-LINE:** H->E S1F15:
  - if already OFF-LINE: E->H S1F0;
  - otherwise: E->H S1F16 OFLACK=0, then S6F11 "Equipment OFF-LINE".
- **Host ON-LINE:** H->E S1F17:
  - if HOST OFF-LINE is not active: E->H S1F18 ONLACK=1 (or 2 if already ON-LINE);
  - otherwise: S1F18 ONLACK=0, then S6F11 "Control State LOCAL/REMOTE".

Event reports (§4.2.1):
- **Event occurs:**
  - [if multi-block] E->H S6F5 / H->E S6F6 GRANT6 (0 ok, 1 busy, 2 not interested);
  - then E->H S6F11 `L,3 {DATAID, CEID, L,a {L,2 {RPTID, L,b {V}}}}` / H->E S6F12 ACKC6.
  - With no linked reports, the report list is `L,0`.
- **Event report request:** H->E S6F15 `<CEID>` / E->H S6F16 (same body as S6F11).
- **Dynamic configuration (§4.2.1.2.5):**
  - optional H->E S2F39 / E->H S2F40 GRANT (0 ok, 1 busy, 2 no space, 3 duplicate DATAID);
  - H->E S2F33 `{DATAID, L {RPTID, L {VID}}}` / E->H S2F34 DRACK;
  - H->E S2F35 `{DATAID, L {CEID, L {RPTID}}}` / E->H S2F36 LRACK;
  - H->E S2F37 `{CEED, L {CEID}}` / E->H S2F38 ERACK.
- **Zero-length semantics (E5):**
  - S2F33 with an empty report list deletes all reports and links.
  - An RPTID with an empty VID list deletes that report and its links.
  - A CEID with an empty RPTID list in S2F35 unlinks all reports from that event.
  - An empty CEID list in S2F37 means all CEIDs.
  - Newly linked events default to disabled.
  - DRACK and LRACK are all-or-nothing: no partial changes.
- **Ack codes:**
  - DRACK: 1 no space, 2 bad format, 3 RPTID already defined, 4 VID unknown.
  - LRACK: 1 no space, 2 bad format, 3 CEID already linked, 4 CEID unknown, 5 RPTID unknown.
  - ERACK: 1 CEID unknown.
- **Persistence:** reports, links and enables are non-volatile.
- **SV `EventsEnabled`:** the list of enabled CEIDs.

Variable, status and constant queries:
- **Report data:** H->E S6F19 `<RPTID>` / E->H S6F20 `L {V}` (§4.2.2.5).
  - SV and ECV values are current;
    DVVALs are valid only at their event;
    an unavailable value is sent as a zero-length item.
- **Status:** H->E S1F3 `L {SVID}` / E->H S1F4 `L {SV}`.
  An empty list means all SVIDs (§4.2.5.5).
- **SV names:** H->E S1F11 `L {SVID}` / E->H S1F12 `L {L,3 {SVID, SVNAME, UNITS}}`.
  An empty list means all.
- **EC values:** H->E S2F13 `L {ECID}` / E->H S2F14 `L {ECV}`.
  An empty list means all.
- **EC names:** H->E S2F29 / E->H S2F30 `L {L,6 {ECID, ECNAME, ECMIN, ECMAX, ECDEF, UNITS}}`.
- **EC set:** H->E S2F15 `L {L,2 {ECID, ECV}}` / E->H S2F16 EAC (0 ok, 1 unknown ECID, 2 busy, 3 out of range) (§4.5.5).
  - The equipment must be in a "safe" condition to accept the change.
  - LOCAL-mode restrictions apply (§3.3).
- **Operator changes an EC:** E->H S6F11 CE "Operator Equipment Constant Change" (with ECID).

Trace (§4.2.3.5):
- H->E S2F23 `L,5 {TRID, DSPER, TOTSMP, REPGSZ, L {SVID}}` / E->H S2F24 TIAACK.
  - TIAACK: 0 ok, 1 too many SVIDs, 2 no more traces, 3 bad period, 4 unknown SVID, 5 bad REPGSZ.
- Then, every REPGSZ samples: E->H S6F1 `L,4 {TRID, SMPLN, STIME, L {SV}}` / H->E S6F2 ACKC6.
  - There is no inquire/grant: S2F23 is an implicit grant.
- **Report count:** TOTSMP/REPGSZ reports.
  The trace is auto-deleted after the last report.
- **Reuse and cancel:** reusing a TRID restarts that trace; TOTSMP=0 cancels it.
- **Concurrency:** at least 4 concurrent traces.
  The same SVID may appear in several traces.

Limits (§4.2.4.5):
- **Define:** H->E S2F45 (optionally after S2F39/40) / E->H S2F46 `{VLAACK, L {VID, LVACK, L {LIMITID, LIMITACK}}}`.
- **Query:** H->E S2F47 / E->H S2F48.
- **Zone transition:** reported as S6F11.

Alarms (§4.3.5):
- **Report:** E->H S5F1 `L,3 {ALCD, ALID, ALTX}` / H->E S5F2 ACKC5;
  then S6F11/S6F12 if that CE is enabled.
- **Enable:** H->E S5F3 `L,2 {ALED, ALID}` / E->H S5F4 ACKC5.
  ALED bit 8 = enable; an empty ALID means all alarms.
- **List:** H->E S5F5 `<ALID…>` / E->H S5F6 `L {L,3 {ALCD, ALID, ALTX}}`.
  An empty ALID means all alarms regardless of enable state.
- **Enabled list:** S5F7/F8 exist in E5 but are not in the GEM subset.
  GEM uses the SVs AlarmsSet and AlarmsEnabled instead.

Remote control (§4.4.5):
- **S2F41:** H->E S2F41 `L,2 {RCMD, L {L,2 {CPNAME, CPVAL}}}` / E->H S2F42 `L,2 {HCACK, L {L,2 {CPNAME, CPACK}}}`,
  then a later S6F11 state-change CE.
- **S2F49:** H->E S2F49 `{DATAID, OBJSPEC, RCMD, L {CPNAME, CEPVAL}}` / E->H S2F50 `{HCACK, L {CPNAME, CEPACK}}`.
  - Use S2F39/40 first if multi-block.
- **HCACK:** 0 done, 1 no such command, 2 cannot perform now, 3 bad parameter, 4 accepted with completion signaled later by an event, 5 already in the desired condition, 6 no such object.
- **CPACK:** 1 unknown CPNAME, 2 bad value, 3 bad format.
- **Completion:** an accepted command (0 or 4) must produce a state transition or CE on completion.
- **RCMD format:** ASCII, at most 20 characters, printable 0x21–0x7E, no spaces.
  Upper case must be accepted.
- **Required RCMDs:** START and STOP.
- **Optional RCMDs:** PP-SELECT, RCP-SELECT, PAUSE, RESUME, ABORT (CPNAME `AbortLevel`).

Recipes, Stream 7 process programs (§4.6.5; PPGNT: 0 ok, 1 already have, 2 no space, 3 bad PPID, 4 busy, 5 will not accept; ACKC7):
- **Delete:** H->E S7F17 / S7F18.
- **Directory:** H->E S7F19 / S7F20.
- **Unformatted upload:** H->E S7F5 / E->H S7F6.
  S7F6 is `L,0` if the PPID does not exist.
- **Unformatted download:** H->E S7F1 / S7F2 PPGNT, then H->E S7F3 / S7F4.
  S7F1 is recommended before any download and required for multi-block.
- **Equipment-initiated:** E->H S7F1/S7F2 then E->H S7F3/S7F4 (upload), or E->H S7F5 / H->E S7F6 (download request).
- **Formatted:** S7F23/F24 and S7F25/F26.
  After a download the equipment verifies and sends E->H S7F29/F30, then S7F27/F28.
- **Large programs:** S7F37–F44, which delegate to S13F1–F8 data-set transfer (§4.6.5.4–5).
- **Operator create/edit/delete:** CE "Process Program Change" with PPChangeName and PPChangeStatus (1 created, 2 edited, 3 deleted).
- **Alternatives:** E42 recipes use Stream 15, E139 recipes use Stream 19 plus Stream 13 (§4.6.6, §4.6.7).

Material movement (§4.7): CE "Material Received" and "Material Removed", reported with S6F11/F12.

Terminal services (§4.8.5):
- **Host display:** H->E S10F3 `{TID, TEXT}` / E->H S10F4 ACKC10 (0 displayed, 1 will not display, 2 terminal unavailable).
  When the operator acknowledges it, E->H S6F11 CE "Message Recognition".
- **Operator entry:** E->H S10F1 / H->E S10F2 ACKC10.
- **Multi-block:** H->E S10F5 / S10F6.
  If unsupported, E->H S10F7 "multi-block not allowed".
- **Overwrite and clear:** a new S10F3 overwrites an unrecognized message.
  A zero-length TEXT clears it.
- **Capacity:** at least 160 characters in each direction.

Clock (§4.10.5):
- **Set:** H->E S2F31 `<TIME>` / E->H S2F32 TIACK (0 ok, 1 error).
- **Read:** H->E S2F17 / E->H S2F18 `<TIME>`.
- **Equipment request (optional):** E->H S2F17 / H->E S2F18.
- **TIME format** follows ECV `TimeFormat`: 0 = 12-byte YYMMDDhhmmss, 1 = 16-byte YYYYMMDDhhmmsscc, 2 = extended (E148).
- **Ordering:** centiseconds should preserve event order;
  E30 §4.10.3 permits `00` when the equipment cannot resolve or represent that order.

Spooling (§4.11.5):
- **Configure:** H->E S2F43 `L {L,2 {STRID, L {FCNID}}}` / E->H S2F44 `{RSPACK, L {…errors}}`.
  - m=0 turns spooling off; n=0 spools the whole stream; Stream 1 is rejected.
- **Unload:** after re-establishing communications, H->E S1F3 (SpoolCountActual) is typical,
  then H->E S6F23 `<RSDC>` / E->H S6F24 RSDA (0 ok, 1 busy, 2 no spooled data).
  - Spooled primaries follow; when the spool is empty, E->H S6F11 "Spooling Deactivated".
- **MaxSpoolTransmit > 0:** the host repeats S6F23 until the spool is empty.
  No CE marks the limit.

Error messages, Stream 9 (E30 §4.9, E5 §8.3, §10.13):

| Condition detected by equipment | Send | Body |
|---|---|---|
| Unrecognized device ID | S9F1 | MHEAD (10-byte header of the offending message) |
| Unrecognized stream | S9F3 | MHEAD |
| Unrecognized function | S9F5 | MHEAD |
| Illegal data/format (E5 §10.3.1.3 non-compliance, unsupported item type) | S9F7 | MHEAD |
| Transaction timer (T3; in SECS-I, T3 or T4) expired on our primary | S9F9 | SHEAD (header of the primary that timed out) |
| Message longer than the equipment can handle | S9F11 | MHEAD |
| Conversation timeout (e.g. grant given, send never came) | S9F13 | `L,2 {MEXP, EDID}` |

Transaction rules (E5 §8.3):
- **Take no further action** on a message in error.
- **A reply of Function 0** closes the transaction, and no S9 is sent.
- **Multiple open transactions** are optional (E5 §6.6).
- **Under HSMS,** MHEAD and SHEAD carry the 10-byte HSMS header (E37 §9.4.2.1).

---

## 3. Transport behavior the equipment must exhibit

### 3.1 HSMS generic (E37 §5–§9)

States: NOT CONNECTED, then CONNECTED {NOT SELECTED, SELECTED}.
Data messages are allowed only in SELECTED.

E37 Table 1 transitions:
1. Preparation leads to NOT CONNECTED.
2. A TCP connection is established: → NOT SELECTED.
3. The TCP connection breaks: → NOT CONNECTED.
4. Select succeeds: → SELECTED.
5. Deselect or Separate: → NOT SELECTED.
6. T7 expires: → NOT CONNECTED.

Control messages (E37 §8.3, Tables 5–9).
Every control message is header-only, PType=0, length 10.

| SType | Message | SessionID | Byte 3 | Behaviour as responder |
|---|---|---|---|---|
| 1/2 | Select.req/.rsp | subsidiary-defined (HSMS-SS: 0xFFFF) | SelectStatus: 0 ok, 1 already active, 2 not ready, 3 connect exhaust | accept → SELECTED |
| 3/4 | Deselect.req/.rsp | as Select | DeselectStatus: 0 ended, 1 not established, 2 busy | → NOT SELECTED |
| 5/6 | Linktest.req/.rsp | 0xFFFF | 0 | always answer; valid in any CONNECTED state |
| 7 | Reject.req | copied from the rejected message | ReasonCode: 1 SType unsupported, 2 PType unsupported, 3 transaction not open, 4 entity not selected | byte 2 = the rejected SType, or its PType when reason = 2 |
| 9 | Separate.req | as Deselect | 0 | no reply; → NOT SELECTED if SELECTED, ignored otherwise |

Rules (E37 §7.10.3, §8.2.6.8–9, §9.4.1):
- **Reject is mandatory for:**
  - a data message received in NOT SELECTED;
  - an unknown SType or PType;
  - a response control message with no open request.
- **System bytes:**
  - A primary must be unique among the sender's open transactions and different from the last completed one.
  - A reply echoes its primary's system bytes.
  - A .rsp echoes its .req.
- **A reply matches its primary on four fields:**
  - same SessionID;
  - same stream;
  - function + 1, or function 0;
  - same system bytes.
- **Message format:**
  - 4-byte length (MSB first), 10-byte header, then the text.
  - Header byte 2 = W-bit (bit 7) plus stream; byte 3 = function (LSB 1 = primary).
  - PType=0 is SECS-II; SType=0 is a data message.

Timers (E37 §4.1.28–32, §9, Table 10; must be settable and non-volatile):

| Timer | Range / typical | Meaning | Passive equipment action on expiry |
|---|---|---|---|
| T3 reply | 1–120 s / 45 s | wait for the reply to a W=1 primary; one timer per open transaction | close the transaction; send **S9F9**; keep the connection (E37.1 Table 1 #6) |
| T5 connect separation | 1–240 s / 10 s | minimum gap between *active* connect attempts | not used by a passive entity |
| T6 control transaction | 1–240 s / 5 s | .req to .rsp | on our Linktest.req (or Deselect under generic HSMS): communication failure, so close TCP |
| T7 NOT SELECTED | 1–240 s / 10 s | TCP accepted but no Select | close TCP → NOT CONNECTED; arm on every accept |
| T8 inter-character | 1–120 s / 5 s | gap between bytes of a single message (receiver only) | communication failure, so close TCP |

Passive-mode configuration (E37 Table 10, §6.3.4):
- Parameters: `Connect Mode = PASSIVE`, local IP and port (typical 5000).
- Procedure: listen, accept, → NOT SELECTED.

When already serving one connection,
a passive entity handles an extra connection request with one of three options (E37 §9.2.4.1.1).
Document which one is used.
- **Preferred:** accept it and answer every Select with status 1.
- Or actively reject the TCP connect.
- Or do not listen at all.
- E37.1 R1-3.1 describes equipment that immediately terminates the second connection.

### 3.2 HSMS-SS restrictions (E37.1, applied on top of E37)

Passive-mode transitions (E37.1 Table 1).
A VE must implement these rows:

| # | From → To | Trigger | Action |
|---|---|---|---|
| 1 | — → NOT CONNECTED | init | listen |
| 2 | NOT CONNECTED → NOT SELECTED | TCP accept succeeds | **start T7** |
| 3 | NOT SELECTED → SELECTED | Select.req received and allowed | cancel T7; send Select.rsp status 0 |
| 4 | NOT SELECTED → NOT CONNECTED | T7 expires; Select refused (non-zero status sent); **any message other than Select.req** (Linktest and data included); length ≠ 10; bad header; T8; TCP error | close TCP |
| 5 | SELECTED → NOT CONNECTED | we send Separate.req; Separate.req received; T6 on Linktest.rsp; length < 10 or > our max; bad header; T8; TCP error | close TCP |
| 6 | SELECTED → SELECTED | T3 expires waiting for a data reply | cancel that transaction; **equipment sends S9F9**; do not close |

Restrictions (E37.1 §5.1, §7, §8, §9):
- **Select:**
  - Only the **active** side initiates it.
    Passive equipment never sends Select.req.
  - It is valid only in NOT SELECTED and uses SessionID 0xFFFF.
  - After a failed Select, both sides close TCP.
- **Deselect is not used.**
  Communications end with **Separate** (SessionID 0xFFFF).
  After sending or receiving it, close TCP immediately.
- **Linktest is allowed only in SELECTED.**
  Either side may initiate it; T6 guards the reply.
- **Reject is optional.**
  If it is unsupported, any condition that would require a Reject is a communication failure, so close TCP.
- **Any other violation** of HSMS-SS rules is a communication failure (§7.7).
- **Session ID:**
  - Data messages: SessionID = Device ID (15 bits, MSB 0).
    The equipment defines at least one Device ID.
  - Control messages: SessionID is always 0xFFFF.
- **Types:** PType is always 0, and only E37-defined STypes are allowed.
  No user-defined STypes.
- **SECS-I compatibility:** a message that E5 defines as single-block should not exceed 254 bytes of HSMS length (10 + 244) (§9.1).
- **Documentation must state:** the supported Device IDs, the termination procedure, and the host/equipment role (§10).
- *Inference:* select-state rules for a VE.
  - Under E37.1, a Select.req received while already SELECTED violates §7.1, so treat it as a communication failure.
  - Generic E37 would instead answer SelectStatus=1.
- *Inference:* on entering SELECTED the GEM layer is still NOT COMMUNICATING.
  - S1F13 attempts made before Select fail, so the equipment is in WAIT DELAY.
  - Many implementations send S1F13 right after Select;
    the standard does not mandate this timing.

### 3.3 SECS-I equivalents (E4 §7–§10, Table 4)

| Param | Meaning | Typical / range |
|---|---|---|
| T1 | inter-character within a block (after the length byte) | 0.5 s / 0.1–10 s |
| T2 | protocol: ENQ→EOT, EOT→length byte, checksum→ACK | 10 s / 0.2–25 s |
| T3 | reply: last block of primary → first block of reply | 45 s / 1–120 s |
| T4 | inter-block within a multi-block message | 45 s / 1–120 s |
| RTY | block send retries before a failed send | 3 / 0–31 |
| DEVID | device ID, 15 bits | 0–32767 |
| M/S | equipment = **master**, host = slave | — |
| BAUD | 9600 typical; 300–9600 required (19200 optional) | — |

Block handshake (E4 §7.2, §7.8).
Codes: ENQ 0x05, EOT 0x04, ACK 0x06, NAK 0x15.
- **Send:**
  - Send ENQ and wait T2 for EOT.
  - Send the length byte (10–254), the block (10-byte header + ≤244 data),
    then a 16-bit checksum (sum of the bytes after the length byte, high byte first).
  - Wait T2 for ACK.
- **Retry:**
  - A timeout or a non-ACK increments the retry count and restarts at ENQ.
  - Exceeding RTY is a **failed send**, which is a communication failure (E4 §4.1.8, triggering E30 Comm #14 or #6).
- **Contention:**
  - A slave that receives ENQ in reply to its own ENQ yields and sends EOT.
  - A master that has sent ENQ ignores everything except EOT, so the equipment always wins.
- **Receive:**
  - On ENQ while idle, send EOT.
  - Wait T2 for the length byte, then enforce T1 between characters.
  - A bad length or checksum: keep listening until T1 expires, then send NAK.
  - A good block gets ACK.

Message layer (E4 §8–§9):
- **Header fields:** R-bit (0 = to equipment, 1 = to host), device ID, W-bit, stream/function, E-bit (last block), 15-bit block number, 4 system bytes.
  - Block number: 0 or 1 for single-block; 1..n for multi-block.
- **Consistency:** W-bit and system bytes are identical in every block of a message.
- **Duplicate blocks:** discard a block whose full header equals the last accepted block's header (§9.4.2).
  This must be switchable.
- **Routing error:** a device ID mismatch at this layer leads to SECS-II S9F1.
- **T4 expiry** cancels the message and aborts the transaction.
- **Interleaving** and multiple open transactions are optional (§9.2.4).

---

## 4. E5 §10 stream/function catalog (GEM subset) with W-bit conventions

Conventions (E5 §7.2, §8, §10.4; E4 §8.4; E37 §8.3.3.3):
- **Numbering:** primary = odd function; reply = function + 1.
- **W-bit:** W=1 on a primary that expects a reply; W=0 on every reply.
- **Reserved even function:** the even function after a no-reply primary is reserved.
- **Function 0:** exists in every stream (S,H<->E).
  It aborts, i.e. closes, a transaction.
- **"reply":** W must be 1.
- **"[reply]":** optional; the sender chooses W, typically configurable.
- **No marker:** a primary without either marker is sent with W=0.
- **User-defined ranges:** S1–S63 with F64–F255, and S64–S127 with F1–F255.
- **Block codes:** S = single-block (≤244 bytes of text for SECS-I compatibility); M = may be multi-block.
- **Inquire/grant:** chosen per message family, not globally.
  Host S2 multi-block (S2F33/35/45/49) uses S2F39/40;
  equipment event reports use S6F5/6;
  process programs use S7F1/2 (either direction);
  other services (S13, S14, S15, S19) define their own conversations.

Direction columns: H->E = host initiates, E->H = equipment initiates, H<->E = both.

| Pair | Name | Blk (pri/rep) | Initiator | Reply |
|---|---|---|---|---|
| S1F1/F2 | Are You There / On Line Data | S/S | both | req |
| S1F3/F4 | Selected Status Req / Data | S/M | H | req |
| S1F11/F12 | SV Namelist Req / Reply | S/M | H | req |
| S1F13/F14 | Establish Comms Req / Ack | S/S | both | req |
| S1F15/F16 | Request OFF-LINE / Ack | S/S | H | req |
| S1F17/F18 | Request ON-LINE / Ack | S/S | H | req |
| S1F21–F24 | DV / CE Namelist (E5 only, not in the E30 subset) | S/M | H | req |
| S2F13/F14 | EC Request / Data | S/M | H | req |
| S2F15/F16 | New EC Send / Ack | S/S | H | req |
| S2F17/F18 | Date-Time Request / Data | S/S | both | req |
| S2F23/F24 | Trace Initialize / Ack | M/S (E30 §7 says S) | H | req |
| S2F29/F30 | EC Namelist Req / Namelist | S/M | H | req |
| S2F31/F32 | Date-Time Set / Ack | S/S | H | req |
| S2F33/F34 | Define Report / Ack | M/S | H | req |
| S2F35/F36 | Link Event Report / Ack | M/S | H | req |
| S2F37/F38 | Enable/Disable Event Report / Ack | S/S | H | req |
| S2F39/F40 | Multi-block Inquire / Grant | S/S | H | req |
| S2F41/F42 | Host Command / Ack | S/S | H | req |
| S2F43/F44 | Reset Spooling / Ack | S/M | H | req |
| S2F45/F46 | Define Variable Limits / Ack | M/M | H | req |
| S2F47/F48 | Variable Limit Request / Send | S/M | H | req |
| S2F49/F50 | Enhanced Remote Command / Ack | M/M | H | req (*header line omits ",reply"; S2F50 exists, so W=1 is inferred*) |
| S5F1/F2 | Alarm Report / Ack | S/S | E | [opt] |
| S5F3/F4 | Enable/Disable Alarm / Ack | S/S | H | [opt] |
| S5F5/F6 | List Alarms Req / Data | S/M | H | req |
| S5F7/F8 | List Enabled Alarms (E5 only) | S/M | H | req |
| S6F1/F2 | Trace Data / Ack | M/S | E | [opt] |
| S6F5/F6 | Multi-block Data Inquire / Grant | S/S | E | req |
| S6F11/F12 | Event Report / Ack | M/S | E | req |
| S6F15/F16 | Event Report Request / Data | S/M | H | req |
| S6F19/F20 | Individual Report Req / Data | S/M | H | req |
| S6F23/F24 | Request Spooled Data / Ack | S/S | H | req |
| S7F1/F2 | PP Load Inquire / Grant | S/S | both | req |
| S7F3/F4 | PP Send / Ack | M/S | both | req |
| S7F5/F6 | PP Request / Data | S/M | both | req |
| S7F17/F18 | Delete PP / Ack | S/S | H | req |
| S7F19/F20 | Current EPPD Req / Data | S/M | H | req |
| S7F23–F26 | Formatted PP Send/Ack, Request/Data | M,S / S,M | both | req |
| S7F27/F28 | PP Verification Send / Ack | S/S | E | req |
| S7F29/F30 | PP Verification Inquire / Grant | S/S | E | req |
| S7F37–F44 | Large (formatted) PP Send/Request, then S13 | S/S | both | req |
| S9F1,3,5,7,9,11,13 | Error notifications | S | E | none (W=0) |
| S10F1/F2 | Terminal Request / Ack | S/S | E | [opt] |
| S10F3/F4 | Terminal Display Single / Ack | S/S | H | [opt] |
| S10F5/F6 | Terminal Display Multi / Ack | M/S | H | [opt] |
| S10F7 | Multi-block Not Allowed | S | E | reply to S10F5 |
| S10F9/F10 | Broadcast (E5 only) | S/S | H | [opt] |

Other streams, as counts of message headers in E5 §10:

| Stream | Messages by initiator |
|---|---|
| S3 materials status | mostly H->E requests with E->H data; a few E->H; carrier services S3F17–F32 (§6.2) |
| S4 material control | mixed |
| S8 control program | both directions |
| S12 wafer map | E->H requests with reply; H->E data |
| S13 data sets | all both directions (used by large recipes) |
| S14 object services | all both directions (GEM uses S14F1/F2 GetAttr); object services and control-job create (§6.1, §6.4) |
| S15 recipes (E42) | mostly both directions |
| S16 jobs (E40/E94) | H->E commands; E->H alerts and notifications (§6.3, §6.4) |
| S17 | H->E requests |
| S18 subsystems | H->E requests |
| S19 recipe and parameter (E139) | mostly both directions |

---

## 5. What is tool-specific (the delta)

### 5.1 Standard across every GEM tool (build once in the VE core)

- **State machines** 1.1, 1.2, 1.4, 1.5 and 1.6, plus the transport machines in §3.
  - Fundamental GEM (E30 Table 9): state models §3.0–3.3, processing states §3.4, host-initiated S1F13/14, event notification, online identification, error messages, operator-initiated control, documentation.
  - Everything else is an opt-in capability (E30 Table 10), per tool.
- **Object machines** (§6), for a tool that declares them: the E87, E40, E94 and E90 state models, the E39 services and their `.1` message mappings.
- **Message set, structures, zero-length semantics and ACK enumerations** (§2, §4).
- **Behavioural invariants:**
  - alarm report before event report;
  - report data built at event time;
  - Stream 1 never spooled;
  - trace auto-delete;
  - at least 4 traces and at least 7 limits per VID;
  - non-volatile reports, links, enables, ECs, limits and spool context.
- **Required variable *names*** (E30 §5.2):
  - DVVAL: AlarmID, EventLimit, LimitVariable, TransitionType, PPChangeName/Status, PPError, RcpChangeName/Status.
  - ECV: EstablishCommunicationsTimeout, MaxSpoolTransmit, OverWriteSpool, TimeFormat, plus EnableSpooling (§4.11.3.2).
  - SV: AlarmsEnabled, AlarmsSet, Clock, ControlState, EventsEnabled, PPExecName, PPFormat, PPError, ProcessState, PreviousProcessState, RcpExecName, SpoolCountActual, SpoolCountTotal, SpoolFullTime, SpoolStartTime.
- **Required CE *names*** (E30 Table 8):
  - Equipment OFF-LINE; Control State LOCAL; Control State REMOTE; Operator Command Issued;
  - Processing Started, Completed, Stopped, State Change;
  - AlarmnDetected and AlarmnCleared for every alarm;
  - Operator EC Change; Limit Zone Transition for every monitored VID;
  - PP Change; Process Recipe Selected; Material Received and Removed;
  - Spooling Activated, Deactivated, Transmit Failure; Message Recognition;
  - E42 recipe events; Successful and Bad Upload.
- **Format restrictions** (E30 §5.1, octal format codes):
  - ALID, CEID, DATAID, ECID, RPTID, SVID, VID, TRID, TOTSMP, REPGSZ, SMPLN, LENGTH: `5()` (unsigned).
  - PPID, RCMD, CPNAME, TEXT: `20` (ASCII); CPNAME is at most 40 characters.
  - SEQNUM: `52`.
  - ACKC7A: `51`.
- **Standard enumerations:** RCMD strings START and STOP (required);
  PP-SELECT, RCP-SELECT, PAUSE, RESUME and ABORT when implemented.
  ControlState codes 1–5.

### 5.2 Must be learned per tool (the per-tool delta)

E30 standardizes **names and semantics, not numeric IDs**.
A VE needs a per-tool map from ID to name, format, units and meaning.

| Item | What varies | Source for the VE |
|---|---|---|
| Identity | MDLN, SOFTREV, Device ID(s), HSMS port, T3–T8, EstablishCommunicationsTimeout default | vendor manual; S1F2/S1F14 captures |
| SVID catalog | IDs, names, format, units, range; which are list-typed; which map to the standard SV names | S1F11/12 (empty list = all), S1F3/4 |
| ECID catalog | IDs, name, min/max/default, units; process-affecting vs not (LOCAL rules); which need a "safe" state | S2F29/30, S2F13/14 |
| DVID catalog | IDs; **which CE each DVVAL is valid at** (E30 §4.2.1.2.3) | vendor manual; S1F21/22 if supported |
| CEID catalog | IDs for every Table 8 event plus the tool's processing, material and custom events; conditions that fire them | vendor manual; S1F23/24 if supported; S6F11 logs |
| Default reports | predefined RPTIDs and links (E30 §4.2.1.1.4 #4); DATAID handling | S6F15/16 per CEID, S6F19/20 |
| Processing model | actual states, transitions, one CEID each; ProcessState value codes | vendor state-model documentation (E30 §3 requires it) |
| Alarms | ALID list, ALTX, set/clear CEID pair per ALID, what each alarm inhibits, which are not controllable | S5F5/6 (empty = all), vendor manual |
| Remote commands | RCMD set beyond START/STOP; CPNAME/CPVAL/CEPVAL definitions; OBJSPEC targets for S2F49; S2F41 vs S2F49 support; case handling; the HCACK a command returns | vendor manual; S2F41/42 captures |
| Limits | which VIDs are limit-capable; LIMITMAX/LIMITMIN; the zone CEID for each | S2F47/48 |
| Recipes | PPFormat; PPID length and charset; S7 vs S15 vs S19; formatted-command codes (S7F22); storage capacity | S7F19/20, vendor manual |
| Material | ports; Material Received/Removed CEIDs; port and material DVVALs | vendor manual |
| Terminal | TIDs; multi-block support | vendor manual |
| Clock | TimeFormat setting; resolution | S2F17/18 |
| Spool | capacity; default S2F43 set; OverWriteSpool/MaxSpoolTransmit defaults | S2F43/44, S1F3 on spool SVs |
| Control config | default control state at init; ON-LINE-fail target (EQ OFF-LINE vs HOST OFF-LINE); operator-restriction settings in REMOTE | vendor manual |
| Capability set | which Table 10 capabilities exist (GEM Compliance Statement, E30 Table 11) | vendor GEM manual |
| Transport | max message size; interleaving and open-transaction limit; refusal option for extra connections | E37 §10.1, E37.1 §10 |
| Custom messages | user-defined SxFy (S1–63 F64–255, S64–127) | vendor message documentation (E5 §11) |
| Object machines | which object standards and revisions the tool implements; ports, buffer type, queues, commands, CEIDs and encodings per §6.1–§6.5 | vendor GEM manual; S3, S14, S16 and S6F11 captures |

Rule of thumb for the VE:
- **Everything in 5.1 is code.**
  It is shared across tools.
- **Everything in 5.2 is data.**
  It forms a per-tool profile, loaded as a catalog of SV/EC/DV/CE/ALID/RCMD, a processing state machine, default reports, and timer/config values.
- **Some of it can be recovered by probing** a real tool with host-initiated queries:
  S1F11, S2F29, S5F5, S7F19, S1F21, S1F23, S6F15.
- **The rest needs the vendor's GEM manual.**
  Most importantly:
  - DVVAL validity per event;
  - the processing-state machine and its CEIDs;
  - RCMD parameter semantics.

---

## 6. Object machines (E39, E87, E40, E94, E90)

The object standards are skeleton capabilities, not GEM fundamentals ([ve-decisions.md](ve-decisions.md) V-10):
a tool has only those its GEM manual declares,
and for those the standard is the prior while the vendor manual and logs supply the delta (§5.2 and each subsection's per-tool delta).
Each base standard defines objects, state models and abstract services;
its `.1` subsidiary standard maps the services to SECS-II messages and the variables to formats.

Revisions read: E39-0703 (reapproved 1109) and E39.1-0703; E87-0312 and E87.1-0702; E40-0813 and E40.1-0304; E94-0713 and E94.1-1101; E90-0312 and E90.1-0703.
Several `.1` revisions predate their base standard and lag it; each subsection lists where they disagree.
A tool may claim older revisions than these, so a revision difference is a per-tool question, not a defect of the tool.

Conventions shared by the object state models (E87 §7.2, E40 §5.1, E94 §6.2, E90 §6.1):
- **One CE per transition**, shared by every instance of the model; the report's object identifier tells instances apart.
  None of the standards assigns CEIDs, so every CEID is per-tool data.
- **Chained transitions:** when entering a state already satisfies the next transition, the machine takes it at once and still reports each transition in order.
- **Report values** are those after the transition, except at an object's extinction, where they are the last values before deletion.
- **Event style:** E30-style S6F11, or E53-style S6F11 or S6F13; E40 also allows its own S16F7/S16F9 notifications (6.3).

### 6.1 Object services (E39, E39.1)

Sources: SEMI E39-0703 (reapproved 1109), E39.1-0703, E5-0813 §9.8 and §10.18.
All S14 messages are `H<->E`, and every odd function carries "reply", so W=1 on all primaries (E5 §10.18).
A passive VE normally answers host-initiated S14 primaries (inference).

Object model (E39 §8, §9.3; E5 §9.8.9; E39.1 §6):
- **Fundamental attributes:** every public object has `ObjType` and `ObjID`, both RO text (E39 §8.3, Table 1).
  - Both use format 20; `ObjType` is fixed per object, and `ObjID` cannot be changed through S14F3 (E5 §9.8.9).
  - The data items OBJTYPE, OBJID and ATTRID also admit `5()` (E5 data items).
- **Text limits (E39.1 §6):** ASCII 0x20–0x7E, excluding `>`, `:`, `?`, `*` and `~`.
  A leading or trailing space is not allowed.
  - ATTRID and OBJTYPE take at most 40 characters; OBJID takes 1–80, and a zero-length OBJID means "not specified".
  - Comparison is case-insensitive; case should be preserved (E39 §8.2.3.4).
- **Uniqueness:** ObjType plus ObjID is unique only below one owner (E39 §8.2.5.2).
- **Object specifier (OBJSPEC, format 20):** a path of `type:id>` segments, each owning the next (E39 §9.3.3–9.3.4; E5 OBJSPEC).
  - The path may start at any object owned by the communications partner, but it may not skip a level (§9.3.6).
  - The type may be dropped when ids are unique (`id1>id2>`) or when context fixes it (§9.3.7–9.3.8).
  - A trailing `>` may be dropped, so a bare id works as a specifier (§9.3.9).
  - The `TypeA@TypeB:id` form for an inherited part is an optional capability (§9.4); inheriting objects shall use it (§14.3.3).
  - A zero-length OBJSPEC means "none given" (E5 S14F1 and others).
- **Which object OBJSPEC names:** the *owner* of the targets in S14F1/F3/F5/F7/F25/F27 (E39 §11.2.2); the owner in S14F9 and the new object in S14F10 (Table 10).
  It names the target itself in S14F11/F13/F15 (Tables 11–14), and the supervisor in S14F17, where TARGETSPEC names the attached object (Table 16).
- **Attribute forms (E39 §4.2.4):** integer kinds, float, enumerated, boolean, text and formatted text, structure, list and ordered list.
  ATTRDATA may be format 0, 10, 20, 3(), 4(), 5() or 11 (E5 ATTRDATA).
- **Equipment as an object:** SVs and DVVALs may be read as RO attributes, and ECVs as RW attributes, of type `Equipment` (E5 §9.8.12).

Relationships (E39 §9.2, §10.2, §10.3):
- **Ownership:** an aggregate (composed of), a container (contains) or a supervisor (supervises) owns its subordinates.
- **Lifecycle:** Create and Delete are optional and object-specific; owners may also create or delete transient objects on their own.
  - A Create whose owner is absent, or lies outside the receiving entity, uses OBJSPEC `""` (or `Equipment`) (§10.2.1.5).
- **Attachment:** binds an object to a single supervisor, which then owns it.
  - The object issues a private, non-zero AttachToken; the token is 0 while the object is unattached.
  - A second Attach to an attached object is denied (§10.3.3.5).
  - After DetachSupervisedObject, the supervisor treats the target as detached, whatever the target replies (§10.3.8).

Services mapped to SECS-II (E39.1 Table 1; bodies from E5 §10.18).
`ST` = `L,2 {OBJACK, L,p {L,2 {ERRCODE, ERRTEXT}}}`; `AS` = `L,n {L,2 {ATTRID, ATTRDATA}}`.

| Service | Pair (blk) | Request body | Reply body | Ack / notes |
|---|---|---|---|---|
| GetAttr | S14F1/F2 (S/M) | `L,5 {OBJSPEC, OBJTYPE, L,i {OBJID}, L,q {L,3 {ATTRID, ATTRDATA, ATTRRELN}}, L,a {ATTRID}}` | `L,2 {L,n {L,2 {OBJID, AS}}, ST}` | n=0: nothing matched; attributes in request order |
| SetAttr | S14F3/F4 (S/M) | `L,4 {OBJSPEC, OBJTYPE, L,i {OBJID}, AS}` | `L,2 {L,i {L,2 {OBJID, AS}}, ST}` | an empty AS for an object means "not found"; the reply echoes the resulting values |
| GetType | S14F5/F6 (S/—) | `<OBJSPEC>` | `L,2 {L,n {OBJTYPE}, ST}` | n=0: the owner has no object types |
| GetAttrName | S14F7/F8 (S/M) | `L,2 {OBJSPEC, L,n {OBJTYPE}}` | `L,2 {L,n {L,2 {OBJTYPE, L,a {ATTRID}}}, ST}` | wildcards allowed in OBJTYPE |
| Create | S14F9/F10 (M/M) | `L,3 {OBJSPEC, OBJTYPE, AS}` | `L,3 {OBJSPEC, AS, ST}` | zero-length OBJSPEC in the reply: nothing created |
| Delete | S14F11/F12 (S/M) | `L,2 {OBJSPEC, AS}` | `L,2 {AS, ST}` | attributes depend on the type (e.g. a delete token, §10.2.1.4) |
| Attach, Reattach | S14F13/F14 (M/M) | `L,2 {OBJSPEC, AS}` | `L,3 {OBJTOKEN, AS, ST}` | OBJTOKEN=0 exactly when p>0 |
| AttachSetAttr, Detach | S14F15/F16 (M/M) | `L,4 {OBJSPEC, OBJCMD, OBJTOKEN, AS}` | `L,2 {AS, ST}` | OBJCMD: 1 attach, 2 detach (token), 3 reattach, 4 set attributes (token) |
| Attach/DetachSupervisedObject | S14F17/F18 (S/M) | `L,4 {OBJSPEC, OBJCMD, TARGETSPEC, AS}` | `L,2 {AS, ST}` | OBJCMD codes as for S14F15 (E5 OBJCMD); 1 attach, 2 detach here is inference |
| ObjectAction | S14F19/F20 (M/M) | `L,5 {DATAID, OPID, OBJSPEC, SVCNAME, L,m {L,2 {SPNAME, SPVAL}}}` | `L,4 {SVCACK, LINKID, L,n {L,2 {SPNAME, SPVAL}}, L,2 {SVCACK, L,p {…}}}` | both SVCACKs equal; the legacy `L,3` form is not for new work |
| ObjectActionCompletion | S14F21/F22 (M/S) | `L,5 {DATAID, OPID, LINKID, L,n {L,2 {SPNAME, SPVAL}}, L,2 {SVCACK, L,p {…}}}` | `<DATAACK>` | only after SVCACK=4; E39 Table 8 types it N, yet E39 Table 19, E39.1 Table 1 and E5 all give it a response, so S14F22 is sent; DATAACK 0 ok, 1 unknown DATAID, 2 bad parameter |
| (multi-block inquire) | S14F23/F24 (S/S) | `L,2 {DATAID, DATALENGTH}` | `<GRANT>` | precedes any multi-block S14 message; S14F19 and F21 name it explicitly |
| GetServiceNames | S14F25/F26 (S/S) | `L,2 {OBJSPEC, L,n {OBJTYPE}}` | `L,2 {L,n {L,2 {OBJTYPE, L,a {SVCNAME}}}, ST}` | see caveats below |
| GetServiceParameters | S14F27/F28 (S/S) | `L,3 {OBJSPEC, OBJTYPE, L,n {SVCNAME}}` | `L,2 {L,n {L,2 {SVCNAME, L,a {SPNAME}}}, ST}` | — |

Codes (E5 data items):
- **OBJACK:** 0 success, 1 error; it describes the reply as a whole (E39 §11.5.10).
- **ERRCODE (subset relevant to S14; the selection is mine):** 1 unknown object in the specifier, 2 unknown target type, 3 unknown instance, 4 unknown attribute name, 5 read-only attribute, 6 unknown object type, 7 invalid attribute value, 11 identifier in use, 14 unsupported option, 15 busy.
  E39 Table 3 names only the meanings of 1–5.
- **ERRTEXT:** at most 120 characters.
- **SVCACK:** 0 performed, 1 no such service, 2 cannot perform now, 3 bad parameter, 4 accepted with completion reported later, 5 prohibited or not completed, 6 no such object.
- **ATTRRELN (U1):** 0 `=`, 1 `≠`, 2 `<`, 3 `≤`, 4 `>`, 5 `≥`, 6 in the set, 7 not in the set.
  An omitted relation means equality.
- **OPID and LINKID:** OPID may be 0 only for a quick service.
  LINKID is non-zero exactly when more completion reports with that OPID will follow (E5 S14F20–F21; E5 LINKID).

Rules a simulator follows (E39 §11.3, §11.5, §14; E5 S14F1–F4):
- **Empty means all:** in S14F1, i=0 applies only the filter; q=0 means no filter; i=q=0 selects every instance; a=0 returns every public attribute.
- **Filter logic:** the OBJID list is an OR pre-filter; qualifiers are ANDed.
  Each qualifier tests `ATTRDATA ATTRRELN value`, with the qualifying value on the left (E5 S14F1).
- **Wildcards (text only):** `?` matches one character; an embedded `*` matches any run, even an empty one, but a lone `*` needs at least one character (E39 §11.3.1–11.3.3; E5 S14F1).
  Matching is case-insensitive (E39 §11.3.4).
- **Omitted OBJTYPE:** S14F1 may omit OBJTYPE only when ids are unique across types and the id list is non-empty (E5 S14F1).
  The slot is fixed (E5 §10.3.1.3), so "omitted" means a zero-length item (inference).
- **Types of all subordinates:** requesting attribute `ObjType` with OBJTYPE omitted returns the types of all subordinates (E39 Table 4).
- **Read-only:** S14F3 on an RO attribute is rejected with ERRCODE 5, naming the attribute (E39 §11.5.9).
- **Partial success:** on OBJACK=1, returned values may be invalid (GetAttr), or unchanged (SetAttr) (E39 §11.5.4–11.5.5).
  - SetAttr need not be all-or-nothing: part of the settings may stay unchanged (E39 §11.5.5), and S14F4 carries an error for each attribute not set (E5 S14F4).
- **SetAttr selection:** the SECS-II SetAttr has no qualifier list, so it selects by OBJID only (E5 S14F3).
  S14F3 applies to all matching objects of the type, but no exception line defines i=0; reading it as every instance is inference.
- **Minimum compliance (E39 §14.2):** ObjType, ObjID, GetAttr and SetAttr.
  - Scope and filters may be ignored, but their presence must not produce an error (§14.2.2).
  - Owner objects must support scope, filters and GetType with wildcards (§14.3.2).
- **Order:** replies keep the order of the request's lists (E39 §5.3.2.4).

Object types exposed through these services (pointer list):
- `Carrier`: E87.1 Table 5.
- `ProcessJob`: E40.1 Table 3.
- `ControlJob`: E94.1 Table 6; it is created by S14F9/F10 (E94.1 Table 1).
- `Substrate`, `SubstLoc` and `BatchLoc`: E90.1 Tables 6–8.
  Register is S14F9/F10, remove is S14F11/F12, and `UpdateSubstrateObject` is S14F19/F20 (E90.1 Table 1).
- `EPTTracker`: E116 §10, Table 3.
- `Equipment` (SV, DVVAL and ECV as attributes) and the other entries of the E5 §9.8 object dictionary.

Caveats (source ambiguities):
- **ATTRRELN has fewer values in E5:** E39 Table 3 lists ten relations, and E5 ATTRRELN encodes eight (0–7).
  E5 labels 6 and 7 "present"/"absent" and glosses them as contained/not contained in the set, so E39's Present/Contained and Absent/Not Contained pairs share a code.
- **The E39 §11.3.6 filter example contradicts itself:** it uses "less than", yet it counts equality as qualifying.
  The E5 S14F1 direction is taken as normative.
- **Rejecting a specifier contradicts §14.2.2:** E39 §11.5.6 says a provider without owners rejects an OBJSPEC.
  Under §14.2.2 such a provider must not error.
  The VE follows §14.2.2 (inference).
- **GetType reports no types two ways:** E39 §13.5 reports "unknown object type", while E5 S14F6 returns n=0.
  ERRCODE 2 and 6 overlap in name.
- **E39.1 Table 2 names a data item E5 never uses:** it maps AttrData to `ATTRVAL`, but every E5 S14 body uses `ATTRDATA`.
- **The supervised-object messages are underspecified:** E39 Table 17 marks DetachSupervisedObject's ObjStatus "reserved", yet S14F18 carries `ST`.
  OBJCMD 1 and 3 overlap S14F13, which is where E39.1 maps Reattach.
- **Body slips in E5:** S14F6 prints no block or direction line, so its reply cell shows "—".
  The S14F2 and S14F8 exception lines mention an OBJSPEC that the reply lacks; for S14F8, "no objects matched" probably means n=0 (inference).
  S14F26 wraps `ST` in an extra one-item `L,2`; expect a plain `ST` (inference).

Per-tool delta for object services:
- Which ObjTypes and attributes are exposed; which attributes are RO and which are RW; any vendor-specific types.
- Whether OBJSPEC is parsed, ignored or rejected; whether filters and wildcards are supported.
- Whether S14F3 is honoured, or always answered OBJACK=1 with ERRCODE 5.
- Error-code habits: 3 versus 6 for an unknown object; 4 for an unknown attribute; whether ERRTEXT is filled.
- Response to an unsupported S14 function: S9F5, S14F0, or a reply with an error.
- Whether S14F23/F24 precedes a multi-block S14F19; whether SVCACK=4 with S14F21 occurs.
- How ObjID values are formed (case, length) for carriers, jobs and substrates.

### 6.2 Carrier management (E87, E87.1)

Sources: SEMI E87-0312 (CMS), E87.1-0702 (SECS-II mapping), E5-0813 Stream 3 and data-item dictionary.
"DV" lists the data a transition's event report must carry; a bare "CE" carries the port or carrier identifier plus the new state variable, and the cited table holds the exact list.

Caveats:
- E87 §16.1.1 promises a service state-mapping table that neither source file contains;
  the accept/reject column in 6.2.5 is assembled from §16.4 prose and the transition tables.
- E87.1-0702 predates E87-0312: it does not map BypassReadID, and its BufferPartitionInfo is `L,4` against 5 members in E87 Table 37.
  Neither standard lists the §15 CarrierHold and UnclampControl selections as variables.
- Neither E87.1 nor E5 enumerates the CARRIERACTION or PORTACTION strings, or says where S3F25 carries ServiceStatus (E87.1 Table 2 gives only U1 0 OUT OF SERVICE, 1 IN SERVICE).
- E87.1 Table 2 maps the AccessMode parameter to PORTACCESS, while its Table 3 and E5 use ACCESSMODE in S3F27; ACCESSMODE is used below.
- Table 7 spells the failed final states "... FAIL"; the attribute and variable enumerations spell them "... FAILED".

#### 6.2.1 Load Port Transfer State Model, per load port (E87 §9.5, Figure 1, Table 5)

Hierarchy: LOAD PORT TRANSFER: {OUT OF SERVICE, IN SERVICE: {TRANSFER READY: {READY TO LOAD, READY TO UNLOAD}, TRANSFER BLOCKED}}.

| # | From | Trigger | Guard | To | Action / messages |
|---|---|---|---|---|---|
| 1 | (no state) | system reset | state held before reset (history) | OUT OF SERVICE or IN SERVICE | CE; DV PortID, PortTransferState |
| 2 | OUT OF SERVICE | ChangeServiceStatus IN SERVICE (host or operator) | — | IN SERVICE | CE |
| 3 | IN SERVICE | ChangeServiceStatus OUT OF SERVICE | — | OUT OF SERVICE | CE; a later transfer attempt raises an alarm (Attempt To Use Out Of Service Load Port, inference from Table 38) |
| 4 | (entry IN SERVICE) | #2, or re-initialization | port and carrier available for transfer | TRANSFER READY, else TRANSFER BLOCKED | CE |
| 5 | (entry TRANSFER READY) | #4, re-initialization, or #10 | carrier present | READY TO UNLOAD, else READY TO LOAD | CE; DV PortID (RTL); PortID, CarrierID, PortTransferState (RTU) |
| 6 | READY TO LOAD | load begins: user-configured manual trigger (Table 8), E84 PIO READY, or CarrierOut starts (internal buffer) | — | TRANSFER BLOCKED | CE |
| 7 | READY TO UNLOAD | unload begins: manual trigger, PIO READY, CarrierIn starts, or CarrierReCreate issued | — | TRANSFER BLOCKED | CE |
| 8 | TRANSFER BLOCKED | unload done: no presence plus operator confirmation (manual), PIO COMPT (auto), or carrier moved into the internal buffer | internal buffer: no CarrierOut queued for this port | READY TO LOAD | CE |
| 9 | TRANSFER BLOCKED | carrier back at the load/unload position after processing, CancelCarrier or CancelCarrierAtPort; or moved there from the internal buffer | — | READY TO UNLOAD | CE; DV PortID, CarrierID, PortTransferState |
| 10 | TRANSFER BLOCKED | transfer failed; carrier neither loaded nor unloaded | — | TRANSFER READY (substate by #5) | CE |

Rules (§7.2.2, §9.2–9.3):
- **One CEID per transition**, shared by all ports; the report's PortID tells them apart.
- **Conditional entries** (#1, #4, #5) may be reported once the leaf state is reached, listing each transition taken.
- **PortTransferState** reports leaf states only, U1: 0 OUT OF SERVICE, 1 TRANSFER BLOCKED, 2 READY TO LOAD, 3 READY TO UNLOAD (E87.1 Table 4).
- **Numbering:** ports count left to right along the bottom row, then the upper row, facing the front; slots count from 1 at the bottom; PortID is PTN, U1 1–255 (E87.1 Table 2).

#### 6.2.2 Carrier State Model, per carrier object (E87 §10.7, Figure 2, Table 7)

CARRIER has three AND substates; the final states are marked (F):
- CARRIER ID STATUS: {ID NOT READ, WAITING FOR HOST, ID VERIFICATION OK (F), ID VERIFICATION FAILED (F)}
- CARRIER SLOT MAP STATUS: {SLOT MAP NOT READ, WAITING FOR HOST, SLOT MAP VERIFICATION OK (F), SLOT MAP VERIFICATION FAILED (F)}
- CARRIER ACCESSING STATUS: {NOT ACCESSED, IN ACCESS, CARRIER COMPLETE (F), CARRIER STOPPED (F)}

| # | From | Trigger | Guard | To | Action / messages |
|---|---|---|---|---|---|
| 1, 12, 17 | (no state) | object instantiated | — | CARRIER; SLOT MAP NOT READ; NOT ACCESSED | no CE required |
| 2 | (no state) | Bind or CarrierNotification | — | ID NOT READ | CE; DV CarrierID, CarrierIDStatus |
| 3 | (no state) | read of a CarrierID no object holds (host-based), or read after Bind but equipment verification fails | — | WAITING FOR HOST | CE; DV CarrierID, PortID, CarrierIDStatus |
| 4 | (no state) | ProceedWithCarrier after CarrierID Read Fail or UnknownCarrierID | no Bind received | ID VERIFICATION OK | create object with the service's CarrierID; CE |
| 5 | (no state) | CancelCarrier after the same events | no Bind received | ID VERIFICATION FAILED | create object with the service's CarrierID; CE |
| 6 | ID NOT READ | read matches the expected ID; or, for a CarrierNotification object, ProceedWithCarrier with PortID and CarrierID after a read fail | — | ID VERIFICATION OK | CE; DV PortID, CarrierID, CarrierIDStatus |
| 7 | ID NOT READ | read fails | — | WAITING FOR HOST | CE; DV PortID, CarrierID, CarrierIDStatus (also #8–11) |
| 8 | WAITING FOR HOST | ProceedWithCarrier | — | ID VERIFICATION OK | CE |
| 9 | WAITING FOR HOST | CancelCarrier | — | ID VERIFICATION FAILED | CE |
| 10 / 11 | ID NOT READ | carrier arrives, reader out of service or absent | BypassReadID FALSE / TRUE | WAITING FOR HOST (await ProceedWithCarrier) / ID VERIFICATION OK (keep the Bind's ID) | CE |
| 13 | SLOT MAP NOT READ | map read and matched by the equipment | — | SLOT MAP VERIFICATION OK | CE; DV PortID, CarrierID, LocationID, CarrierAccessingStatus, SlotMapStatus |
| 14 | SLOT MAP NOT READ | map read for host check; equipment check failed; read failed; or bad substrate position | — | WAITING FOR HOST | store the read map in SlotMap; CE; DV PortID, CarrierID, LocationID, SlotMap, Reason, SlotMapStatus |
| 15 | WAITING FOR HOST | ProceedWithCarrier | — | SLOT MAP VERIFICATION OK | continue with the carrier; CE; DV PortID, CarrierID, LocationID, SlotMapStatus |
| 16 | WAITING FOR HOST | CancelCarrier | — | SLOT MAP VERIFICATION FAILED | make the carrier ready for unload; CE; DV as #13 |
| 18 | NOT ACCESSED | equipment starts using the carrier | — | IN ACCESS | CE; DV CarrierID, CarrierAccessingStatus |
| 19 | IN ACCESS | access ends normally | — | CARRIER COMPLETE | CE |
| 20 | IN ACCESS | access ends abnormally | — | CARRIER STOPPED | CE |
| 21 | CARRIER | carrier unloaded; CancelBind or CancelCarrierNotification before load; equipment self-CancelBind after failed equipment verification | — | (no state) | destroy object; CE; DV CarrierID |

Rules (§10.2–10.7, §19.2.1):
- **Instantiation** needs only one CE, carrying the entry state of all three substates (Table 7 note).
- **Created by:** Bind, CarrierNotification, CarrierReCreate with attributes; a read of an ID no object holds; ProceedWithCarrier or CancelCarrier at a NOT ASSOCIATED port.
- **Destroyed by:** unload; CancelBind or CancelCarrierNotification; failed equipment ID verification after Bind; CarrierReCreate.
- **ObjID equals CarrierID**, unique at the equipment; an object created by ID read has ContentMap `L,0`, SlotMap all UNDEFINED, and SubstrateCount null if unknown.
- **The slot map is read before any substrate leaves the carrier.**
- **Read fail vs unavailable reader:** a fail is a transient error of a working reader; an unavailable reader is a standing condition handled by BypassReadID (#10, #11).
- **Attribute values in reports** are post-transition, except at destruction (#21), where they are pre-transition.
- **Encodings (E87.1 Table 5):** Capacity U1 1–25; SubstrateCount U1 0–25; ContentMap `L,n {L,2 {LotID, SubstID}}`;
  SlotMap `L,n` of U1: 0 UNDEFINED, 1 EMPTY, 2 NOT EMPTY, 3 CORRECTLY OCCUPIED, 4 DOUBLE SLOTTED, 5 CROSS SLOTTED;
  the three status attributes are U1 0–3 in the listed order; Reason U1: 0 VERIFICATION NEEDED, 1 VERIFICATION BY EQUIPMENT UNSUCCESSFUL, 2 READ FAIL, 3 IMPROPER SUBSTRATE POSITION.

#### 6.2.3 Access Mode, Reservation and Association, per load port (E87 §11–13; Tables 9–11)

| Model | # | From | Trigger | Guard | To | Action / messages |
|---|---|---|---|---|---|---|
| Access | 1 | (no state) | system restart | mode before reset (history, non-volatile) | MANUAL or AUTO | CE; DV PortID, AccessMode |
| Access | 2 / 3 | MANUAL / AUTO | ChangeAccess to the other mode (host or operator) | no transfer in progress; port not RESERVED | AUTO / MANUAL | CE |
| Reserv., Assoc. | 1 | (no state) | system reset | — | NOT RESERVED; NOT ASSOCIATED | no CE required |
| Reserv. | 2 | NOT RESERVED | ReserveAtPort or Bind; or equipment physically starts a CarrierOut | — | RESERVED | visible signal on, if configured; CE; DV PortID, LoadPortReservationState, optionally CarrierID (Bind, CarrierOut) |
| Reserv. | 3 | RESERVED | CancelReservationAtPort or CancelBind; or a carrier arrives | — | NOT RESERVED | visible signal off; CE |
| Assoc. | 2 | NOT ASSOCIATED | Bind on an empty port; ProceedWithCarrier or CancelCarrier after CarrierID Read Fail or UnknownCarrierID; read of an ID no object holds; CarrierOut moving a known carrier to the port | — | ASSOCIATED | CE; DV PortID, CarrierID, PortAssociationState |
| Assoc. | 3 | ASSOCIATED | CancelBind before arrival or transfer start; carrier removed; carrier moved into the internal buffer | — | NOT ASSOCIATED | CE; DV PortID, PortAssociationState |
| Assoc. | 4 | ASSOCIATED | equipment ID verification fails (Bind only); or (internal buffer) carrier unloaded and a queued CarrierOut starts | — | ASSOCIATED | the read ID replaces the bound ID; hold until CancelCarrier or ProceedWithCarrier; CE; DV PortID, CarrierID, PortAssociationState |

Rules:
- **ChangeAccess to the current mode** is accepted with no CE (§11.3.3).
- **Transfer boundaries (Table 8):** AUTO runs from E84 READY to COMPT; MANUAL boundaries are user-configured (sensor, door, switch, timer).
- **Manual transfer in AUTO (§11.3.3.3.1):** the mode stays AUTO, and AccessModeViolation is set.
  A removal from READY TO UNLOAD clears it on reaching READY TO LOAD.
  A placement on READY TO LOAD leaves the transfer open until the carrier is removed or the operator chooses "continue" (which configuration can disable); either ends the alarm.
- **Reservation is required for internal-buffer equipment** and optional for fixed-buffer equipment (§12.1.2–12.1.3).
  A reservation made by a physical CarrierOut cannot be cancelled by CancelReservationAtPort (§16.4.9).
- **An ID read that creates an association** must be reported with the CarrierID in a CE (§13.1.1).
- **Encodings (E87.1 Table 4):** AccessMode U1 0 MANUAL, 1 AUTO; LoadPortReservationState U1 0/1; PortAssociationState U1 0 NOT ASSOCIATED, 1 ASSOCIATED.

#### 6.2.4 Verification, release and carrier-out queue (E87 §14, §15, §16.4.12, §17, §20.3)

- **Who verifies (§14.1):** the equipment, when the host supplied the expected value before the read; otherwise the host, from the reported value.
  The expected ID comes with Bind or CarrierNotification; an expected slot map comes with Bind, CarrierNotification or ProceedWithCarrier, and only in equipment-based mode (§10.3.5).
- **Equipment ID check (Table 12):** a match is Carrier #6, and the equipment carries on unprompted.
  A mismatch after Bind triggers a self-CancelBind and a new object holding the read ID (#21, #3, Assoc #4), plus the alarm Carrier Verification Failure (§20.4);
  the carrier stays closed and outside the buffer until ProceedWithCarrier or CancelCarrier names the read ID.
- **Host ID check**, also used for an unexpected carrier after CarrierNotification (which binds no port): the read ID goes out with Carrier #3;
  the carrier is held until ProceedWithCarrier, or CancelCarrier / CancelCarrierAtPort on failure.
- **Slot map (Table 13):** equipment-based gives #13 on a match, or #14 (Reason 1) with the alarm Slot Map Verification Failed (§20.5);
  host-based gives #14 (Reason 0), and the host answers ProceedWithCarrier (#15) or CancelCarrier (#16).
  Host-based mode thus uses two ProceedWithCarrier calls, one after the ID read and one after the map read (§16.4.19.2).
- **Reservation and verification (Table 14):** Bind = reservation + equipment ID check; ReserveAtPort = reservation + host ID check; CarrierNotification = equipment ID check only; none = host ID check.
  In the host-ID cases, a ProceedWithCarrier carrying a SlotMap makes the map check equipment-based.
- **Duplicate CarrierID (§20.3):** set the alarm; skip the second carrier, and the first too unless it has started, in which case send CE Duplicate CarrierID In Process.
- **CarrierHold (§15.2, §17):** Host Release keeps the carrier at the tag write position until CarrierRelease;
  Equipment Release frees it on CARRIER COMPLETE or STOPPED and ignores CarrierRelease.
  Release allows a move but does not force one; undocking waits for CARRIER COMPLETE or STOPPED and a closed door (§15.3, §17).
- **UnclampControl (§15.4):** unclamp on CARRIER COMPLETE or STOPPED, or (AUTO only) stay clamped until the AMHS starts its PIO unload; no effect in MANUAL.
- **CarrierOut queue (internal buffer, §16.4.12):** room for as many entries as internal buffer locations plus internal FIMS ports, FIFO per port, and the equipment picks the port when none is given.
  An entry starts once current substrate handling ends and the port is NOT ASSOCIATED, and starting it makes the port TRANSFER BLOCKED and ASSOCIATED.
  CancelCarrierOut drops one entry and CancelAllCarrierOut drops all; CarrierIn is refused unless CarrierOut brought that carrier out (§16.4.10).

#### 6.2.5 Services to SECS-II (E87.1 Tables 1–3; E87 §16; E5 Stream 3)

Bodies (E5; every request W=1; S3F20, S3F26 and S3F32 reuse the S3F18 body):
- H->E S3F17 (M) `L,5 {DATAID, CARRIERACTION, CARRIERID, PTN, L,n {L,2 {CATTRID, CATTRDATA}}}` / E->H S3F18 `L,2 {CAACK, L,n {L,2 {ERRCODE, ERRTEXT}}}`.
  PTN may be zero-length when CARRIERID is given, and unused items are sent zero-length (E87.1 §5.3 note 2);
  a multi-block S3F17 needs S3F15/F16 first.
- H->E S3F19 is header only; H->E S3F25 is `L,3 {PORTACTION, PTN, L,m {L,2 {PARAMNAME, PARAMVAL}}}`.
- H->E S3F27 `L,2 {ACCESSMODE, L,n {PTN}}` (n=0: all ports) / E->H S3F28 `L,2 {CAACK, L,n {L,3 {PTN, ERRCODE, ERRTEXT}}}`; partial success is CAACK=6 with n>0.
- H->E S3F29 `L,4 {LOCID, CARRIERSPEC, DATASEG, DATALENGTH}` / E->H S3F30 `L,2 {DATA, L,2 {CAACK, L,s {L,2 {ERRCODE, ERRTEXT}}}}`;
  H->E S3F31 adds DATA as a fifth item.
- **CAACK:** 0 done, 1 invalid command, 2 cannot perform now, 3 invalid data, 4 accepted with completion signalled by a later event, 5 invalid state, 6 done with errors (E5 only).
  CAACK 4 is allowed only for CarrierOut, CarrierIn, CancelCarrier, CancelCarrierAtPort, ChangeServiceStatus and ChangeAccess, with a supplier-documented completion CE from Table 17;
  every other service answers 0 on success (§16.3.2).
- **ERRCODE used by CMS:** 3 unknown object, 4 unknown attribute, 7 invalid attribute value, 11 identifier in use, 12 bad parameters, 13 missing parameters, 14 unsupported option, 17 invalid in current state, 48 no such port, 49 port in use, 50 missing carrier.

| Service | SxFy | Action value (inference) | Key parameters | Accepted when / rejected when |
|---|---|---|---|---|
| Bind | S3F17 | "Bind" | PTN, CARRIERID, attrs (C) | rejects an ID already instantiated (11), an unknown port (48), or a port in use (49) |
| CancelBind | S3F17 | "CancelBind" | PTN or CARRIERID | before the carrier arrives or the transfer starts (Assoc #3) |
| CarrierNotification | S3F17 | "CarrierNotification" | CARRIERID, attrs (C) | rejects an ID already instantiated (11) |
| CancelCarrierNotification | S3F17 | "CancelCarrierNotification" | CARRIERID | before load (Carrier #21) |
| ProceedWithCarrier | S3F17 | "ProceedWithCarrier" | CARRIERID, PTN (C), attrs (C) | ID or slot map WAITING FOR HOST, or a NOT ASSOCIATED port after a read fail (Carrier #4, #6, #8, #15); otherwise 5 (inference) |
| CancelCarrier | S3F17 | "CancelCarrier" | CARRIERID, PTN (C) | rejected once substrates have left for processing (§16.4.5) |
| CancelCarrierAtPort | S3F17 | "CancelCarrierAtPort" | PTN | rejected for no such port, no carrier, or substrates removed (§16.4.6) |
| CarrierOut / CancelCarrierOut / CancelAllCarrierOut | S3F17 / S3F17 / S3F19 | "CarrierOut" / "CancelCarrierOut" / — | CARRIERID, PTN (C) / CARRIERID / none | internal buffer only; CancelCarrierOut needs a queued entry |
| CarrierIn | S3F17 | "CarrierIn" | CARRIERID | only after a CarrierOut of that carrier |
| CarrierRelease | S3F17 | "CarrierRelease" | PTN or CARRIERID | denied when PTN and CARRIERID disagree |
| CarrierReCreate | S3F17 | "CarrierReCreate" | CARRIERID, attrs (C) | only in READY TO UNLOAD; attrs present means equipment-based, absent means host-based |
| ReserveAtPort / CancelReservationAtPort | S3F25 | "ReserveAtPort" / "CancelReservationAtPort" | PTN | ReserveAtPort: 49 if the port is in use; Cancel: not for a CarrierOut reservation |
| ChangeServiceStatus | S3F25 | unspecified (see caveats) | PTN | 48 for an unknown port |
| ChangeAccess | S3F27 | ACCESSMODE 0/1 | PTN list | per port: no transfer in progress and not RESERVED; partial success is CAACK 6 |
| CarrierTagReadData / WriteData | S3F29 / S3F31 | — | LOCID or CARRIERSPEC, DATASEG, DATALENGTH, DATA | only at the read or write position; denied when LOCID and CARRIERSPEC disagree |

#### 6.2.6 Required events, variables and alarms (E87 §7.2.2, §18–20; E87.1 Table 4)

- **State-model CEs**, one CEID per transition shared across instances: Load Port Transfer #1–10; Carrier #2–11, #13–16, #18–21; Access Mode #1–3; Reservation #2–3; Association #2–4.
- **Additional CEs (§18):** Buffer Capacity Changed; Carrier Approaching Complete; Carrier Clamped; Carrier Unclamped; Carrier Opened; Carrier Closed; Carrier Location Change; CarrierID Read Fail; ID Reader Available; ID Reader Unavailable; UnknownCarrierID; Duplicate CarrierID In Process.
  Clamp events apply only to ports that clamp, door events only to carriers with doors, and buffer events only to internal buffers (§18.2.1; its BufferPartitionInfo is internal-buffer only, Table 37).
- **DVVAL:** AccessMode, CarrierAccessingStatus, CarrierID, CarrierIDStatus, LoadPortReservationState, LocationID, PortAssociationState, PortID, PortStateInfo, PortTransferState, Reason, SlotMap, SlotMapStatus;
  internal buffer only: AvailPartitionCapacity, BufferPartitionInfo, PartitionCapacity, PartitionID, PartitionType, UnAllocatedPartitionCapacity.
- **SV:** a per-port or per-location form (subscript i) of each DVVAL above that has one;
  plus the lists CarrierLocationMatrix, LoadPortReservationStateList, PortAssociationStateList, PortStateInfoList, PortTransferStateList, and BufferCapacityList (internal buffer).
  In CarrierLocationMatrix, an empty location has a null CarrierID and an occupied location with an unknown ID has "UNKNOWN".
  BypassReadID is a read-write Boolean, default FALSE (§10.7.7); E87.1 gives it no class, presumably ECV (inference).
- **Alarms (Table 38, a minimum set):** PIO Failure; Access Mode Violation; Carrier Verification Failure; Slot Map Read Failed; Slot Map Verification Failed; Attempt To Use Out Of Service Load Port; Carrier Presence Error; Carrier Placement Error; Carrier Dock/UnDock Failure; Carrier Open/Close Failure; Duplicate CarrierID; Carrier Removal Error; Internal Buffer Carrier Move Failure (internal buffer only).

#### 6.2.7 Normal roundtrip, host-based verification (after E87 R1-2.2)

R1-2.2 assumes fixed-buffer equipment, a FOUP and host-based verification of both values, and lists only state changes.
The SECS-II mapping and the clamp, door, access and release steps come from Table 7, §15, §18 and E87.1 Table 1, not from R1-2.2.
Each CE is an E->H S6F11 / H->E S6F12, sent only if enabled (§2 above); the port starts READY TO LOAD and NOT ASSOCIATED, with no carrier object.

1. Load starts (PIO READY or the manual trigger): CE LP #6 TRANSFER BLOCKED.
2. Load completes; clamp: CE Carrier Clamped, if the port clamps.
3. ID read OK: CE Carrier #3 (ID WAITING FOR HOST, SLOT MAP NOT READ, NOT ACCESSED in one report) and CE Assoc #2 ASSOCIATED, in unspecified order (inference).
4. H->E S3F17 `L,5 {DATAID, "ProceedWithCarrier", <CarrierID>, <PTN>, L,0}` / E->H S3F18 `L,2 {0, L,0}`; CE Carrier #8 ID VERIFICATION OK (S3F18 vs S6F11 order unspecified).
5. Dock and open (CE Carrier Opened); map read: CE Carrier #14 with SlotMap and Reason=0.
6. H->E S3F17 ProceedWithCarrier, with no SlotMap attribute in host-based mode / E->H S3F18 CAACK=0; CE Carrier #15 SLOT MAP VERIFICATION OK.
7. First substrate taken: CE Carrier #18 IN ACCESS; last substrate returned: CE Carrier #19 CARRIER COMPLETE.
8. Close (CE Carrier Closed); with CarrierHold = Host Release, the host may use S3F29/S3F31 and then S3F17 "CarrierRelease" / S3F18.
9. Undock; carrier at the unload position: CE LP #9 READY TO UNLOAD; unclamp per UnclampControl (CE Carrier Unclamped).
10. Unload starts: CE LP #7 TRANSFER BLOCKED.
11. Unload completes: CE LP #8 READY TO LOAD, CE Assoc #3 NOT ASSOCIATED, CE Carrier #21 (object destroyed).

#### 6.2.8 Per-tool delta for carriers

| Item | What varies |
|---|---|
| Ports | number of load ports; input, output or both; PTN values; LocationID names (a FOUP port has separate delivery and dock locations, §10.5) |
| Buffer type | fixed or internal buffer: CarrierOut/CarrierIn/queue, partitions and buffer events are internal-buffer only; reservation is mandatory there, optional on fixed buffer |
| ID reader | present or not; read position; BypassReadID setting |
| Verification modes | which the host uses: Bind, CarrierNotification, ReserveAtPort, or none (Table 14) |
| Implemented services | which §16 services the tool actually accepts; which reply CAACK 4 and which CE marks their completion |
| Encodings | CARRIERACTION/PORTACTION spellings and case; ChangeServiceStatus encoding in S3F25 (see caveats) |
| IDs | CEID per transition and per §18 event; DVID/SVID numbers; ALIDs and any extra carrier alarms |
| Hardware and config | clamps, carrier door, docking; CarrierHold and UnclampControl settings; AUTO "continue" option; Table 8 manual boundaries |
| Carrier | Capacity; SlotMap values the mapper can detect (NOT EMPTY vs DOUBLE/CROSS SLOTTED); Usage values; Carrier Approaching Complete rule (§18.3.4) |

### 6.3 Process jobs (E40, E40.1)

Sources: E40-0813 (services and behaviour), E40.1-0304 (SECS-II mapping), E5-0813 §10.20 (Stream 16 bodies and data items).
Revision skew between E40-0813 and E40.1-0304:
- **PRSTATE codes:** E40.1 Table 3 gives 0 QUEUED/POOLED, 1 SETTING UP, 2 WAITING FOR START, 3 PROCESSING, 4 PROCESS COMPLETE, 5 reserved, 6 PAUSING, 7 PAUSED, 8 STOPPING, 9 ABORTING.
  E40 Table 2 also has STOPPED and ABORTED; no source here assigns them a code, and the E5 PRSTATE entry has no enumeration.
- **Start command name:** E40 Tables 5 and 19 spell it `STARTPROCESS`; the E5 PRCMDNAME entry spells it `START`.
- **ObjType value:** E40 Table 2 has "PROCESSJOB"; E40.1 Table 3 has "ProcessJob".
- **Multi-block inquire:** E40.1 §8.2 requires S16F1/F2 before a multi-block S16F3 or S16F5; E5 S16F1 requires it before any multi-block Stream 16 job message, and S16F11/F15 restate it.
- **Service set:** E40.1 Table 1 maps all twelve E40 Table 4 services; E5 S16F13/F14 (PRJobDuplicateCreate) are marked unused and have no E40 service.

Hierarchy (E40 §8.3, Figure 4):
- (no state) → QUEUED/POOLED
- ACTIVE, which contains:
  - EXECUTING: {SETTING UP, WAITING FOR START, PROCESSING}
  - PAUSE: {PAUSING, PAUSED}
  - STOPPING
  - ABORTING
- POST ACTIVE: {PROCESS COMPLETE, STOPPED, ABORTED} → (extinction)

| # | From | Trigger | Guard | To | Action / messages |
|---|---|---|---|---|---|
| 1 | (no state) | create request accepted | create validation passed | QUEUED/POOLED | job enters the queue/pool; create ack sent (S16F4/F12/F16) |
| 2 | QUEUED/POOLED | resource allocated to the job | needed material positions free or already holding the job's material (§8.3.2.10.2) | SETTING UP | leave the queue; milestone Setup; pre-condition; prepare material on arrival |
| 3 | SETTING UP | material present and resource ready | PRProcessStart = FALSE | WAITING FOR START | milestone Waiting for Start |
| 4 | SETTING UP | material present and ready | PRProcessStart = TRUE | PROCESSING | milestone Processing; process material |
| 5 | WAITING FOR START | START / STARTPROCESS command | — | PROCESSING | milestone Processing; process material |
| 6 | PROCESSING | material processing finished | — | PROCESS COMPLETE | milestone Processing Complete; post-condition; wait for material to leave |
| 7 | any POST ACTIVE substate | job material leaves the equipment, or (no control job) a job naming the same material supersedes this one | — | (extinction) | milestone Job Complete; delete the job |
| 8 | EXECUTING | PAUSE command or internal pause (e.g. a PauseEvent fires) | — | PAUSING | pause at the first convenient point |
| 9 | PAUSING | pause reached | — | PAUSED | none |
| 10 | PAUSE | RESUME (resource resumes) | — | EXECUTING, shallow history (H) | resume the paused activity |
| 11 | EXECUTING | STOP command or internal stop | — | STOPPING | stop at the first convenient point |
| 12 | PAUSE | STOP command or internal stop | — | STOPPING | as #11 |
| 13 | EXECUTING | ABORT command or internal abort | — | ABORTING | end current activity at once |
| 14 | STOPPING | ABORT command or internal abort | — | ABORTING | as #13 |
| 15 | PAUSE | ABORT command or internal abort | — | ABORTING | as #13 |
| 16 | ABORTING | abort (and any recovery) finished | — | ABORTED | substrates may be moved to a safe place first |
| 17 | STOPPING | stop finished, all material safe | — | STOPPED | none |
| 18 | QUEUED/POOLED | CANCEL, ABORT or STOP command | — | (extinction) | remove from queue/pool; milestone Job Complete; delete the job |

Rules the table does not show (E40 §5.1, §7, §8.2, §8.3.2):
- **Every transition is a state-change event** carrying at least PRJobID, PRJobState and Timestamp (§5.1.3, §8.2.2.4).
  Figure 4 numbers #1, #7 and #18 too, so they emit it as well (§8.2.2.4); the PRJobState value reported at extinction is not stated.
- **Already-satisfied conditions:** if a new state already meets the next transition's trigger, take it at once and still report each transition in order (§5.1.4).
- **SETTING UP with material already present:** only prepare it; if the recipe needs pre-conditioning without material, the job fails (§8.3.2.4.1).
  The failure path is not specified (inference: internal abort, #13 → #16 → #7).
- **WAITING FOR START is a safe state:** STOP or ABORT from it leaves material unaltered (§8.2.3.8).
- **Queue:** every job passes through QUEUED/POOLED (§8.3.2.10.2).
  Without queuing, at most one job may be there; with queuing, capacity is at least the number of load ports (§8.3.2.10).
  Jobs are listed in creation order (§7.2.6); activation order is FIFO, material-arrival or host-ordered, per tool (§8.3.2.10.1).
- **Phases:** pre-conditioning, preparation, processing, post-conditioning; only processing is mandatory (§7.2.1–§7.2.2).
- **Material groups:** one material type, identical processing; the whole group must arrive before processing and none may leave before all is done (§7.6).
- **Concurrent jobs:** independent, unique PRJobIDs, never sharing material; one active job per material (§7.7.2).
- **Consecutive jobs on the same material:** kept in arrival order; the next activates only after the previous finishes processing (§7.8).
  The previous job ends via #7 while the material is still present and the material moves to the new job (§7.8.2).
  If the previous job needs post-conditioning, the new job may not supersede it and stays queued (§8.3.2.9.1).
- **Job without material:** no material-arrival dependency; it ends when active processing ends (§7.9.1).
  E5 allows an empty material list only in S16F11/F15; E40 Table 7 makes material mandatory in PRJobCreate and a list needs one element unless zero is allowed (§5.3.2.4), so S16F3 cannot create one.
- **POST ACTIVE persistence:** with no control job, a finished job stays until its material leaves or a job on the same material replaces it (§8.3.2.13).
- **Mandatory minimum:** single job to completion, success/partial/failure reporting, ABORT at all times, Table 2 attributes, reject bad or unsupported requests (§12.2).
  Queuing, concurrency, pause/resume, stop, manual start, tuning and events are optional (§12.3).
  Milestones are optional per §12.3 and Table 20, yet §10.4.6 calls Setup, Processing, Processing Complete and Complete required (ambiguous).

ProcessJob attributes (E40 §9.2 Table 2; SECS-II form E40.1 §9 Table 3; read with E39 GetAttr S14F1/F2 per E40.1 §8.1; all RO, so SetAttr does not apply):

| Attribute | Rqmt | SECS-II | Meaning |
|---|---|---|---|
| ObjID | Y | PRJOBID (A) | job identifier, fixed at create |
| ObjType | Y | A | object type (spelling: see skew above) |
| PauseEvent | N | `L,n {CEID}`, CEID U() | events that put the job into PAUSING automatically |
| PRJobState | Y | PRSTATE (U1) | current substate per Figure 4 |
| PRMtlNameList | Y | MF 13: `L,n {L,2 {CARRIERID, L,j {SLOTID}}}`; MF 14: `L,n {MID}` | material processed by the job |
| PRMtlType | Y | MF (B): 13 carrier, 14 substrate | material type |
| PRProcessStart | N | Boolean | TRUE automatic start, FALSE manual start |
| PRRecipeMethod | Y | U1: 1 recipe only, 2 recipe with variable tuning | recipe method |
| RecID | Y | RCPSPEC (or PPID, ASCII) | recipe applied, unique within the tool |
| RecVariableList | N | `L,n {L,2 {RCPPARNM, RCPPARVAL}}` | tuning variables |

PRJobCommand set (E40 §8.2.3, §10.4.5, Table 5; E5 PRCMDNAME):

| Command | Valid in | Transition | Notes |
|---|---|---|---|
| ABORT | QUEUED/POOLED, EXECUTING, STOPPING, PAUSE | #18, #13, #14, #15 | only mandatory command; overrides STOP, CANCEL, PAUSE; on a queued job equals CANCEL |
| STOP | QUEUED/POOLED, EXECUTING, PAUSE | #18, #11, #12 | orderly; on a queued job equals CANCEL; from PAUSE it may effectively resume first so material ends fully processed or untouched (§8.2.3.5) |
| CANCEL | QUEUED/POOLED only | #18 | reject on any ACTIVE job (§8.2.3.4); no physical action |
| PAUSE | EXECUTING | #8 | — |
| RESUME | PAUSE (PAUSING or PAUSED) | #10 | — |
| START / STARTPROCESS | any time after create | #5 if in WAITING FOR START | otherwise only sets PRProcessStart (§8.2.3.9) |

Any command/state pair not listed has no Table 1 row, although §8.2.3.2, §8.2.3.3 and §8.2.3.5 allow ABORT, STOP and PAUSE at any time (inference: reject with ACKA=FALSE, ERRCODE 17).

Services → SECS-II (E40 §10.4 Tables 7–18; E40.1 §5 Table 1, §6 Table 2; E5 §10.20):

| Service | SxFy | Key parameters | Accept / reject |
|---|---|---|---|
| PRJobCreate | S16F3 / F4 | `L,5 {DATAID, MF, L,n {MID}, L,3 {PRRECIPEMETHOD, RCPSPEC\|PPID, L,m {L,2 {RCPPARNM, RCPPARVAL}}}, PRPROCESSSTART}`; reply `L,2 {PRJOBID, L,2 {ACKA, L,n {L,2 {ERRCODE, ERRTEXT}}}}` | tool assigns PRJOBID; validate recipe, material, start mode and variable ranges before replying (§8.2.1.2.1); reject 12 bad parameter, 13 missing parameter, 14 unsupported option, 15 busy / queue full |
| PRJobCreateEnh | S16F11 / F12 | `L,7 {DATAID, PRJOBID, MF, material, L,3 {recipe}, PRPROCESSSTART, PRPAUSEEVENT}` | host assigns PRJOBID; as S16F3 plus 11 identifier in use |
| PRJobMultiCreate | S16F15 / F16 | `L,2 {DATAID, L,p {L,6 {PRJOBID, MF, material, recipe, PRPROCESSSTART, PRPAUSEEVENT}}}`; reply `L,2 {L,m {PRJOBID created}, L,2 {ACKA, errors}}` | per-job; ERRTEXT names jobs not created |
| PRJobCommand | S16F5 / F6 | `L,4 {DATAID, PRJOBID, PRCMDNAME, L,n {L,2 {CPNAME, CPVAL}}}` | 12, 13, 14, 17 command not valid for state |
| PRJobDequeue | S16F17 / F18 | `L,m {PRJOBID}`; m=0 dequeues all | only QUEUED/POOLED jobs; reply lists removed jobs; ERRTEXT names the rest |
| PRGetAllJobs | S16F19 / F20 | header only; reply `L,m {L,2 {PRJOBID, PRSTATE}}` | all jobs not yet complete |
| PRGetSpace | S16F21 / F22 | header only; reply `<PRJOBSPACE>` (U2) | jobs that can still be created |
| PRJobSetRecipeVariable | S16F23 / F24 | `L,2 {PRJOBID, L,m {L,2 {RCPPARNM, RCPPARVAL}}}`; reply `L,2 {ACKA, errors}` | fail if any value is out of bounds; ERRTEXT names the failed variables; valid states not stated |
| PRJobSetStartMethod | S16F25 / F26 | `L,2 {L,m≥1 {PRJOBID}, PRPROCESSSTART}` | QUEUED/POOLED only; ACKA TRUE only if every listed job ends with that method |
| PRSetMtrlOrder | S16F29 / F30 | `<PRMTRLORDER>` 1 ARRIVAL, 2 OPTIMIZE, 3 LIST; reply `<ACKA>` | — |
| (multi-block inquire) | S16F1 / F2 | `L,2 {DATAID, DATALENGTH}`; reply GRANT | 0 granted, 1 busy, 2 no space, 3 duplicate DATAID |

- **ACKA** is Boolean: TRUE success, FALSE failure; the error list may be empty on success.
- **ERRCODE** values (E5): 11–17 as above; 16 not available for processing also exists in E5 but not in E40 Table 5.
- **W-bit:** E5 marks S16F19, F21, F23 and F25 as "S,H->E" without ",reply"; each has a defined reply, so treat as W=1 (inference, as for S2F49).
- **Recipe item:** RCPSPEC, or PPID in ASCII; the variable list may be `L,0` depending on PRRECIPEMETHOD (E5 S16F3 exception; inference: method 1).
- **Multi-create partial success:** ACKA for a partially created batch is not specified (ambiguous).

Alerts and events (E40 §8.2.1–§8.2.2, §10.4.6–§10.4.7, Table 19; E40.1 Table 1):
- **Style:** PRJobEvent is E30-style (S6F11/F12), E40-style (S16F9/F10) or E53-style (S6F11 or S6F13); the host uses one, chosen by a tool-specific mechanism such as an ECV (§8.2.2).
  PRJobAlert maps the same way: S6F11 for E30 style, S16F7/F8 for E40 style.
- **S16F7** `L,4 {TIMESTAMP, PRJOBID, PRJOBMILESTONE, L,2 {ACKA, L,n {L,2 {ERRCODE, ERRTEXT}}}}`, W-bit optional; S16F8 is header only.
  PRJOBMILESTONE: 1 Setup (#2), 2 Processing (#4, #5), 3 Processing Complete (#6), 4 Job Complete (#7, #18), 5 Waiting for Start (#3).
  Job Complete reports the outcome with ERRCODE 18 no material altered, 19 partial, 20 all processed, 21 recipe error, 22–24 failure kinds, 25 aborted, 26 stopped, 27 cancelled.
- **S16F9** `L,4 {PREVENTID, TIMESTAMP, PRJOBID, L,n {L,2 {VID, V}}}`, W-bit optional; S16F10 is header only.
  PREVENTID 1 Waiting for Material, 2 Job State Change.
- **Waiting for Material:** only in SETTING UP, once pre-conditioning is done and no material is on its way; carries PRJobID and Timestamp; suppressed when a control job owns the job (§8.2.2.3).
- **E30 style:** every Figure 4 transition maps to a CE with at least the DVVALs PRJobID, PRJobState and Timestamp (§5.1.3, §8.2.2.4).
  One CEID per transition or one shared state-change CEID is not settled by the text (§5.1.3–§5.1.4 vs Table 5 PREventID; ambiguous).
  Enabling follows E30 (S2F37), not E40 (§8.2.2.2).
  Equivalent milestone CEIDs are optional (§8.2.2.1).
- **Report variables:** every attribute is available as a DVVAL for job events (E40.1 §7); the most used are PRJobID, PRJobState, RecID, RecVariableList, PRMtlNameList (§11.2).

Normal scenario, E40-style notifications, one carrier job, automatic start (E40 Figure 2, Tables 9 and 19):
1. H->E S16F15 `L,2 {DATAID, L,1 {L,6 {PRJOBID, MF=13, L,1 {L,2 {CARRIERID, L,j {SLOTID}}}, L,3 {PRRECIPEMETHOD=1, RCPSPEC, L,0}, PRPROCESSSTART=TRUE, L,0}}}` / E->H S16F16 `L,2 {L,1 {PRJOBID}, L,2 {ACKA=TRUE, L,0}}`.
2. #1 → QUEUED/POOLED: E->H S16F9 PREVENTID=2, VID/V pairs carrying PRJobState=0 (§8.2.2.4; E40.1 Table 3) / H->E S16F10.
3. #2 → SETTING UP: E->H S16F7 milestone 1 / S16F8; E->H S16F9 PREVENTID=2 / S16F10.
4. Optional, while the carrier is not yet on its way: E->H S16F9 PREVENTID=1 / S16F10.
5. Carrier arrives (carrier/substrate services, outside E40); #4 → PROCESSING: S16F7 milestone 2, then S16F9 PREVENTID=2.
6. #6 → PROCESS COMPLETE: S16F7 milestone 3, then S16F9 PREVENTID=2.
7. Carrier removed; #7 → extinction: S16F7 milestone 4 with ACKA=TRUE and an empty error list or ERRCODE 20 (inference); PRJOBID is no longer valid.
In E30 style, each S16F9 above becomes S6F11 with the matching CEID and each S16F7 becomes S6F11 if milestone CEIDs exist.

Per-tool delta for process jobs:
- Queue depth (PRGetSpace value), activation order, and whether pooling is supported.
- Commands beyond ABORT; spelling of the start command (`START` or `STARTPROCESS`); accepted CPNAMEs.
- Numeric PRSTATE codes for STOPPED and ABORTED, if reported at all.
- Notification style (E30, E40 or E53), how it is selected, the CEIDs (per transition or shared) and linked DVVALs, and the W-bit on S16F7/F9.
- Create services supported (S16F3, F11, F15) and optional services (F17, F23, F25, F29).
- Material form: MF 13 (carrier + slots) or 14 (substrate MIDs); groups, concurrent and consecutive jobs; jobs without material.
- Recipe identifier form (RCPSPEC or PPID), PRRECIPEMETHOD 2 support, and tunable variables with their ranges.
- Manual start, PauseEvent, pre/post-conditioning, and the ERRCODE/ERRTEXT values actually returned.
- Interaction with E94 control jobs (Waiting for Material suppression, #7 extinction trigger).

### 6.4 Control jobs (E94, E94.1)

Sources: SEMI E94-0713 (CJM), E94.1-1101 (SECS-II mapping), E39.1-0703, E5-0813; cross-references to E40-0813 and E87-0312.

Caveats:
- E94.1-1101 is a provisional 2001 mapping and predates E94-0713; where they differ, both readings are given.
- E94 names no CEs and assigns no CEIDs; the event names in its R1-1 example (CJSELECTED, ControlJobStart, ControlJobCompleted) are non-normative.
- The S16F27 header reads "S,H->E" without ",reply" (E5); treated as W=1 because S16F28 is its reply (inference).

#### 6.4.1 Control Job State Model (E94 §9, Figure 2, Table 3; §7.4.5; §14.1)

Hierarchy: QUEUED; ACTIVE {SELECTED, WAITING FOR START, EXECUTING, PAUSED}; COMPLETED.

| # | From | Trigger | Guard | To | Action / messages |
|---|---|---|---|---|---|
| 1 | (no state) | create request from host (S14F9) or operator console | queue has space and is not locked (§10.2, §10.6) | QUEUED | append the CJ at the tail of the queue; **S14F10 OBJACK=0** (host create) |
| 2 | QUEUED | CJCancel, CJStop or CJAbort (host or operator) | — | (no state) | remove from queue and destroy; jobs behind it each move up one place |
| 3 | QUEUED | processing resource can take on another CJ | CJ is at the head of the queue | SELECTED | take the head job off the queue; its material may still be absent |
| 4 | SELECTED | CJDeselect (host or operator) | its material has not arrived; resources for the head-of-queue job are available | QUEUED | it takes the head position and the former head job becomes SELECTED |
| 5 | SELECTED | material for the first PJ has arrived, or that PJ needs none and its resource is free | StartMethod = TRUE (Auto) | EXECUTING | —; a carrier's PJs are not initiated before its ID and slot map are verified |
| 6 | SELECTED | as #5 | StartMethod = FALSE (UserStart) | WAITING FOR START | — |
| 7 | WAITING FOR START | CJStart (host or operator) | — | EXECUTING | —; same note as #5 |
| 8 | EXECUTING | CJPause, or an event listed in PauseEvent occurs | — | PAUSED | — |
| 9 | PAUSED | CJResume | — | EXECUTING | initiate PJs again |
| 10 | EXECUTING | every PJ in ProcessingCtrlSpec has finished | CJ itself not stopped or aborted; each PJ is POST ACTIVE or deleted (§7.4.5.3); all substrates are in destination carriers (§14.1) | COMPLETED | —; may cover post-processing |
| 11 | ACTIVE (any substate) | CJStop from host or operator, or raised internally | every PJ that was active at the CJStop has reached POST ACTIVE | COMPLETED | —; running PJs should be sent STOP (§12.3.9) |
| 12 | ACTIVE (any substate) | CJAbort from host or operator, or raised internally | as #11, for CJAbort | COMPLETED | —; running PJs are sent ABORT (§12.3.10) |
| 13 | COMPLETED | (a) its material has left the equipment, (b) it has no material, or (c) the host asks for deletion | for (a), reusable material such as fillers need not leave | (no state) | delete the CJ |

Rules the table does not show:
- **Chained transitions:** if on entry the conditions for the next transition already hold, the equipment takes it at once and still reports each step in order (§6.2.5).
- **SELECTED** reserves the processing resource for this CJ alone and initiates no PJ (§9.2.2).
- **EXECUTING:** PJs are initiated in ProcessOrderMgmt order as resources free up and their material is verified (§9.2.4).
  An initiated PJ sitting in WAITING FOR START or PAUSED keeps its resources and blocks later PJs that need them (§9.2.4 and its Note 1).
- **PAUSED:** no new PJ is initiated; PJs already processing continue (§9.2.5, §12.3.5).
- **A PJ stopped or aborted on its own does not stop the CJ:** the CJ still leaves via #10, and PRJobStatusList shows how each PJ ended (§7.4.5).
- **State codes** (E94.1 Table 6, U1): 0 QUEUED, 1 SELECTED, 2 WAITINGFORSTART, 3 EXECUTING, 4 PAUSED, 5 COMPLETED; ACTIVE has no code.

#### 6.4.2 Control job queue model (E94 §10, §14.1)

- **Order:** broadly FIFO; a new CJ joins at the tail (§10.1, §9.2.1).
- **Integrity:** one queue operation at a time; the queue is locked while one runs, and any other request is refused, e.g. a create during CJHOQ (§10.2).
- **CJHOQ:** the named CJ moves to the head; every job from the old head up to it shifts back one place; the rest keep their order; it locks the queue and is a no-op with one queued job (§10.3, §12.3.11).
- **Deadlock:** when the head job and the SELECTED job both await material, the host moves another job to the head with CJHOQ and then deselects the SELECTED job (§10.4).
- **Head job lacks resources:** CJDeselect is refused; the host may alternate CJDeselect and CJHOQ to find a selectable job, or CJStop the SELECTED job to force a hot job next (§10.5).
- **QueueAvailableSpace** (§10.6), reliable only while no queue operation runs; a create is refused at 0:
  +1 when cancel, stop or abort removes a queued CJ, and on entry to SELECTED other than by a deselect swap;
  −1 on an accepted create, and on a deselect when no other job is queued.
- **QueuedCJobs** (§10.7): CtrlJobIDs from the head; changes on create, on cancel/stop/abort of a queued job, and on any deselect.
- **Serial execution:** the next queued CJ shall start as soon as possible after the previous CJ's last PJ has begun processing; whether "start" means #3 is not stated (§14.1).

Source conflict: §10.6 rule 3 decrements the space on a deselect with an empty queue, but Table 5 makes CJDeselect fail (Busy) when the queue is empty, and §12.3.8 needs a head job to trade places with.

#### 6.4.3 ControlJob attributes (E94 Tables 1, 2, 6; formats E94.1 Table 6)

Create = use as an S14F9 AttrSetting: M mandatory, O optional, R ignored if sent (E94 Table 6).

| Attribute | Req | Access | Create | SECS-II form | Meaning |
|---|---|---|---|---|---|
| ObjID | Y | RO | M | OBJID, A 1–80 | host-assigned CtrlJobID |
| ObjType | Y | RO | R | A "ControlJob" | object type |
| CurrentPRJob | Y | RO | R | `L,n {PRJOBID}` | PJs now executing, paused, stopping or aborting |
| DataCollectionPlan | N | RW | O | A | host label echoed in reports; meaningless to the tool (§8.4.4) |
| CarrierInputSpec | Y | RW | M | `L,n {CARRIERID}` | input carriers; may be empty |
| MtrlOutSpec | Y | RW | M | `L,p {L,2 {SourceMap, DestinationMap}}`, each map `L,2 {CARRIERID, L,m {SLOTID}}` | source-to-destination slot mapping; empty selects slot integrity |
| MtrlOutByStatus | N | RW | O | `L,n {L,2 {MaterialStatus U1, DestinationMap}}`; m=0 lets the tool pick any slot | destination per material status |
| PauseEvent | N | RW | O | `L,n {CEID}` | events at which the CJ pauses |
| PRJobStatusList | Y | RO | R | not defined in E94.1 | each PJ with its PRJobState |
| ProcessingCtrlSpec | Y | RW | M | `L,p {L,3 {PRJOBID, L,m {L,2 {RuleName A, RuleValue any}}, L,n {L,2 {MaterialStatus U1, OutputRuleValue any}}}}`; m=0 / n=0 means none | PJs to run, each with ControlRule and OutputRule |
| ProcessOrderMgmt | Y | RW | M | U1: 1 ARRIVAL, 2 OPTIMIZE, 3 LIST | PJ initiation order |
| StartMethod | Y | RO | M | BOOLEAN: TRUE Auto, FALSE UserStart | auto start vs user start |
| State | Y | RO | R | U1 0–5 | current state |

Rules (§8.4, §14.3):
- **ProcessOrderMgmt:** LIST follows ProcessingCtrlSpec order; ARRIVAL follows material arrival, PJs without material first; OPTIMIZE is the tool's own throughput order; any extra value is supplier-documented (§8.4.8).
- **Slot Integrity Mode (mandatory):** empty MtrlOutSpec returns each substrate to its source slot in its source carrier; only sorter jobs such as randomization are exempt (§8.4.5.2).
- **Material Redirection Mode (optional):** a non-empty MtrlOutSpec (§8.4.5.3.1–.5):
  - SourceMap carriers must agree with any carriers named in the E40 material lists, and each source or destination location may appear only once;
  - an empty slot list means ascending order for a source and source order for a destination (Table 2);
  - on return, material goes to its mapped slot if free and reachable; otherwise set alarm SubstrateDestinationNotAccessible, cleared once a usable DestinationMap arrives or the operator intervenes.
- **No redirection support:** refuse any attempt to set MtrlOutSpec (§8.4.5.3.2) or change it once defined (§14.3.2).
  An empty MtrlOutSpec is still mandatory at create (Table 6), so the refusal applies to non-empty values (inference).
- **Incomplete MtrlOutSpec (optional; empty DestinationMaps):** SetAttr may fill destinations until COMPLETED, provided SourceMaps are unchanged and the affected material is neither placed in nor on its way to a destination carrier (§8.4.5.3.6, §14.3.3).
  If still empty when processing ends, set alarm SubstrateDestinationUnknown, keep accepting updates, and clear it once the material is placed (§8.4.5.3.7).
- **Modification:** other attributes change by SetAttr only outside EXECUTING and COMPLETED; refuse a wrong state or out-of-range value (§14.3.1); PAUSED edits are supplier-defined (§9.2.5).
- **OutputRule** assigns a MaterialStatus from results; with MtrlOutByStatus the material goes to that status's destination (§8.4.6); **ControlRule** is tool-specific (§8.4.3).

#### 6.4.4 Relation to process jobs (E40) and carriers (E87)

Process jobs:
- **Pooling:** under CJs, PJs wait in a pool; the CJ's order replaces E40 queue management (§7.2.2.1, §7.4.3).
- **Initiation** by the CJ corresponds to E40 Table 1 #2, QUEUED/POOLED → SETTING UP with PR Job Setup (inference).
- **Suppressed event:** a PJ owned by a CJ never sends PR Job Waiting for Material (E40 §8.2.2.3).
- **Completion:** CJ #10 waits on every PJ reaching POST ACTIVE (PROCESS COMPLETE, STOPPED or ABORTED) or being deleted (§7.4.5.3).
- **Termination:** CJStop sends STOP and CJAbort sends ABORT to running PJs (§12.3.9–.10).
  Action applies only to PJs still QUEUED/POOLED: SAVEJOBS keeps them, REMOVEJOBS destroys them (§12.2.1; E94.1 Table 3), via E40 Table 1 #18 (inference).
- **No event mapping:** E94 §14.5 (PJ paused/stopped/aborted vs CJ events) is reserved; the CJ–PJ relationship is supplier-documented (§7.4.4).

Carriers:
- **Verification (§11.2):** a CJ initiates only PJs whose carrier is verified; the level (ID read, slot map) depends on the tool; the section is reserved; reading it as E87 ID VERIFIED plus SLOT MAP VERIFIED is an inference.
- **Completion (§11.3):** stages not accessed → in access (substrate unloading has begun; carrier must stay) → completed (all material back, no active CJ using it, no queued CJ naming it; carrier may leave).
  Each stage change raises CarrierComplete or CarrierStopped; in redirection mode a carrier may complete before its CJ, once no substrate waits to move in or out of it.
- **E87 mismatch:** E87 CarrierAccessingStatus has four substates (NOT ACCESSED, IN ACCESS, CARRIER COMPLETE, CARRIER STOPPED; E87 §10.7.3.2); E94 names three stages.
- **Shared carriers:** behaviour when a carrier appears in several CJs is supplier-documented (§8.3.1).

#### 6.4.5 Services → SECS-II (E94 §12, E94.1 Tables 1, 3, 4; bodies from E5)

- **Create:** H->E S14F9 `L,3 {OBJSPEC, OBJTYPE="ControlJob", L,a {L,2 {ATTRID, ATTRDATA}}}` / E->H S14F10 `L,3 {OBJSPEC, L,b {L,2 {ATTRID, ATTRDATA}}, L,2 {OBJACK, L,p {L,2 {ERRCODE, ERRTEXT}}}}`.
  OBJACK: 0 success, 1 error; a zero-length OBJSPEC in S14F10 means nothing was created; in S14F9 OBJSPEC names the object owner, zero-length meaning none given (E5); E94 gives no value for it.
- **Commands:** H->E S16F27 `L,3 {CTLJOBID, CTLJOBCMD, L,2 {CPNAME, CPVAL}}` (`L,0` when no parameter) / E->H S16F28 `L,2 {ACKA, L,2 {ERRCODE, ERRTEXT}}` (`L,0` when no error).

| Service | SxFy | Key parameters | Accepted when | Rejection ERRCODE |
|---|---|---|---|---|
| Create | S14F9/F10 | the M attributes above | queue has space and is unlocked; every PJ exists; attributes valid; non-empty MtrlOutSpec only with redirection support (§8.4.5.3.2) | OBJACK=1; missing PRJobIDs go in ObjStatus (§12.3.3), which E39.1 Table 2 does not map; S14F10 has only OBJACK and the ERRCODE/ERRTEXT list for them |
| CJStart | S16F27 CTLJOBCMD=1 | — | WAITING FOR START (§12.3.4) | 17 |
| CJPause | CTLJOBCMD=2 | — | EXECUTING (Table 3 #8) | 17 |
| CJResume | CTLJOBCMD=3 | — | PAUSED | 17 |
| CJCancel | CTLJOBCMD=4 | Action | QUEUED only (§12.3.7) | 17; 27 Job cancelled |
| CJDeselect | CTLJOBCMD=5 | — | SELECTED, material absent, head job selectable (§12.3.8) | 17; 15 Busy (queue empty or head job lacks resources) |
| CJStop | CTLJOBCMD=6 | Action | ACTIVE, or QUEUED where it acts as CJCancel | 26 Job stopped |
| CJAbort | CTLJOBCMD=7 | Action | ACTIVE, or QUEUED where it acts as CJCancel | 25 Job aborted |
| CJHOQ | CTLJOBCMD=8 | — | target is QUEUED (implied by §10.3.1, Table 4); queue unlocked | 17 |

Notes:
- **Every S16F27 command** (Table 4 services; not Create) may also fail with 3 Unknown object instance, 12 Parameters improperly specified, or 13 Insufficient parameters specified (E94 Table 5).
- **ERRCODE numbers** come from matching E94's error names to E5's ERRCODE table; E94.1 Table 3 maps ErrorCode to ERRCODE without numbers.
- **Action** is CPVAL U1: 0 SAVEJOBS, 1 REMOVEJOBS (E94.1 Table 3); the CPNAME string is not specified.
  ACKcode maps to ACKA, whose range E94 gives as SUCCESS/FAILURE and E94.1 as TRUE/FALSE.
- **Gaps in E94 Table 5:** CJStop and CJAbort list no "Command not valid for current state" even though §12.3.9–.10 restrict their states; why "Job stopped/aborted/cancelled" would reject a command is not explained.
- **Other access:** attributes are read and modified through E39 services (§8.4.1, §14.3), i.e. S14F1/F2 and S14F3/F4 (E39.1 Table 1).
  E94.1 maps no message for deletion by service (#13 c); S14F11/F12 Delete Object is an inference.

#### 6.4.6 Events and variables (E94 §6.2.4, §7.3.2, §13; E94.1 Tables 2, 5)

- **Events:** every CJ transition #1–#13 is a CE (§6.2.4), reported as E30-style S6F11/S6F12, or E53-style S6F11/S6F12 plus S6F13/S6F14 (E94.1 Table 2).
- **Event data:** CtrlJobID at minimum (§7.3.2.2.1).
  Each attribute has a variable of the same type; reports carry post-transition values, except extinction events (#2, #13), which carry the values from just before deletion (§13.1).
- **Other E94 events and alarms:** CarrierComplete / CarrierStopped (§11.3); SubstrateDestinationNotAccessible and SubstrateDestinationUnknown in redirection mode (§8.4.5.3).

| Variable | Class | Format | Meaning |
|---|---|---|---|
| CtrlJobID | DVVAL | OBJID | CJ in the current event |
| QueuedCJobs | SV | `L,n {CtrlJobID}` | queued CJs, head first |
| QueueAvailableSpace | SV | U1/U2/U4/U8 | free queue slots, never negative; 0 = full |
| SetUpName | ECV | A 1–80 | host-set label for the tool's current setup |

Source conflict on SetUpName: E94 Table 15 and §14.4 set it to "unknown" after local operation or any EC change, while E94.1 Table 5 encodes unknown as a zero-length string.
E94 §14.4 also spells it SetupName.

#### 6.4.7 Normal scenario: auto start, one carrier, one PJ (after E94 R1-1, non-normative)

1. H->E S16F11 PRJobCreateEnh / E->H S16F12 (or S16F15/F16): the PJ enters QUEUED/POOLED.
2. H->E S14F9 `{OBJSPEC, "ControlJob", L {ObjID, CarrierInputSpec=L,1 {CARRIERID}, MtrlOutSpec=L,0, ProcessingCtrlSpec=L,1 {L,3 {PRJOBID, L,0, L,0}}, ProcessOrderMgmt=3, StartMethod=TRUE}}` / E->H S14F10 OBJACK=0; E->H S6F11 CE #1.
3. With no CJ selected, #3 follows at once: E->H S6F11 CE #3 (SELECTED) / H->E S6F12.
4. The carrier arrives; E87 ID and slot-map verification follow (E87 subsection).
5. First PJ's material is verified and StartMethod=TRUE: E->H S6F11 CE #5 (EXECUTING).
6. The CJ initiates the PJ: E40 PR Job Setup, Processing and Processing Complete as S6F11s; the carrier goes in access.
7. All substrates are back in the carrier: E->H S6F11 CarrierComplete.
8. The PJ is POST ACTIVE and every substrate placed: E->H S6F11 CE #10 (COMPLETED); R1-1 shows this after CarrierComplete.
9. The material leaves: E40 PR Job Complete (E40 #7) and E->H S6F11 CE #13 as the CJ is deleted (R1-3, R1-4 show PJ deletion before CJ deletion); R1-1 lists PR Job Complete earlier, which does not fit E40 #7.

#### 6.4.8 Per-tool delta for control jobs

- **Queue:** depth (initial QueueAvailableSpace); parallel CJ execution and its extra services (§14.2).
- **Events and commands:** CEIDs and names per transition; E30 or E53 style; which CTLJOBCMDs exist; extra commands or parameters (§15.1); the CPNAME used for Action; user-defined ERRCODEs.
- **Order and output:** supported ProcessOrderMgmt values and extras (§8.4.8.1); redirection and incomplete MtrlOutSpec support (Table 16); MtrlOutByStatus, OutputRule, MaterialStatus values, ControlRule names.
- **Pause and carriers:** usable PauseEvent CEIDs (§8.4.7); PAUSED edits (§9.2.5); verification level (§11.2); a carrier shared by several CJs (§8.3.1).
- **CJ–PJ relation:** PJs per carrier or per substrate; PRJobMultiCreate use (§7.4.2, §7.4.4).
- **Create and delete:** expected OBJSPEC; how S14F10 reports missing PJs; deletion on departure vs host request; filler exception (#13).
- **Operator console:** which commands and user starts the operator can issue locally.

### 6.5 Substrate tracking (E90, E90.1)

Sources: SEMI E90-0312 (behaviour), E90.1-0703 (SECS-II mapping), E39.1-0703 and E5-0813 (Stream 14 bodies).
E90.1 is older than E90-0312 and lags it in places; each gap is flagged where it matters.

Caveats:
- E90 has no substrate transition #15.
  Table 1 and Figure 3 go from #14 to #16.
- SubstProcState values differ by source: E90 Table 2 lists eight, including LOST and SKIPPED.
  E90 Table 14 stops at REJECTED, and E90.1 Table 5 codes 0–6, ending at LOST.
  **SKIPPED has no numeric code anywhere**; a VE needs a per-tool value (inference).
- SubstType values differ: E90 Table 2 and E90.1 Table 5 use WAFER, FLAT PANEL, CD, MASK (0–3); E90 Table 14 says wafer, disc, flat panel, reticle.
- Name variants: attribute `SubstLocID` vs variable `SubstSubstLocID` (Table 14);
  SV `SubstrLocSubstrIDi` (E90 Table 17) vs `SubstLocSubstrIDi` (E90.1 Table 5).

#### 6.5.1 Substrate object state model (E90 §9.2–§9.4, Figure 3, Table 1)

Hierarchy: SUBSTRATE, with concurrent AND substates:
- SUBSTRATE TRANSPORT: {AT SOURCE, AT WORK, AT DESTINATION}
- SUBSTRATE PROCESSING: {NEEDS PROCESSING, IN PROCESS, PROCESSING COMPLETE: {PROCESSED, ABORTED, STOPPED, REJECTED, LOST, SKIPPED}}
- SUBSTRATE READING STATUS, only with a substrate ID reader: {NOT CONFIRMED, WAITING FOR HOST, CONFIRMED, CONFIRMATION FAILED}

| # | From | Trigger | Guard | To | Action / messages |
|---|---|---|---|---|---|
| 1 | (none) | substrate registered | — | AT SOURCE | object created; first SubstHistory record {location, TimeIn=now, TimeOut empty}; location = SubstLocID, or BatchLocID+"."+SubstPosInBatch if created in a batch location; CE |
| 2 | AT SOURCE | taken from source location into the equipment | — | AT WORK | CE |
| 3 | AT WORK | moved back to source location | optional | AT SOURCE | CE |
| 4 | AT WORK | moved between two equipment substrate or batch locations | not needed with a single equipment location | AT WORK | CE |
| 5 | AT WORK | arrives at its destination (a substrate or batch location) | — | AT DESTINATION | CE |
| 6 | AT DESTINATION | taken from destination back into the equipment | optional | AT WORK | CE |
| 7 | AT DESTINATION | leaves the equipment by normal transfer | — | (extinct) | object deleted; CE |
| 8 | AT DESTINATION | user says, or equipment detects, it is at source | optional | AT SOURCE | CE |
| 9 | any SUBSTRATE substate | removal detected or reported by the user (abnormal) | not after #7 | (extinct) | object deleted; CE |
| 10 | (none) | object created | — | NEEDS PROCESSING | CE |
| 11 | NEEDS PROCESSING | actual processing begins | trigger documented per tool | IN PROCESS | CE |
| 12 | IN PROCESS | processing ends (completed, aborted or stopped) | — | PROCESSING COMPLETE | one terminal substate chosen; CE |
| 13 | IN PROCESS | step done, more processing needed | optional; trigger documented per tool | NEEDS PROCESSING | CE |
| 14 | NEEDS PROCESSING | removed or missing, or skipped | — | PROCESSING COMPLETE | LOST or SKIPPED; CE |
| 16 | (none) | object created | reader present and enabled | NOT CONFIRMED | **not reported** |
| 17 | NOT CONFIRMED | read OK and AcquiredID equals the ContentMap SubstID | — | CONFIRMED | CE with SubstID, AcquiredID |
| 18 | NOT CONFIRMED | read failed after retries | — | WAITING FOR HOST | CE with SubstID; wait for ProceedWithSubstrate or CancelSubstrate |
| 19 | NOT CONFIRMED | read OK but AcquiredID differs from the instantiating ID | — | WAITING FOR HOST | CE with SubstID, AcquiredID; wait as #18 |
| 20 | WAITING FOR HOST | ProceedWithSubstrate received | — | CONFIRMED | CE with SubstID, AcquiredID if valid |
| 21 | WAITING FOR HOST | CancelSubstrate received | — | CONFIRMATION FAILED | CE; SubstProcState → SKIPPED (via #14, inference); substrate goes to its source or destination |

Rules (E90 §6.1, Table 1 comments, R1-1):
- **Every transition is a CE** when GEM events are used, unless the table says otherwise (#16); transitions impossible on a design are omitted from its model (R1-1.3).
- **Already-satisfied states:** if entering one state already meets the next transition's conditions, the equipment takes it at once.
- **Registration record:** Table 4 #1 also covers a substrate registered at a location, so it and Table 1 #1 describe the same first SubstHistory record, not two (inference).

#### 6.5.2 Substrate Location and Batch Location models (E90 §10–§11, Figures 4–5, Tables 4–7)

Both models: {UNOCCUPIED, OCCUPIED}.
The initial state is picked by a conditional connector with no numbered transition, so no CE (inference).

| # | Model | From | Trigger | To | Action / messages |
|---|---|---|---|---|---|
| 1 | SubstLoc | UNOCCUPIED | a substrate arrives (or is registered here) | OCCUPIED | append SubstHistory record {SubstLocID, TimeIn=now, TimeOut empty} to that substrate; CE |
| 2 | SubstLoc | OCCUPIED | the substrate leaves | UNOCCUPIED | fill TimeOut in that substrate's open record; CE |
| 1 | BatchLoc | UNOCCUPIED | the whole group has arrived | OCCUPIED | for each substrate, append {BatchLocID+"."+SubstPosInBatch, TimeIn, empty}; CE |
| 2 | BatchLoc | OCCUPIED | the whole group has left | UNOCCUPIED | fill TimeOut for each substrate; CE |

- **Equipment substrate locations and batch locations are persistent**; carrier substrate locations (slots) are created and deleted with the carrier (§10.1, §11.1).
- **DisableEvents** (Boolean, RW, required) on each SubstLoc and BatchLoc suppresses that object's CEs (Tables 5, 7).
- **Attributes:** SubstLoc has ObjID, ObjType="SubstLoc", SubstID, SubstLocState; BatchLoc has ObjID, ObjType="BatchLoc", BatchLocState, BatchSubstIDMap (positions in order; "" empty, "filler" for a filler).
- **Ambiguous:** Table 6 comments say no events are reported during "subsequent creation/destruction of a batch"; the scope of that suppression is not defined.

#### 6.5.3 Substrate object lifecycle and attributes (E90 §8.3, §9.1, §12.1, Tables 2–3, 8)

- **Created** when registered: with the carrier's content information (E87 slot map), or by host S14F9 Create for a substrate delivered without a carrier (§9.1.2–§9.1.3).
  The equipment creates the objects itself, host Create and Delete are optional (§12.1.1), and objects are deleted at #7 or #9.
- **Create must name a location:** SubstLocID, or BatchLocID with SubstPosInBatch, never both; an ObjType in the attribute list is ignored (§12.1.3).
- **Default SubstID** when the content map gives none: carrierID, ".", slot (§8.3).
  The two-digit slot appears only in the example; §5.1.8 and Table 2 do not specify the separator or padding.

| Attribute | Access / Rq | Format | Meaning |
|---|---|---|---|
| ObjID | RO / Y | text = SubstID | identifier; ObjType = "Substrate" |
| SubstLocID | RO / Y | text | current slot or equipment location; empty while in a batch |
| SubstState | RO / Y | AT SOURCE, AT WORK, AT DESTINATION | transport state |
| SubstProcState | RO / Y | see caveat | processing state |
| SubstSource / SubstDestination | RO / Y | text | location registered at / final location (empty destination = source); for carriers, carrierID + slot number |
| SubstHistory | RO / Y | list of {location, TimeIn, TimeOut} | visited locations; TimeOut empty until departure |
| LotID | RW / N | text | lot set by the user |
| SubstType, SubstUsage | RW / N | enum | material kind; PRODUCT, TEST, FILLER (+ supplier values) |
| MaterialStatus | RO / N | enum, tool-specific | quality criterion |
| AcquiredID, SubstIDStatus | RO / N | text; enum | ID read (empty before reading); reading status (reader only) |
| BatchLocID, SubstPosInBatch | RO / N | text | batch location and position (batch tools only) |

#### 6.5.4 Requirements a simulator must meet (E90 §8, §14, Table 18)

- **Fundamental:** substrate object and model, substrate location object and model, services (except host Create/Delete), variables, and CEs for all three models.
  Batch locations, UpdateSubstrateObject, the reading-status model, ProceedWithSubstrate/CancelSubstrate and §14 events are optional capabilities.
- **One model instance per object**, so each SubstLoc and BatchLoc emits its own CE per transition (§8.6–§8.7).
- **Discrete vs grouped:** unrelated substrate transitions get one CE each; a related group move (internal-buffer tools) gets a single CE carrying the *List variables (§8.4.1, §8.5).
- **Reader present:** provide ECV `SubstrateReaderEnabled` to turn reading and the reading-status model on or off (§8.3).
  Also CEs SubstrateIDReaderAvailable and SubstrateIDReaderUnavailable when the reader's availability changes (§14).

#### 6.5.5 Services → SECS-II (E90.1 Tables 1, 3, 4; E39.1 Table 1; E5 §10.18)

| Service | SxFy | Key parameters | Conditions |
|---|---|---|---|
| GetAttr | H->E S14F1 / S14F2 | `L,5 {OBJSPEC, OBJTYPE, L {OBJID}, L {filter}, L {ATTRID}}` / `L,2 {L {L,2 {OBJID, L {L,2 {ATTRID, ATTRDATA}}}}, L,2 {OBJACK, L {L,2 {ERRCODE, ERRTEXT}}}}` | required (§8.1, §13.1); OBJTYPE Substrate, SubstLoc or BatchLoc |
| SetAttr | H->E S14F3 / S14F4 | `L,4 {OBJSPEC, OBJTYPE, L {OBJID}, L {L,2 {ATTRID, ATTRDATA}}}` | only RW attributes (LotID, SubstType, SubstUsage, DisableEvents) |
| register substrate | H->E S14F9 / S14F10 | `L,3 {OBJSPEC, "Substrate", L {L,2 {ATTRID, ATTRDATA}}}` / `L,3 {OBJSPEC, L {attrs}, L,2 {OBJACK, errors}}` | optional; settable attributes per Table 8 |
| remove substrate | H->E S14F11 / S14F12 | `L,2 {OBJSPEC, L {attrs}}` / `L,2 {L {attrs}, L,2 {OBJACK, errors}}` | optional |
| UpdateSubstrateObject | H->E S14F19 / S14F20 | `L,5 {DATAID, OPID, OBJSPEC, SVCNAME, L {L,2 {SPNAME, SPVAL}}}` / `L,4 {SVCACK, LINKID, L {L,2 {SPNAME, SPVAL}}, L,2 {SVCACK, errors}}` | optional; needs SubstID, SubstType and SubstState or SubstProcState; fails if transfer is unfinished |
| ProceedWithSubstrate, CancelSubstrate | H->E S14F19 / S14F20 (E90 Tables 12–13 as ObjectAction; E39.1 Tables 1–2) | SubstID, SubstLocID in and out; OPID | reader only; **not mapped by E90.1-0703** |
| change substrate state | — | — | listed as "not yet defined" in E90.1 Table 1 |

- **Codes:** OBJACK 0 ok, 1 error (E5).
  SVCACK 0 done, 1 no such service, 2 cannot now, 3 bad parameter, 4 completion later via S14F21/F22, 5 not completed or prohibited.
  SVCACK 6 is "not used" in E90 Table 10 but "no such object" in E5-0813.
- **Formats:** SubstID and SubstLocID map to OBJID (1–80 characters, E39.1 §6); TimeIn and TimeOut use TIMESTAMP (E90.1 Table 4).

#### 6.5.6 Events and variables (E90 §13, Tables 14–17; E90.1 Tables 2, 5, 6)

- **Every transition reports** with S6F11/S6F12 under E30 events; under E53 events, S6F13/S6F14 is also allowed (E90.1 Table 2).
- **Substrate CEs** (Table 14): SubstID, SubstSubstLocID, SubstState, SubstProcState, SubstHistory, SubstSource, SubstDestination, SubstLotID, SubstType, SubstUsage, SubstMtrlStatus; plus AcquiredID and SubstIDStatus with a reader, SubstBatchLocID and SubstPosInBatch on batch tools.
  Grouped CEs carry the same set as ordered *List variables (SubstIDList, SubstLocIDList, …).
- **Location CEs:** SubstLocID, SubstLocState, SubstLocSubstID ("" when unoccupied) (Table 15); BatchLocID, BatchLocState, BatchSubstIDMap (Table 16).
- **SVs for persistent objects** (Table 17), indexed per location i: SubstLocIDi, SubstLocStatei, SubstrLocSubstrIDi, BatchLocIDi, BatchLocStatei, BatchSubstIDMapi.
- **E90.1 formats:** IDs A[1..80]; states U1 (UNOCCUPIED 0, OCCUPIED 1; AT SOURCE 0, AT WORK 1, AT DESTINATION 2); SubstHistory `L,n {L,3 {OBJID, TIMESTAMP, TIMESTAMP}}`; LotID ASCII.
- **Gap:** E90.1 Table 5 gives no variable format for AcquiredID, SubstIDStatus, SubstLotID, SubstDestination, SubstSubstLocID, SubstBatchLocID, SubstPosInBatch, SubstLocSubstID, or any *List variable.
  SubstID and SubstLocID map to OBJID only as service parameters and attributes (Tables 3, 6–7); a VE copies the tool's formats (inference).

#### 6.5.7 Normal scenario: slot → process module → slot (single-wafer tool, no reader)

Placeholder locations `slot`, `robot`, `pm`; each CE is E->H S6F11 / H->E S6F12, in an order E90 does not fix within one move (inference).

1. Carrier docked and slot map accepted (E87, outside E90); `slot` objects created through the unnumbered entry, no CE (inference).
2. Substrate registered: #1 → AT SOURCE, #10 → NEEDS PROCESSING; SubstLocID=`slot`; SubstHistory [{slot, t1, —}].
   Table 4 #1 allows SubstLoc `slot` #1 on registration, but whether the slot's conditional entry instead starts it OCCUPIED is not settled by E90 (ambiguous).
3. Robot picks: SubstLoc `slot` #2 (TimeOut=t2), SubstLoc `robot` #1, substrate #2 → AT WORK; SubstLocID=`robot`.
4. Robot places in chamber: SubstLoc `robot` #2, SubstLoc `pm` #1, substrate #4 (AT WORK → AT WORK); SubstLocID=`pm`.
5. Recipe starts acting on the wafer: #11 → IN PROCESS.
6. Recipe ends normally: #12 → PROCESSING COMPLETE / PROCESSED.
7. Robot picks: SubstLoc `pm` #2, SubstLoc `robot` #1, substrate #4; SubstLocID=`robot`.
8. Robot places in the slot: SubstLoc `robot` #2, SubstLoc `slot` #1, substrate #5 → AT DESTINATION; SubstLocID=`slot`; SubstHistory now has five records (slot, robot, pm, robot, slot).
9. Carrier leaves: substrate #7 (object deleted); `slot` objects deleted with the carrier (§10.1).

#### 6.5.8 Per-tool delta for substrates

- **Location map:** SubstLocIDs of equipment locations (whether robot blades, aligners, load locks count) and the carrier-slot ID format, all supplier-documented (§7.3.1–§7.3.2, §10.1).
- **Batch locations:** whether present, their BatchLocIDs, position notation, and fixed-buffer vs internal-buffer (discrete vs grouped CEs).
- **Transitions implemented:** which optional ones (#3, #6, #8, #13) exist, the documented triggers of #11 and #13, and which terminal processing states occur.
- **CEIDs** per implemented transition and model (E90 assigns none), report links, and DisableEvents defaults.
- **ID reader:** presence, SubstrateReaderEnabled default, reader-availability CEIDs, and how Proceed/Cancel are carried.
- **Enumerations and formats:** SubstProcState codes including SKIPPED, SubstType, SubstUsage extras, MaterialStatus, and the formats E90.1 leaves open.
- **Optional services:** S14F9/F11 Create/Delete and UpdateSubstrateObject support; default SubstID rule actually used.

### 6.6 How the object machines synchronise

The standards couple their machines only through cross-references; none gives one end-to-end sequence.
The order below is assembled from 6.2–6.5 for a fixed-buffer, single-wafer tool with one carrier, one control job and one process job.
Steps without a direct source are marked (inference).

| Step | Carrier (E87) | Control job (E94) | Process job (E40) | Substrate (E90) |
|---|---|---|---|---|
| Host creates the PJ (S16F11 or S16F15) | — | — | #1 QUEUED/POOLED | — |
| Host creates the CJ naming the PJ and carrier (S14F9) | — | #1 QUEUED, then #3 SELECTED if the tool is free | stays pooled; the CJ now orders it (E94 §7.4.3) | — |
| Carrier arrives; ID and slot map verified (6.2.7) | Carrier #3, #8, #14, #15; Assoc #2 | waits: no PJ starts before its carrier is verified (E94 §11.2) | — | registered from the slot map: #1 AT SOURCE, #10 NEEDS PROCESSING (E90 §9.1.2) |
| First PJ's material verified | — | #5 EXECUTING (StartMethod Auto) or #6 then CJStart #7 | CJ initiates it: #2 SETTING UP (inference) | — |
| First substrate leaves the carrier | Carrier #18 IN ACCESS | — | #4 PROCESSING once material and resource are ready | #2 AT WORK; SubstLoc events per move |
| Each wafer is processed | — | — | — | #11 IN PROCESS, #12 PROCESSED |
| Last wafer back in its slot | Carrier #19 CARRIER COMPLETE | — | #6 PROCESS COMPLETE (inference: at the end of processing, before the last return) | #5 AT DESTINATION |
| Every PJ POST ACTIVE and every substrate placed | — | #10 COMPLETED | — | — |
| Carrier unloaded | LP #7, #8; Assoc #3; Carrier #21 destroyed | #13 deleted, its material gone | #7 extinct (material left) | #7 extinct |

Rules this coupling adds:
- **Verification gates work:** a control job initiates no process job whose carrier is unverified (E94 §9.2.4, §11.2);
  reading "verified" as ID VERIFICATION OK plus SLOT MAP VERIFICATION OK is an inference.
- **Pooling replaces queuing:** a process job owned by a control job never sends Waiting for Material (E40 §8.2.2.3), and the control job's ProcessOrderMgmt orders initiation.
- **Termination propagates downward:** CJStop sends STOP and CJAbort sends ABORT to running process jobs; a process job stopped or aborted on its own does not end its control job (E94 §7.4.5, §12.3.9–.10).
- **Extinction follows material:** process job #7, control job #13 and substrate #7 each wait for their material to leave, so all three end around the carrier's unload.
  Their relative order is not fixed by any standard (inference); a learned tool delta fixes it per tool.
