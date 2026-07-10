# waffle vs Gotenberg — final comparison

Two ways to produce the same business document (a multi-page offer/appointment
letter with a full-page letterhead, embedded fonts, and a compensation table):

- **waffle** — this library. The document is a React/JSX template compiled into
  the Go app; each render executes it in-process (goja + esbuild, pure Go,
  `CGO_ENABLED=0`) and writes the PDF directly. No sidecar, no subprocess.
- **Gotenberg** (`gotenberg/gotenberg:8`) — a container exposing LibreOffice
  over HTTP. The app renders a DOCX from a template and POSTs it to
  `/forms/libreoffice/convert`, which converts it to PDF.

Both were measured **on the same machine** (Apple M-series, Docker Desktop
arm64), **under the same container caps** (swap disabled so memory limits are
real), **warm** (Gotenberg with `--libreoffice-restart-after=10`; waffle with
its template compiled once), 30 sequential requests each. The caps are the
profiles a Gotenberg sidecar is typically sized under — 0.25 vCPU is the
observed floor below which LibreOffice cold-starts trip Gotenberg's internal
timeout and return 503s.

## Results

### 0.25 vCPU / 384 MiB (the sidecar sizing floor)

| Metric | Gotenberg | waffle | waffle advantage |
|---|---|---|---|
| p50 latency | 2.38 s | **102 ms** | **23×** |
| p95 latency | 7.47 s | **322 ms** | **23×** |
| mean latency | 3.22 s | **120 ms** | **27×** |
| throughput (serial) | 0.30 conv/s | **8.3 render/s** | **28×** |
| peak container memory | 264 MiB | **83 MiB** | **3.2×** |
| failures | 0 / 30 | 0 / 30 | — |

### 0.5 vCPU / 512 MiB

| Metric | Gotenberg | waffle | waffle advantage |
|---|---|---|---|
| p50 latency | 0.89 s | **29 ms** | **31×** |
| p95 latency | 1.87 s | **91 ms** | **21×** |
| mean latency | 1.07 s | **46 ms** | **23×** |
| throughput (serial) | 0.92 conv/s | **21.6 render/s** | **23×** |
| peak container memory | 190 MiB | **91 MiB** | **2.1×** |

### Uncapped (native, 12 cores)

waffle: **p50 15 ms**, 66.7 renders/s on one goroutine, **147 renders/s at
concurrency 8**, ~11 MiB Go heap in use. Gotenberg has no equivalent mode: one
instance drives one LibreOffice process, so conversions serialize regardless of
cores — throughput ≈ 1/latency, and scaling means more replicas, each carrying
its own sidecar.

### One-time costs

| Cost | Gotenberg | waffle |
|---|---|---|
| startup | container boot + LibreOffice start (503s if CPU-starved) | `LoadTemplate`: 8 ms native, ~0.4 s at 0.25 vCPU |
| first request | LibreOffice cold conversion (~3× slower; the reason `--libreoffice-restart-after` exists) | first render fills the asset cache (~1 s at 0.25 vCPU, ~165 ms native), then steady-state |

## Operational surface

| | Gotenberg | waffle |
|---|---|---|
| deployment | separate sidecar container per app instance | in-process Go library |
| runtime dependencies | LibreOffice inside the container | none — pure Go, no cgo, static binary |
| concurrency model | one conversion at a time per instance | scales with goroutines across cores |
| resilience machinery | health checks, restart policy, `--libreoffice-restart-after` tuning, CPU-floor sizing to avoid 503s | none needed — a render is a function call |
| input | **any** DOCX/Office file, from anywhere | a React/JSX template compiled into the app |
| output (this document) | 228 KB PDF | 281 KB PDF |

## The honest trade-off

The performance gap is not the interesting decision axis — the **input model**
is. waffle requires owning the document as a JSX template; it cannot convert an
arbitrary user-uploaded `.docx`. Gotenberg converts whatever Office file it is
handed, which is sometimes exactly the requirement.

- **You own the template** (offer letters, payslips, invoices, statements,
  certificates): waffle is ~20–30× faster, ~2–3× lighter, removes an entire
  sidecar from the deployment, and turns "PDF generation capacity" from an
  infrastructure-sizing problem into an ordinary function call.
- **You must accept arbitrary uploaded Office documents**: that is Gotenberg's
  job, and waffle does not compete for it.

The documents are the same class but not byte-identical: waffle renders the
8-page letter with the letterhead composited on every page and four embedded
TTF faces; Gotenberg converts a 167 KB single-letterhead DOCX. waffle is doing
at least as much layout work per document.

## How waffle got here

The initial in-process implementation was already ~2× faster than Gotenberg at
the same caps. Profiling the steady-state render loop then showed ~90% of
per-render CPU was redundant asset work — re-decoding the letterhead PNG
(~67%, including re-compressing its pixels) and re-flating embedded font
programs — while JS execution and layout were noise. waffle now caches these on
the `Template`: images are decoded and their pixels compressed once, fonts
fetched and parsed once, and each face's compressed `FontFile2` stream is built
once and shared across every render. That took the same-document native p50
from 165 ms to 15 ms (11×) and produced the numbers above. Consequence: file
and URL assets are read once per `Template`; a changed file on disk is picked
up by loading a new Template, not by re-rendering.

## Reproduce

```sh
# waffle under both caps (cross-compiles a static linux binary, runs in Docker)
REQUESTS=30 bash examples/appointment/bench/bench.sh

# steady-state CPU profile of the render loop
cd examples/appointment && CPUPROFILE=/tmp/waffle.prof REQUESTS=30 go run ./bench
go tool pprof -top /tmp/waffle.prof

# Gotenberg on the same machine/caps: run gotenberg/gotenberg:8 with
# --cpus/--memory/--memory-swap (swap disabled) and --libreoffice-restart-after=10,
# then POST a letter .docx to /forms/libreoffice/convert, timing each response.
```
