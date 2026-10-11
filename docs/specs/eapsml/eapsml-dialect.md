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
  veq therefore reads the W-bit and the session ID from the header only when the header names the same message as the body line,
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
with an `S1F1` heartbeat about once a minute and carrier-purge messages in stream 98,
which the tool's manual does not define.
Stream 99 carries lot and recipe exchanges with a third peer,
which answers no `S1F1` and reported one alarm naming an FDC monitoring strategy;
the owner labels it an FDC connection.
On one EAP version it has a connection of its own;
on the other its messages carry the tool's session ID, so no connection rule tells them apart from the tool's.
They stay in the tool's capture; the VE Profile declares them foreign traffic (VE decision V-21).
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

The converter `cmd/eapconv` writes tracepack `log` captures,
following the tracepack specification (go-secs `docs/specs/tracepack/`) at the version veq pins (V-14).
The multi-capture rules below are those of specification 2.29 (go-secs decisions G5-186 and G5-187),
and the output and identity rules those of 2.31 (G5-192 and G5-193, V-19);
tracepack v0.4.0, pinned since VE decision V-33, implements 2.32, which leaves these rules unchanged.
One run converts an ordered set of log files.

- **One capture per equipment connection** found in the logs (§3):
  transaction keys and epochs are scoped per capture, so two connections in one capture would mix their System Bytes.
  A message's connection is `deviceid=N` when its header line carries the DEVICEID token,
  else `session=N` from its own logged header (§2);
  a reply takes its primary's connection.
  The operator maps each connection to its tool ID, and a connection without one fails the run before anything is written.
  Each capture holds only its own connection's entries.
- **Lines of no connection** go into one more capture, written only if there are any:
  lines that hold no message, such as the EAP's own scenario and message-bus lines,
  messages the EAP built for its own use (§4),
  and messages whose connection is unknown, which the pilot sample does not contain.
  Its tool ID is the EAP's identity as configured in the converter, and it must differ from every equipment's tool ID.
  The converter checks that, since each tracepack writer sees only its own capture.
- **Pack metadata**: transport `hsms-ss`, since the logged headers are HSMS headers;
  capture method `log`, vantage `host`, time source `source-log` in the log's time zone;
  source dialect `eapsml/1 sml.Parse`, this dialect read by `internal/eapsml` with the lenient SML parser,
  and the converter with that parser as classifier.
  A source reference is the file's base name and the SHA-256 of its content, never a local path.
  Every pack of every capture of a run lists the run's source files as the same source references in the same order (go-secs G5-187),
  so a source index names the same file in every capture.
  The capture of lines of no connection has the same metadata.
- **Records**: a message that parses and builds becomes one `data` record, fidelity `reconstructed`, decode status `reconstructed-ok`,
  at the time of its header line.
  A body logged without its header is still a message, of unknown direction.
  Every other entry becomes one annotation carrying its source bytes and range, at the entry's time:
  an entry that holds no message is an `unrecognised-line`;
  each entry of an encrypted body, a header without a body, a body that does not parse or a message the EAP built for itself is an `unparsed-entry` with `parse-failed`.
- **The W-bit**: a W written in the body line is source text, so a W on a reply there fails the parse (`parse-failed`);
  a W taken only from the header is metadata, so the parse succeeds and the build rejects it (`build-rejected`).
- **Identities**: System Bytes, Session ID and W come only from a logged header that this dialect shows to be the message's own wire header (§2);
  otherwise their field-validity bits stay clear.
  The log has no connect or accept lines, so every record is in epoch 0, which the format marks `correlation-incomplete`.
  The TID has no field; it stays in the entry's source text.
- **Time**: an entry without a parseable timestamp takes the previous entry's time,
  or for the first entries of a run the first parseable time,
  and is marked `ordering-uncertain`;
  so is a local time that occurs twice or never in a daylight-saving change, which takes the earlier reading.
- **Nothing is dropped**:
  coverage is counted over the whole run:
  each input byte range is in exactly one record or annotation of exactly one capture,
  and no capture repeats another's bytes as annotations.
  The converter reports, per capture, its records per decode status and the source bytes they cover.
- **Output**: generation-0 segment packs, one per run of a capture's records in one UTC hour, under the store's staging keys (G5-193);
  seq runs across the whole capture,
  and the start and stop boundaries sit once per capture, at the times of its first and last records.
  During the pilot the output stays as these segments in a local directory; nothing merges it (VE decision V-24).
- **Reproducible output**: the writer start time is the last entry's unless the operator gives one,
  so the same files and settings always yield the same bytes, under the same IDs (G5-192).
  Capture and pack IDs are UUIDv7 values whose time is the capture's first record or the segment's hour,
  and whose other bits hash what they stand for:
  - a capture ID, everything its records depend on:
    the source references, which carry each file's content digest, in order;
    the tool ID and the connection it was mapped from, or none for the EAP's capture;
    the source dialect, the time zone, the recorder instance ID, which every pack of a capture shares,
    and the converter and classifier versions;
  - a pack ID, everything its bytes depend on:
    the capture ID, the hour and first seq that divide the capture into packs,
    the digest of the pack metadata, which holds the writer tag,
    the writer start time and options, and the version of the zstd implementation.

  The converter version is the module version, else the VCS revision of the build, marked when the tree was modified;
  a modified tree hashes as its base revision.
- **No overwrite**: a pack whose key already holds the same bytes is left in place and counted as already present;
  one whose key holds different bytes fails the run, and the existing file stays.
