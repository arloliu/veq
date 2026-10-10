# VE evidence baseline

Status: draft (2026-10-10) — pilot evidence in; the requested inputs are due 2026-10-12

This document records, per evidence source, what the VE can learn from it, what it cannot, and at what fidelity.
It is phase 1 of the design re-derivation (see the [roadmap](../../ROADMAP.md));
phases 2 and 3 reopen design questions only against what is recorded here.
Provenance terms (`capture_method`, `vantage`, `fidelity`) are the tracepack fields of [ve-architecture.md](ve-architecture.md) §6.
Rationale and owner decisions are in [ve-decisions.md](ve-decisions.md).

Terminology: `H->E` host to equipment, `E->H` equipment to host.

---

## 1. Sources at a glance

| Source | State | Capture method, vantage | Record fidelity |
|---|---|---|---|
| EAP SML logs (§2) | pilot sample obtained; multi-day logs requested | `log`, `host` | `reconstructed` |
| Equipment gateway traffic (§3) | not available; a wire-level capture requested | `decoded-message`, `intermediary`; or `raw-stream` | `re-encoded`; or `wire-exact` |
| FDC service sample store (§4) | described by its maintainers; not obtained | not a tracepack source | named values only |
| Vendor GEM manual (§5) | obtained for the pilot tool type; change notes requested | — | cited assertions |
| Identifier exports (§6) | requested | — | catalog |
| EAP report and link definitions (§7) | requested | — | catalog |
| Gateway, EAP and FDC configuration (§8) | requested | — | configuration |
| SEMI standards (§9) | obtained (V-25) | — | prior |

## 2. EAP SML logs

The pilot sample is the EAP logs of two etch tools of one model: 69 minutes and 6 hours of one day.
How veq reads them is in [eapsml-dialect.md](../eapsml/eapsml-dialect.md);
`cmd/eapconv` converts them to tracepack `log` captures.

### 2.1 What the pilot sample shows

The next-event measurements come from a throwaway prototype, whose code is kept outside the main line.
Task: predict the next collection event (CEID) on a held-out window, given the true history,
with models frozen on the training window.
Interpolation weights are fixed, so log-likelihoods compare models within one task only.
On the 6-hour log, with 4 hours of training and 2 hours of test (3,019 test events):

| Model | top-1 | top-3 | mean log-likelihood (nats) |
|---|---|---|---|
| global unigram | 9.9 % | 30.8 % | −3.37 |
| global bigram | 51.5 % | 71.2 % | −1.93 |
| global trigram | 58.4 % | 76.5 % | −1.73 |
| independent modules (next module from the previous module) | 54.6 % | 74.0 % | −1.92 |
| synchronised modules (next module from the previous events) | 56.3 % | 77.1 % | −1.68 |
| within a module, module given | 77.3 % | 88.6 % | −0.92 |

An earlier comparison of 55 % global against 71 % per module set two different tasks against each other;
on one task, per-module models do not beat a global one.
The gaps between events show no drift from the training to the test window (two-sample Kolmogorov–Smirnov, p = 0.34).
Splitting by object (wafer, chamber) cannot be tested until the report definitions are known (§7).

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
  hypothesis H1 of the generation service tests the object-machine explanation.
- **These captures are too short and too narrow.**
  *Observed*: on the 6-hour log, accuracy levels off after 2–3 hours of training
  while the event vocabulary still grows at 4 hours;
  on the 69-minute log, 21 % of held-out events carry IDs never seen in training.
  Neither log holds a connection start or a rejected command.
  *Reading*: these captures miss behaviour the VE needs; how much log a tool type needs is measured per type, not assumed.
- **Tools of one model share most of their observed catalogs.**
  *Observed*: of the events both tools reported, 89 of 93 link to identical reports,
  and about 96 % of shared equipment constants have the same format.
  *Reading*: enough to motivate a shared tool-type layer;
  it says nothing about guards, timing, values or quirks, which hypothesis H2 of the generation service tests.
- **The tool's E157 events and variables do not appear.**
  The pilot tool type's manual documents its own E157 implementation,
  and the logs carry none of its events or variables (V-25).
- **Interval polling for FDC does not appear.**
  The logged `S1F3`/`S1F4` exchanges are the EAP's own queries, about 0.6–0.7 per minute.
  The FDC service's polling is the gateway's traffic (§3) and never reaches the EAP log.
- **Encrypted bodies are recipe bodies, by count.**
  The number of encrypted bodies equals the number of `S7F26` and `S7F108` messages exactly.
  This is an inference from counts; no body was decrypted.

### 2.2 What an EAP log cannot show

- Variable names in report values:
  the report definitions (`S2F33`/`S2F35`/`S2F37`) were sent before the sample, so values are positional only (§7).
- Transport: the log has no transport events (connect, Select, Linktest, T3, T7 or a socket close),
  so every converted record is in epoch 0 and marked `correlation-incomplete` (V-20).
- Wire time: timestamps are the EAP's log times at 0.1 ms resolution, not wire times.
- Wire bytes: records are rebuilt from SML, so fidelity is `reconstructed`;
  on one EAP version a reply is logged under its primary's header, so its session ID stays unknown
  ([eapsml-dialect.md](../eapsml/eapsml-dialect.md) §2).
- Traffic the gateway answers or originates itself, including the FDC service's polling (§3).
- Clear-text recipe bodies, which the EAP logs encrypted.

One log can carry several equipment connections; `cmd/logcat` and `cmd/eapconv` keep them apart (V-16, V-18).

## 3. Equipment gateway traffic

The lasting source is the traffic log service (V-12):
the gateway sends it the traffic it carries, and it packs that traffic as tracepack captures.
It does not run yet.

- **Supplies**: the traffic the EAP log cannot show —
  messages the gateway answers or originates toward the tool, including the FDC service's polling and the gateway's `S1F1` ping;
  gateway timestamps; transport events, if the recorder keeps them.
- **Cannot**: wire bytes of a frame it decoded successfully, unless it is a `raw-stream` recorder;
  polling bodies it thins on purpose (V-27), which a capture declares as a known unobserved source.
- **Known so far** (gateway and FDC maintainers):
  the gateway keeps System Bytes and rewrites only the session ID (V-8);
  it does not acknowledge event and alarm reports (V-8);
  it polls for FDC itself, only inside processing contexts, every second by default.
- **Requested**: a wire-level capture with transport events.

## 4. FDC service sample store

- **Supplies**: value trajectories of configured variables during processing contexts,
  a secondary source beside the gateway's thinned polling (V-27).
- **Cannot**: variable IDs, item types, failed polls or raw event reports;
  values outside processing contexts;
  per-sample times other than the gateway's.
  Values are kept only as numbers or text.
- **Retention**: 3 days in its primary store and 30 days in a best-effort offline copy,
  so it must be read within days of the traffic it describes.

## 5. Vendor GEM manual

- **Supplies**: the tool type's identifiers, state models, events, reports, remote commands and their parameters,
  and the manual's own implementation of the object standards, as cited assertions.
- **Cannot**: what the deployed software version does where it differs from the manual's version;
  timing, value trajectories and quirks.
- **Requested**: the tools' software versions and the manual's change notes between those versions.

## 6. Identifier exports

- **Supplies**: the tool's own catalog of status and data variables, equipment constants, alarms and collection events,
  with formats where the export carries them.
- **Cannot**: which identifiers the EAP uses, or behaviour.
- **Requested**: the exports or the EAP's copy of them,
  and why the EAP queries identifiers the tool answers as empty.

## 7. EAP report and link definitions

- **Supplies**: the variable behind each position of each report, and which reports each event carries.
  41 report IDs fire in the pilot sample, and none of their definitions appear in it.
- **Cannot**: a definition the EAP changed before the capture began, unless the export is dated.
- **Requested**: the EAP's `S2F33`, `S2F35` and `S2F37` (and `S5F3`, `S2F23` if used).

## 8. Gateway, EAP and FDC configuration

- **Supplies**: what the gateway answers or originates itself;
  the FDC configuration (polling interval, variable count), which sets V-27's sampling rule.
- **Cannot**: behaviour of the tool.
- **Requested**: the gateway's and EAP's configuration for the pilot tools, and the FDC event and data-collection configuration.

## 9. SEMI standards

- **Supplies**: the priors the skeleton was distilled from ([ve-gem-skeleton.md](ve-gem-skeleton.md)).
- **Cannot**: a tool's deviations, or a value a standard leaves undefined (V-22).
- E157 and older object-standard revisions are acquired only when needed (V-25).

## 10. Inputs outstanding

| Input | Source section | Requested from |
|---|---|---|
| Multi-day EAP logs with a connection start, full carrier cycles, alarms and rejected commands | §2 | EAP |
| Identifier exports, and why some queried identifiers are empty | §6 | tool UI or EAP |
| Report and link definitions | §7 | EAP |
| Gateway and EAP configuration | §8 | gateway owners, EAP |
| Wire-level capture with transport events | §3 | gateway owners |
| Tool software versions and manual change notes | §5 | tool owners, vendor |
| The other devices and streams 98 and 99 in the logs | §2 | EAP |
| FDC event and data-collection configuration | §8 | FDC maintainers |
| Recipe bodies decrypted, or confirmation they may stay opaque | §2 | EAP |
| Manual and logs of a second tool type | §2, §5 | tool owners, EAP |

Each input is recorded here as obtained or unavailable when its reply arrives.
