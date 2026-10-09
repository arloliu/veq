# eapsml

How veq reads the SML text logs an EAP writes, and how they map to tracepack.
Code: `internal/eapsml` (reader and correlator) and `cmd/logcat`; the converter `cmd/eapconv` is planned.
Documents here follow `.agents/rules/450-doc-lifecycle.md`.

| File | Class | Status | Purpose |
|---|---|---|---|
| `eapsml-dialect.md` | living | current | entry shape, identities and irregular bodies of the first EAP log dialect, and its tracepack mapping |

Next proposal ID: P1.
