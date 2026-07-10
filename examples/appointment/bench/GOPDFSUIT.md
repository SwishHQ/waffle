# waffle vs gopdfsuit — same-machine benchmark

[gopdfsuit](https://github.com/chinmay-sawant/gopdfsuit) is a pure-Go PDF
service/library that renders its own JSON template format directly to PDF. Its
published [BENCHMARKS.md](https://github.com/chinmay-sawant/gopdfsuit/blob/master/guides/BENCHMARKS.md)
numbers (k6 HTTP suites + Go benchmarks) were recorded on different hardware
(i7-13700HX, WSL2), so quoting them next to waffle's would be meaningless.
Instead, gopdfsuit's **own Go benchmarks** were run **on this machine** (Apple
M4 Pro, 12 cores) right next to waffle's.

## Method

- gopdfsuit: `master` @ clone date, Go 1.26.4 (their pin), running their
  `BenchmarkGenerateTemplatePDF_FinancialReport{,_Parallel}` — the in-process
  Gin handler for `POST /api/v1/generate/template-pdf` rendering their
  `financial_report.json` sample (1 A4 page: title, 2 tables / ~52 cells,
  2 images, footer). Best of 5 runs.
- waffle: `BenchmarkRenderReport{,Parallel}` and `BenchmarkRenderTreeReport` —
  a comparable 1-page report (title, 2 styled tables / 17 rows, footer),
  data-driven via props. Best of 3 runs, warm template.

## Results (same machine, best run)

| Benchmark | time/op | allocs/op | ≈ throughput |
|---|---|---|---|
| gopdfsuit financial report (serial) | **0.31 ms** | 295 | ~3,200/s |
| gopdfsuit financial report (parallel) | **0.069 ms** | 292 | ~14,500/s |
| waffle report — full render (serial) | 6.6 ms | 61,507 | ~150/s |
| waffle report — full render (parallel) | 3.0 ms | 61,499 | ~330/s |
| waffle report — `RenderTree`, no JavaScript (serial) | 1.9 ms | 11,207 | ~515/s |

gopdfsuit is **~21× faster** than waffle's full render on this document class
(~6× against waffle's no-JS `RenderTree` path).

## Why — and why that's the expected shape

The two tools sit at different layers of abstraction:

- **gopdfsuit** parses a bespoke JSON template DSL and emits PDF primitives in
  what is essentially a single pass — ~300 allocations per document. There is
  no scripting engine, no CSS model, no general layout solver.
- **waffle** executes **real React on an interpreted JS VM** every render
  (~4.7 ms of the 6.6 ms), then resolves CSS-style stylesheets and runs a
  general flexbox + text-measurement pipeline (~1.9 ms). That cost buys the
  authoring model: JSX components, props/hooks/context, react-pdf
  compatibility, CSS semantics (shorthands, inheritance, media queries),
  measured custom fonts, justification, and pagination controls.

Both are far past the point where PDF generation is a bottleneck for typical
document workloads (payslips, letters, statements): waffle sustains hundreds of
renders/second per process, gopdfsuit thousands. Pick by authoring model —
React/JSX with CSS semantics (waffle) vs a purpose-built JSON template format
(gopdfsuit) — not by these microseconds.

Not reproduced here: gopdfsuit's k6 HTTP suites (they load-test the Gin server
loop at 48 VUs — dominated by HTTP mechanics rather than PDF generation; the
handler-level Go benchmark above already includes their full generate path)
and their Gotenberg comparison (waffle's own Gotenberg comparison lives in
[RESULTS.md](RESULTS.md)).

## Reproduce

```sh
# gopdfsuit (their benchmarks, their pinned toolchain)
git clone --depth 1 https://github.com/chinmay-sawant/gopdfsuit.git
cd gopdfsuit && GOTOOLCHAIN=auto go test -bench FinancialReport -run '^$' -benchmem -count=5 ./test/

# waffle (from the repo root)
GOTOOLCHAIN=local go test -bench 'RenderReport|RenderTree' -run '^$' -benchmem -count=3 .
```
