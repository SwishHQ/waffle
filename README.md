# waffle

**Write PDFs in React. Render them from Go. No Node.js, no browser, no sidecar.**

Documents are authored in **JSX/TSX exactly as [react-pdf](https://github.com/diegomura/react-pdf) users write them** — components, props, hooks, `StyleSheet`, `Font.register`. Everything after that happens **inside your Go process**: waffle transpiles the source with [esbuild](https://github.com/evanw/esbuild), runs real React 18 on the [goja](https://github.com/dop251/goja) JavaScript engine, lays the document out with its own flexbox engine, and writes the PDF with its own writer. Pure Go, `CGO_ENABLED=0`, one static binary.

```
JSX ──esbuild──▶ bundle ──goja/React──▶ element tree ──flexbox──▶ pages ──paint──▶ PDF
                 (all in-process: compile once, render in milliseconds)
```

## Highlights

- **React in, PDF out** — real React (hooks, context, `memo`, function components), not a lookalike DSL. Existing react-pdf documents largely work as-is.
- **Zero runtime dependencies** — no Node, no Chromium, no LibreOffice, no cgo. `go build` and ship one binary.
- **Fast** — a template compiles once (~4 ms) and renders a one-page report in ~6–7 ms; an 8-page letter with a full-page letterhead and four embedded fonts in ~24 ms. Renders scale across cores with plain goroutines. See [Benchmarks](#benchmarks).
- **Template asset caching** — images are decoded (and their pixels pre-compressed) once per `Template`, fonts fetched and parsed once, embedded font streams compressed once. Repeat renders skip all of it.
- **Real typography & layout** — flexbox (grow/shrink/wrap/percentages/min-max/aspect-ratio), the standard 14 PDF fonts with true AFM metrics, custom TTF embedding measured with real glyph advances, justification, inline styled runs, `letterSpacing`/`wordSpacing`, `maxLines` + ellipsis.
- **The react-pdf surface** — pagination (`wrap`, `break`, `fixed`, `minPresenceAhead`, orphans/widows, `{pageNumber}`/`{totalPages}` and function render-props evaluated per page on the live VM), images (PNG/JPEG from data URI, file path, or URL), SVG (paths, shapes, gradients, clipPath, text), `Canvas` painter, links, bookmarks/outline, AcroForm fields, `Note` annotations, opacity, CSS transforms, rounded/dashed/dotted borders, and encryption (RC4-128, AES-128, AES-256).
- **Validated output** — the test suite strict-validates generated PDFs with [pdfcpu](https://github.com/pdfcpu/pdfcpu).

## Quickstart

Author a document in React (`invoice.jsx`). `@waffle/react` is a virtual module the engine provides at transpile time — there is nothing to `npm install` and no `node_modules`:

```jsx
import { Document, Page, View, Text } from '@waffle/react';

export default function Invoice({ customer, total }) {
  return (
    <Document title="Invoice">
      <Page size="A4" style={{ padding: 40, fontFamily: 'Helvetica' }}>
        <Text style={{ fontSize: 24 }}>Invoice</Text>
        <Text style={{ fontSize: 12, color: '#666' }}>Billed to: {customer}</Text>
        <Text style={{ marginTop: 20 }}>Total due: ${total}</Text>
      </Page>
    </Document>
  );
}
```

Render it from Go, supplying data as props:

```go
package main

import (
	"context"
	_ "embed"
	"os"

	"github.com/swish/waffle"
)

//go:embed invoice.jsx
var invoiceJSX []byte

func main() {
	tmpl, err := waffle.LoadTemplate(invoiceJSX, waffle.TemplateOptions{Filename: "invoice.jsx"})
	if err != nil {
		panic(err)
	}
	out, _ := os.Create("invoice.pdf")
	defer out.Close()
	_, err = tmpl.Render(context.Background(), map[string]any{
		"customer": "Globex Corporation",
		"total":    2400,
	}, out)
	if err != nil {
		panic(err)
	}
}
```

Or from the CLI:

```sh
waffle invoice.jsx invoice.pdf props.json   # JSX + props → PDF
waffle tree.json   out.pdf                  # pre-serialized waffle-tree/v1 → PDF
```

### API at a glance

| Call | Use |
|---|---|
| `waffle.LoadTemplate(src, opts) → *Template` | Compile once (esbuild + goja). The Template holds the compiled program and the asset cache; safe for concurrent `Render`. |
| `Template.Render(ctx, props, w)` | Execute with props on a fresh VM and write a PDF. Function render-props stay live for per-page evaluation. |
| `Template.Tree(props)` | Return the intermediate `waffle-tree/v1` JSON instead of a PDF. |
| `waffle.RenderReact(ctx, src, props, w)` | One-shot compile + render. |
| `waffle.RenderTree(ctx, treeJSON, w)` | Render a pre-serialized element tree from any producer of the contract — no JavaScript runs at all. |

## Architecture

Two phases: **compile once, render many**. The full picture is in
[`docs/architecture.excalidraw`](docs/architecture.excalidraw) — open it at
[excalidraw.com](https://excalidraw.com) or with the VS Code Excalidraw
extension.

**1 · `LoadTemplate` — compile (once, ~4 ms).** esbuild transpiles and bundles
the JSX in memory, resolving `react`, `react/jsx-runtime`, and `@waffle/react`
from assets embedded in the binary (vendored `react.production.min.js` 18.3 and
waffle's runtime: a synchronous hooks dispatcher, `Font`/`StyleSheet` shims, and
the tree serializer). The bundle is compiled to a `*goja.Program`, reusable
across VMs.

**2 · `Render` — per call (milliseconds, concurrent-safe).** Each render runs
the program on a fresh goja VM with your props. React renders synchronously in
one pass — a document is a pure function of props: `useState` returns initial
values and effects never fire, which is the correct model for producing a file.
The VM emits a `waffle-tree/v1` JSON document (elements, styles, fonts, inline
assets); function-valued `render` props are registered as `$cb` callbacks and
evaluated back on the live VM per page during pagination.

The Go pipeline then takes over:

| Stage | Package | What happens |
|---|---|---|
| contract | `internal/contract` | parse + version-gate the tree JSON (`$cb` callbacks, `$inline` assets) |
| tree | `internal/tree` | node model, structural validation, prop normalization |
| styles | `internal/stylesheet` | units, colors, shorthands, media queries, inheritance |
| layout | `internal/layout` | flexbox engine (`internal/flexbox`), text measurement (`internal/fontstore`: embedded-TTF glyph advances or AFM core-14 metrics), image/SVG sizing |
| pagination | `internal/layout` | block + mid-element splits, forced breaks, `fixed` elements, orphans/widows, page-number substitution |
| paint | `internal/render` | boxes → PDF content streams: text runs, images, SVG, canvas replay, borders, gradient shadings, links, forms, outline |
| write | `internal/pdf` | object model, Flate streams, font embedding (FontFile2), image XObjects, encryption, deterministic xref/trailer |

**The asset cache.** A `Template` memoizes everything render-invariant: decoded
images (pixels already FlateDecode-compressed), fetched + parsed font stores,
and each embedded face's compressed `FontFile2` stream. The first render fills
the cache; every render after that skips decode, fetch, parse, and
recompression entirely — profiling showed those were ~90% of per-render cost on
image-heavy documents. Consequence: file/URL assets are read once per
`Template`; load a new Template to pick up changed files.

## Benchmarks

Measured with `go test -bench` on an Apple M4 Pro (12 cores), Go 1.24, warm
template, best of 3 runs. *Parallel* rows run on all cores via `b.RunParallel`
and report aggregate per-op time.

| Benchmark | Document | time/op | ≈ throughput |
|---|---|---|---|
| `LoadTemplate` | compile the report template | 4.0 ms | one-time |
| `RenderReport` | 1-page report: title + 2 styled tables (17 rows) + footer | 6.6 ms | ~150/s |
| `RenderReportParallel` | same, all cores | 3.0 ms | ~330/s |
| `RenderTreeReport` | same document from pre-serialized tree JSON (no JavaScript) | 1.9 ms | ~515/s |
| `AppointmentRender` | 8-page letter: full-page letterhead ×8, 4 embedded TTFs, inline runs, tables | 24 ms | ~41/s |
| `AppointmentRenderParallel` | same, all cores | 9.0 ms | ~110/s |

The `RenderTree` row isolates the Go pipeline (layout + paint + write); the gap
to `RenderReport` is the cost of executing real React on goja each render.

Under tight container caps (Docker, swap disabled, `GOMAXPROCS=1`), the 8-page
letter holds up on a fraction of a core:

| Cap | p50 | p95 | throughput | peak memory |
|---|---|---|---|---|
| 0.25 vCPU / 384 MiB | 102 ms | 322 ms | 8.3/s | 83 MiB |
| 0.5 vCPU / 512 MiB | 29 ms | 91 ms | 21.6/s | 91 MiB |

Reproduce:

```sh
GOTOOLCHAIN=local go test -bench . -benchmem -run '^$' .                        # engine benchmarks
GOTOOLCHAIN=local go test -bench . -benchmem -run '^$' ./examples/appointment/  # full-letter benchmarks
bash examples/appointment/bench/bench.sh                                        # capped-container harness
# CPU profile of the steady-state render loop:
cd examples/appointment && CPUPROFILE=/tmp/waffle.prof REQUESTS=30 go run ./bench
```

## Examples

| Example | Shows |
|---|---|
| [`examples/react`](examples/react) | data-driven invoice — the headline "JSX + Go data" flow |
| [`examples/appointment`](examples/appointment) | 8-page offer letter: letterhead, custom fonts, inline runs, prop-driven tables; `go test` checks + benchmarks |
| [`examples/hello`](examples/hello) · [`boxes`](examples/boxes) · [`text`](examples/text) · [`multipage`](examples/multipage) | writer, flexbox, text, and pagination basics |

## Development

```sh
export GOTOOLCHAIN=local   # deps are pinned to Go 1.24
go build ./...
go vet ./...
go test ./...              # PDF validation tests use pdfcpu if installed
go run ./examples/react    # writes invoice.pdf
```

PDF validation (optional but recommended):

```sh
go install github.com/pdfcpu/pdfcpu/cmd/pdfcpu@latest
export PATH="$PATH:$(go env GOPATH)/bin"
```

Regenerate golden files after an intentional output change: `WAFFLE_UPDATE=1 go test ./...`.

## Authors

- **Debarshi Chaudhuri** ([@Chaudhuri-Debarshi](https://github.com/Chaudhuri-Debarshi)) — creator & maintainer
- Built pair-programming with **Claude** (Anthropic); commits carry co-authorship trailers.

## License

[MIT](LICENSE) © 2026 Debarshi Chaudhuri.

Third-party components: the vendored `react.production.min.js` is MIT-licensed
(Meta — see [`internal/jsruntime/assets/VENDOR.md`](internal/jsruntime/assets/VENDOR.md));
the example fonts are the BSD-licensed Go fonts (Bigelow & Holmes — see
[`examples/appointment/assets/fonts/NOTICE.md`](examples/appointment/assets/fonts/NOTICE.md)).
