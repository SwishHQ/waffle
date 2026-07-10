# waffle vs Gotenberg — PDF generation benchmark

waffle (in-process JSX→PDF) measured under the **same container resource caps** a
Gotenberg docx→PDF sidecar is typically sized under (0.25 / 0.5 vCPU, capped
memory, swap disabled). Both engines render the **same class of document** — a
multi-page offer/appointment letter with a full-page letterhead, embedded fonts,
and a compensation table — warm, 30 requests, concurrency 1, on the same machine
(Apple M-series, Docker Desktop arm64, 8 cores visible to the container).

- **waffle**: `examples/appointment/appointment.jsx` → PDF (8 pages, 598 KB), compiled once then rendered. Harness: `bench.sh`.
- **Gotenberg**: `gotenberg/gotenberg:8`, an offer-letter DOCX template (167 KB) → PDF (228 KB) via LibreOffice, `--libreoffice-restart-after=10`.

## 0.25 vCPU (the dev floor Gotenberg was sized to)

| Metric              | Gotenberg (0.25 / 384m) | waffle (0.25 / 384m) | waffle advantage |
|---------------------|-------------------------|---------------------|-----------------|
| p50 latency         | 2.38 s                  | **1.12 s**          | 2.1× faster     |
| p95 latency         | 7.47 s                  | **2.20 s**          | 3.4× faster     |
| mean latency        | 3.22 s                  | **1.28 s**          | 2.5× faster     |
| throughput (serial) | 0.30 conv/s             | **0.78 render/s**   | 2.6×            |
| peak memory         | 264 MiB                 | **140 MiB**         | 1.9× lighter    |
| failures            | 0 / 30                  | 0 / 30              | —               |

## 0.5 vCPU

| Metric              | Gotenberg (0.5 / 512m)  | waffle (0.5 / 512m)  | waffle advantage |
|---------------------|-------------------------|---------------------|-----------------|
| p50 latency         | 0.89 s                  | **0.60 s**          | 1.5× faster     |
| p95 latency         | 1.87 s                  | **0.80 s**          | 2.3× faster     |
| mean latency        | 1.07 s                  | **0.64 s**          | 1.7× faster     |
| throughput (serial) | 0.92 conv/s             | **1.57 render/s**   | 1.7×            |
| peak memory         | 190 MiB                 | **126 MiB**         | 1.5× lighter    |

## waffle beyond the capped serial comparison

- **One-time compile** (`LoadTemplate`, esbuild transpile+bundle): **~10 ms**, amortized across every subsequent render (mirrors keeping LibreOffice warm).
- **Uncapped host ceiling** (12 cores): p50 **259 ms**, 3.64 render/s serial.
- **Scales with cores**: concurrency 8 → **14 render/s** on the host. Gotenberg serializes on one LibreOffice process (≈1/latency); more throughput there means more app replicas, each carrying its own sidecar.
- **No sidecar**: waffle is a pure-Go library, in-process, `CGO_ENABLED=0`. It removes the entire Gotenberg sidecar apparatus — separate container, LibreOffice cold-start 503s, `--libreoffice-restart-after`, a self-healing container restart policy, health checks, and CPU-floor tuning (too little CPU makes LibreOffice cold-starts trip Gotenberg's internal timeout and 503).

## Honest caveats

- **Not byte-identical documents.** waffle renders an 8-page letter (letterhead on every page + embedded custom fonts + a table → 598 KB); the Gotenberg run converts the offer-letter template (→ 228 KB). Same *class* and same caps; waffle is doing at least as much layout work, and is still faster and lighter.
- **Different input model — the real trade-off.** waffle requires the document authored as React/JSX and compiled into the app; it is not a drop-in converter for arbitrary user-supplied `.docx`. Gotenberg/LibreOffice converts whatever DOCX you hand it. If you own the template (offer letters, payslips, statements), waffle wins decisively on latency, memory, and operational surface. If you must accept arbitrary uploaded Office files, that's Gotenberg's job.

## Reproduce

```sh
# waffle, under the two caps (cross-compiles a static linux binary, runs in Docker)
REQUESTS=30 bash examples/appointment/bench/bench.sh

# Gotenberg on the same machine/caps: start gotenberg/gotenberg:8 with
# --cpus/--memory/--memory-swap (swap disabled) and POST a letter .docx to
# /forms/libreoffice/convert, timing each response.
```

---

## Update: after template asset caching

The tables above were the baseline that motivated profiling. A CPU profile of
the steady-state render loop showed ~90% of per-render time was redundant asset
work — re-decoding the letterhead PNG (~67%, including re-compressing its
pixels) and re-flating font programs — while JS execution and layout were noise.
waffle now caches those on the `Template`: images are decoded and their pixels
compressed once, fonts fetched/parsed once, and each embedded face's compressed
`FontFile2` stream is built once and shared across every render.

Re-measured with the same harness (30 requests, concurrency 1, swap disabled;
the example document is the current de-branded letter — lighter letterhead and
Go fonts than the baseline runs above, so the like-for-like row is the native
before/after pair, measured on the identical document):

| Profile | p50 | p95 | throughput | peak memory |
|---|---|---|---|---|
| native, uncapped — before caching | 165 ms | 203 ms | 5.8/s | — |
| **native, uncapped — after caching** | **15 ms** | **16 ms** | **66.7/s** (147/s at concurrency 8) | heap 11 MiB |
| **0.25 vCPU / 384 MiB — after** | **102 ms** | **322 ms** | **8.3/s** | **83 MiB** |
| **0.5 vCPU / 512 MiB — after** | **29 ms** | **91 ms** | **21.6/s** | **91 MiB** |

Same-document native comparison: **11× faster** (p50 165 ms → 15 ms). Against
the Gotenberg rows above, at Gotenberg's own 0.25-vCPU sizing floor waffle now
renders at **~23× lower p50 latency** (102 ms vs 2.38 s), **~28× the
throughput** (8.3 vs 0.30 conversions/s), in **~3× less memory** (83 vs
264 MiB). Caveats: the first render of a Template still pays the full
decode/parse cost (the cache fills on first use), and file/URL assets are read
once per Template — a changed file on disk needs a new Template to be noticed.
