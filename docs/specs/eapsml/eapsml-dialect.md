# EAP SML log dialect

Status: current (2026-10-10) — first dialect, read by `internal/eapsml`

An EAP (equipment automation program) logs the SECS messages it exchanges with a tool as SML text.
This document records what one such log dialect contains and how veq reads it.
Every statement here comes from the pilot sample (two etch tools of one model, about six hours of one day),
except where it names the equipment gateway's maintainers as its source;
a claim not yet confirmed on a second EAP or a longer span says so.
The two tools run different EAP versions, and where they differ this document says "one EAP version".
The examples are synthetic.

## 1. Entry shape

A message appears as a header line, followed by a body line and the indented SML item tree, ending with a line holding `.`:

```
2026-01-02 08:00:00.0001 INFO - [Sender.Send]: Send Msg: [S1F3] /Header=[00-01-81-03-00-00-00-00-00-10] TID=16 APPID=EAP
2026-01-02 08:00:00.0001 INFO - [Logger.PrintSECSMsgBody]: S1F3_QueryState: 'S1F3' W
    <L [1]
        <U4 [1] 1001 >
     >
.
```

- `Received SxFy /Header=[…] TID=n` is equipment to host; `Send Msg: [SxFy] /Header=[…] TID=n` is host to equipment.
- The body line is `<name>: 'SxFy'` with an optional ` W`;
  the name is the EAP's own label for the message and may be empty.
- One EAP version appends tokens after the TID, such as `DEVICEID=n` and `FORMAT=RAW`;
  the other ends the header line at the TID or at `APPID`.
- Timestamps have four fractional digits (0.1 ms) and no zone.
  They are the EAP's log times, not wire times, and the messages reach the EAP through the equipment gateway.
- The log holds no transport events: no connect, Select, Linktest, separate or T3 timeout.

## 2. Identities

- `TID` is the EAP's transaction id.
  A primary and its reply carry the same TID, so veq pairs them by direction, stream, function and TID.
- Whether the TID equals the logged System Bytes depends on the EAP version:
  on one tool it does for every message, on the other for none.
  The equipment gateway keeps System Bytes and rewrites only the session ID (gateway maintainers),
  so the difference comes from the EAP.
- One EAP version logs a host reply with its primary's header
  (an `S6F6` logged with a header reading `S6F5` and the W-bit set);
  the other logs the reply's own header.
  veq therefore reads the W-bit and the session ID from the header
  only when the header names the same message as the body line,
  and otherwise takes the W-bit from the body line and leaves the session ID unknown.
- `DEVICEID=n`, where logged, is the EAP's number for the equipment connection.
  It can differ from the header's session ID:
  on one tool, messages received on two connections both carry session ID 0 under different device numbers.
- In the sample, every primary carrying the W-bit has a logged reply.
  Event reports (`S6F11`) and alarm reports (`S5F1`) arrive without the W-bit, and no `S6F12` or `S5F2` is logged,
  because the tool is configured not to ask for them:
  the vendor's equipment constants turn the W-bit off for streams 5, 6 and 10, and one tool reports them off.
  The gateway does not acknowledge these reports itself (gateway maintainers).
- The gateway can answer some primaries itself or originate messages toward the tool,
  depending on its deployment configuration (gateway maintainers);
  such messages never reach the log.
  In one FDC mode they include the gateway's own data-collection traffic to the tool:
  S1F3 polling, queries after events, and an `S1F1` ping every minute by default.

## 3. Connections in one log

A log can carry more than one equipment connection.
Besides the tool, the sample's EAP polls an auxiliary load-port controller on its own session,
with an `S1F1` heartbeat about once a minute,
and exchanges vendor-specific streams (`S98`, `S99`) that the tool's manual does not define.
On one tool that controller's logged headers have the top bit of the session field set (`80-00`),
the SECS-I reverse bit, while the device token names the connection.

`cmd/logcat` keeps the connections apart:
a message belongs to its logged device number, else to its own header's session ID,
and a reply belongs to its primary's connection.

## 4. Bodies that are not plain wire messages

| Case | What the log shows | How veq treats it |
|---|---|---|
| Reply printed by a second logger | a body line from a different logger, right after the reply's header | taken as that message's body |
| Message the EAP builds for its own use | a body line with no header line, from a logger other than the main body logger | not traffic; skipped by `internal/eapsml` today, to become an annotation in the converter |
| Encrypted body | `Encrypt SnFy=SxFy,TID=n,Message=<hex>` instead of SML | message kept with `FlagEncrypted` and no item |
| Malformed body | e.g. the EAP's heartbeat `S1F1` to the load-port controller, with two root items, which SECS-II does not allow | message kept with `FlagParseFailed` and the raw text |

## 5. Mapping to tracepack

The converter (`cmd/eapconv`, not yet written) stores the log as a tracepack `log` capture with `reconstructed` fidelity.
It follows the tracepack specification (go-secs `docs/specs/tracepack/`):

- System Bytes, Session ID and W come only from a logged header that the dialect shows to be this message's wire header;
  otherwise their field-validity bits stay clear and the record is marked `correlation-incomplete`.
- The TID has no field in the format; it stays in the entry's source text.
- Nothing is dropped:
  non-wire bodies, encrypted bodies and unparsable bodies become annotations covering their source lines.
