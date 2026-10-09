# EAP SML log dialect

Status: current (2026-10-09) — first dialect, read by `internal/eapsml`

An EAP (equipment automation program) logs the SECS messages it exchanges with a tool as SML text.
This document records what one such log dialect contains and how veq reads it.
Every statement here comes from the pilot sample (two etch tools of one model, about six hours of one day);
a claim not yet confirmed on a second EAP or a longer span says so.
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
- Timestamps have four fractional digits (0.1 ms) and no zone.
  They are the EAP's log times, not wire times, and the messages reach the EAP through the equipment gateway.
- The log holds no transport events: no connect, Select, Linktest, separate or T3 timeout.

## 2. Identities

- `TID` is the EAP's transaction id.
  A primary and its reply carry the same TID, so veq pairs them by direction, stream, function and TID.
- For equipment-initiated primaries the TID equals the System Bytes of the logged header;
  for host-initiated ones it does not.
- The logged header of a host reply can carry its primary's stream and function
  (an `S6F6` logged with a header reading `S6F5` and the W-bit set).
  veq therefore reads the W-bit from the header only when the header names the same message as the body line,
  and otherwise from the body line.
- In the sample, every primary carrying the W-bit has a logged reply.
  Event reports (`S6F11`) and alarm reports (`S5F1`) arrive without the W-bit, and no `S6F12` or `S5F2` is logged.
  Whether the tool sends them that way or the equipment gateway acknowledges them is not yet known.

## 3. Bodies that are not plain wire messages

| Case | What the log shows | How veq treats it |
|---|---|---|
| Reply printed by a second logger | a body line from a different logger, right after the reply's header | taken as that message's body |
| Message the EAP builds for its own use | a body line with no header line, from a logger other than the main body logger | not traffic; skipped by `internal/eapsml` today, to become an annotation in the converter |
| Encrypted body | `Encrypt SnFy=SxFy,TID=n,Message=<hex>` instead of SML | message kept with `FlagEncrypted` and no item |
| Malformed body | e.g. a heartbeat `S1F1` with two root items, which SECS-II does not allow | message kept with `FlagParseFailed` and the raw text |

## 4. Mapping to tracepack

The converter (`cmd/eapconv`, not yet written) stores the log as a tracepack `log` capture with `reconstructed` fidelity.
It follows the tracepack specification (go-secs `docs/specs/tracepack/`):

- System Bytes, Session ID and W come only from a logged header that the dialect shows to be this message's wire header;
  otherwise their field-validity bits stay clear and the record is marked `correlation-incomplete`.
- The TID has no field in the format; it stays in the entry's source text.
- Nothing is dropped:
  non-wire bodies, encrypted bodies and unparsable bodies become annotations covering their source lines.
