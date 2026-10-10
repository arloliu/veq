# Data intake: vendor manuals and EAP logs

Status: draft (2026-10-10) — manual page text and EAP log conversion

How to bring a vendor GEM manual or an EAP SML log into `data/` and convert it into the forms veq reads.
Written for an agent working in a deployment that holds real inputs.
The guide ends where interpretation begins:
turning manual pages into cited assertions is the Spec Compiler of `docs/specs/ve/ve-architecture.md`, whose design is still open,
and learning from converted logs belongs to the VE generation service.

---

## 1. Before you start

Read `data/README.md`: the rules for real inputs, the layout this guide writes into, and `data/LOCAL.md`, the deployment's local index.
Add a line to `data/LOCAL.md` for every input you bring in.

## 2. Vendor manual

Prerequisites: Python 3 and poppler-utils (`pdfinfo`, `pdftotext`, `pdftohtml`).

```sh
python3 scripts/manual-text.py data/<model>/manual/<manual>.pdf
```

It writes `text/` beside the PDF in under a second for a thousand-page manual:

- `pages/pNNNN.txt`: the layout text of one page, from `pdftotext -layout`.
  The PDF page number, not the printed page label, is the citation unit: cite "manual p. N".
- `toc.tsv`: one bookmark per line, tab-separated: level (0 is top), title, start page (empty when the bookmark has no target).
- `rotated_pages.txt`: the pages whose rotation is nonzero, space-separated.
  These are usually landscape tables, and their layout text is the least reliable:
  check a value read from one against the PDF page before relying on it.

Reading a manual:

1. Find the chapter in `toc.tsv` and read its page range; do not read the manual end to end.
2. Read pages by number; a table that crosses pages continues on the next file.
3. Record every fact with its page, so that a later reader can check it against the same file.
4. Where the manual and the standards disagree, the manual describes the tool and the standard is the prior
   (`docs/specs/ve/ve-architecture.md` §1, decision V-10);
   a value neither settles stays unresolved (decision V-22).

Note also the manual's document number, revision and the software version it covers in `data/LOCAL.md`;
a tool running another software version may differ from it.

## 3. EAP SML logs

Prerequisites: the Go toolchain pinned in `mise.toml`.
The log dialect, its identities and its irregular bodies are in `docs/specs/eapsml/eapsml-dialect.md`;
read it before trusting any count.

Place each log as received under `traces/eap-sml/<tool-id>/<yyyy-mm-dd>.log`, then:

1. **Catalog** what the log holds:

   ```sh
   go run ./cmd/logcat -tz <zone> -out data/<model>/derived/log-catalog <tool-id>=<log>...
   ```

   `-tz` is the IANA time zone the EAP logs in.
   `catalog.json` gives, per tool and connection, message counts and log-quality flags,
   and the observed events, reports, alarms and remote commands;
   `svid_formats.csv` and `ecid_formats.csv` give the item format of each status variable and equipment constant seen.
   Log-quality flags (parse failures, bodies without headers, encrypted bodies) show where the log departs from the dialect spec.
   Many of them, or a new kind, mean a new dialect or EAP version:
   record it in `eapsml-dialect.md` before converting.

2. **Build the converter** from a clean, committed tree, to a path outside the repository:

   ```sh
   go build -o "$TMPDIR/eapconv" ./cmd/eapconv
   ```

   Its commit and the go-secs version enter every capture ID, and through it every pack ID;
   the tracepack and codec versions enter every pack ID.
   Use `go build`, not `go run`: `go run` stamps no commit, so the version reads `devel`;
   a tree with uncommitted changes stamps it `-dirty`.

3. **List the connections**: run the converter with every flag but `-tool`.
   It reads the logs, writes nothing, and fails naming every connection it found with its message count,
   as `deviceid=N` when the log names the EAP's device number and `session=N` otherwise.

4. **Map the connections to tool IDs.**
   Tool IDs are operator configuration: take them from the owner, never from guesswork.
   One log can carry several connections, e.g. the main tool and an auxiliary controller (dialect §3).

5. **Convert** to tracepack:

   ```sh
   "$TMPDIR/eapconv" -tz <zone> -out data/<model>/derived/eapconv/<tool-id> \
     -instance <uuid> -eap <eap-id> -tool deviceid=<n>=<tool-id> -tool session=<n>=<controller-id> <log>...
   ```

   - `-instance` is this deployment's recorder instance, a UUID generated once and kept in `derived/eapconv/instance-id`.
   - `-eap` names the capture of lines that belong to no connection; it must differ from every tool ID.
   - List the files of one run in time order.

   A re-run over the same files with the same flags and build reports every segment "already present".
   A build from another commit derives new capture IDs,
   so its output would sit beside the old one as a second capture of the same traffic:
   move the old output aside before converting again.
   A pack whose key holds different bytes fails the run rather than being overwritten.

## 4. Other inputs

Identifier exports, report and link definitions, wire-level captures and configuration files get a section here once their format is known.
Until then, keep each as received under `data/<model>/` with a line in `data/LOCAL.md`,
and record what it supplies in `docs/specs/ve/ve-evidence.md`.
