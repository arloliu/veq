# GEM behavioural skeleton

Status: current (2026-10-09) — reference distilled from SEMI E5, E30, E37 and E37.1

This document describes the GEM behavioural skeleton a passive Virtual Equipment (VE) must exhibit:
its state models, the standard scenarios, and the transport behaviour underneath them.

Sources: SEMI E30-0611 (GEM), E5-0813 (SECS-II), E37-0413 (HSMS), E37.1-0702 (HSMS-SS), E4-0699 (SECS-I).
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
| S3 materials status | mostly H->E requests with E->H data; a few E->H |
| S4 material control | mixed |
| S8 control program | both directions |
| S12 wafer map | E->H requests with reply; H->E data |
| S13 data sets | all both directions (used by large recipes) |
| S14 object services | all both directions (GEM uses S14F1/F2 GetAttr) |
| S15 recipes (E42) | mostly both directions |
| S16 jobs (E40/E94) | H->E commands; E->H alerts and notifications |
| S17 | H->E requests |
| S18 subsystems | H->E requests |
| S19 recipe and parameter (E139) | mostly both directions |

---

## 5. What is tool-specific (the delta)

### 5.1 Standard across every GEM tool (build once in the VE core)

- **State machines** 1.1, 1.2, 1.4, 1.5 and 1.6, plus the transport machines in §3.
  - Fundamental GEM (E30 Table 9): state models §3.0–3.3, processing states §3.4, host-initiated S1F13/14, event notification, online identification, error messages, operator-initiated control, documentation.
  - Everything else is an opt-in capability (E30 Table 10), per tool.
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
