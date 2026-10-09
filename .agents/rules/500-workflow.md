# 500 — Workflow

## Before commit

1. `make lint` — fix all issues.
2. `make test` — must pass with the race detector.
3. Touched `docs/`?
   `make docs-check` must pass.
4. Touched a parser of external input?
   Run its fuzz target.
5. Search the diff for sensitive data (`100-overview.md`, prime directive 1) before staging.

`make ci` runs every gate.

## Commits

See `550-git-conventions.md` for branch, commit-message, and pull-request conventions.

## Review checklist

- [ ] Correctness
- [ ] No sensitive data: names, tool IDs, paths, vendor content, real traffic
- [ ] Tests use synthetic data and fail when the behaviour they guard changes
- [ ] Docs updated under `450-doc-lifecycle.md`; Status lines and subject READMEs in step
- [ ] New comments and prose use semantic linefeeds (`400-documentation.md`)

## Make targets

```bash
mise install        # first-time setup: Go + golangci-lint at the pinned versions
make lint           # golangci-lint run
make fmt            # golangci-lint fmt
make vet
make check          # lint + vet
make docs-check     # docs/ against 450-doc-lifecycle.md
make test           # -short, -race
make bench
make ci             # check + docs-check + test
```

Run `make` with no target for the help list.
