# 300 — Testing

## Organization

- **Unit**: `*_test.go` beside the code (same package or `_test` suffix).
- **Benchmarks**: `*_bench_test.go`.
- **Fuzz**: `Fuzz*` targets for every parser of external input, usually in `fuzz_test.go`.

## Rules

- Test data is synthetic.
  Never copy real traffic, tool IDs, variable IDs or vendor values from `data/` into a test, fixture or golden file.
- No emojis in test output.
- `t.Context()` for contexts, `t.Setenv()` for env.
- `for b.Loop()` for benchmarks.
- Assertions: the standard `testing` package; no mocking framework.
- Cleanup via `t.Cleanup` or `defer`.
- `t.Parallel()` where safe.

## Async

- Never use `time.Sleep` to wait for state; wait on the event or channel that signals it.
- `time.Sleep` is acceptable only to *inject* a delay into the scenario itself.

## Patterns

Table-driven only when there are multiple cases; a single case is a plain function.

## Running

```bash
make test    # all packages, race, -short
make bench
```
