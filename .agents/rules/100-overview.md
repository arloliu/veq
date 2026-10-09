# 100 — Overview & Prime Directives

veq (`github.com/arloliu/veq`) builds Virtual Equipment (VE):
passive-side simulators of semiconductor tools, derived from recorded host–equipment traffic and vendor GEM manuals,
so that host software (EAP, FDC) can be developed and regression-tested without a production tool.
The design is in `docs/specs/ve/`; the roadmap is `docs/ROADMAP.md`.

## Layout

| Path | Scope |
|---|---|
| `cmd/logcat` | lists the GEM identifiers and data formats observable in EAP SML logs |
| `internal/eapsml` | reads an EAP's SML text log into messages and pairs primaries with replies |
| `docs/` | specs, plans and guides under `450-doc-lifecycle.md` |
| `scripts/` | repository checks (`docs-check.py`) |
| `data/` | sensitive inputs (vendor manuals, fab traffic logs) and their derived outputs; gitignored, owner-only |
| `tmp/` | local scratch; gitignored |

Create a package directory when its first code lands, not before.
Planned components (runtime, profile schema, evaluator, registry, harness, `cmd/eapconv`) are described in `docs/specs/ve/`,
not pre-created.

## Dependencies

- SECS-II, HSMS and SML come from `github.com/arloliu/go-secs`.
- Recorded traffic is stored as tracepack, the nested module `github.com/arloliu/go-secs/tracepack`,
  pinned to a pseudo-version of go-secs `main` until a tagged release has what veq needs.
- Runtime deps are intentionally minimal — check `go.mod` before adding any.
  Prefer stdlib.
- The module's minimum Go version is the `go` directive in `go.mod`; `mise.toml` pins the development toolchain.

## Prime Directives

1. **No sensitive data in the repository.**
   No company, site, tool-ID, internal-service or person names, no local paths,
   no quoted vendor-manual content, no real traffic.
   Real inputs live only in `data/`; tests and examples use synthetic data.
2. **Neutral terms.**
   The host-side automation program is the EAP;
   the gateway between tools and EAPs is the equipment gateway.
3. **Evidence over assumption.**
   A design claim about tool or EAP behaviour cites the evidence it rests on, or says it is an assumption.
4. **Small diffs.**
   Don't rewrite files unnecessarily.
