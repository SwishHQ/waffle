# feast

Write PDFs in React, render them from Go — no Node.js.

Documents are authored in **ReactJS** exactly as [react-pdf](https://github.com/diegomura/react-pdf) users write them today (JSX/TSX, components, props, styles). Everything after that — transpiling the JSX, running React, layout, text shaping, pagination, and PDF generation — happens **inside the Go process**. feast transpiles your source with [esbuild](https://github.com/evanw/esbuild) and runs real React on the [goja](https://github.com/dop251/goja) JavaScript engine, both pure Go. **goja is the only JS engine — there is no Node sidecar and no cgo.**

See [PLAN.md](PLAN.md) for the full architecture, the react-pdf parity target, and the phase-by-phase build plan.

## Quickstart

Author a document in React (`invoice.jsx`) — no React import needed. `@feast/react`
is a virtual module the engine provides at transpile time, so there is nothing to
`npm install` and no `node_modules`:

```jsx
import { Document, Page, View, Text } from '@feast/react';

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

	"github.com/swish/feast"
)

//go:embed invoice.jsx
var invoiceJSX []byte

func main() {
	tmpl, err := feast.LoadTemplate(invoiceJSX, feast.TemplateOptions{Filename: "invoice.jsx"})
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

`LoadTemplate` compiles once; `Render` runs the document on a fresh goja VM with the given props and writes a PDF. For a one-shot render use `feast.RenderReact`. If you already have a serialized element tree (from any producer of the contract), `feast.RenderTree` ingests `feast-tree/v1` JSON directly.

The CLI does the same:

```sh
feast invoice.jsx invoice.pdf props.json   # JSX + props → PDF (in-process, goja)
feast tree.json   out.pdf                   # a pre-serialized feast-tree/v1 → PDF
```

## Status

The engine is feature-complete for the core react-pdf surface, and React runs in-process:

- **PDF writer** (`internal/pdf`) — object model, deterministic xref/trailer, FlateDecode streams, content-stream operator builder, 14 standard Type1 fonts with Adobe Core AFM metrics, image XObjects with SMask.
- **Layout** — flexbox engine (grow/shrink/justify/align, percentages, absolute/fixed positioning, aspect-ratio, `flexWrap` + `alignContent`, min/max width/height), react-pdf-shaped stylesheet (units, colors, shorthands, media queries, inheritance).
- **Text** — measurement, greedy wrapping, text-align incl. justify, line-height; the 14 standard fonts **plus custom fonts** (`Font.register` a TTF as a data URI → embedded via `FontFile2`, measured with real glyph advances).
- **Pagination** — block + mid-element splitting, forced breaks, `minPresenceAhead`, orphans/widows, fixed headers/footers, `{pageNumber}`/`{totalPages}` templates.
- **Graphics** — JPEG/PNG decode from data URI, **http(s) URL, or file path**; image `objectFit` (fill/contain/cover/none/scale-down, centered + clipped); SVG (paths + shapes); Canvas replay.
- **Links** — block-level `<Link src>` renders a clickable URI annotation.
- **Rounded corners** — `borderRadius` (per-corner + `%`) on backgrounds, uniform borders, and image clipping (circular avatars via `borderRadius: '50%'`).
- **Opacity** — `opacity` via ExtGState, applied to a box and its subtree, with CSS-style nested multiplication.
- **Transforms** — `transform` (`rotate`/`scale`/`translate`/`skew`/`matrix`) about `transform-origin` (default center), applied to a box and its subtree.
- **Encryption** — `<Document userPassword ownerPassword permissions>` produces a password-protected PDF (standard security handler, RC4-128).
- **JS engine** (`internal/jsruntime`) — esbuild transpiles JSX/TSX and bundles embedded React + `@feast/react`; goja executes it to produce a `feast-tree/v1` document. Real React runs: components, props, `.map`, **hooks** (`useState`/`useMemo`/`useContext`/`useRef`/`useReducer`/…), **context** (`<Ctx.Provider>` + `useContext`), and `React.memo`/`forwardRef`.

  **Function render-props** work too: `<Text render={({ pageNumber, totalPages }) => \`${pageNumber} / ${totalPages}\`} />` is evaluated on the live VM per page during pagination (the closure stays in goja; the Go engine calls back with each page's context).

A document is rendered as a **pure function of props** in one synchronous pass: `useState` returns its initial value and effects don't fire (there's no screen to re-render), which is the correct model for generating a file. State setters and `useEffect` are inert by design.

Canvas `paint={fn}` works too: `<Canvas paint={(painter, w, h) => painter.rect(0,0,w,h).fill('#f00')} />` runs the react-pdf painter API on the VM and replays into the PDF.

Next: render-props that return styled element subtrees (text-returning ones work today), then forms (AcroForm), encryption, SVG gradients, and external font/image fetch. Tracked in [PLAN.md](PLAN.md).

## Develop

```sh
export GOTOOLCHAIN=local   # stay on Go 1.24
go build ./...
go vet ./...
go test ./...              # PDF validation tests use pdfcpu if installed
go run ./examples/react    # writes invoice.pdf from React + Go data
```

Some tests validate generated PDFs with [pdfcpu](https://github.com/pdfcpu/pdfcpu); they are skipped if it is not on `PATH`. To enable them:

```sh
go install github.com/pdfcpu/pdfcpu/cmd/pdfcpu@latest
export PATH="$PATH:$(go env GOPATH)/bin"
go test ./...
```

To regenerate golden files after an intentional change:

```sh
FEAST_UPDATE=1 go test ./...
```

## Layout

```
feast.go / render.go / template.go   public API: RenderTree, LoadTemplate/Template, RenderReact
internal/pdf/            the PDF writer core
  afm/                   Adobe Core-14 font metrics + WinAnsi encoding
internal/jsruntime/      THE JS engine: esbuild transpile/bundle → goja execute → feast-tree JSON
  assets/                vendored react.production.min.js + embedded @feast/react runtime
internal/contract/       feast-tree/v1 parse + version gating
internal/tree/           node model + structural validation
internal/stylesheet/     units, colors, shorthands, media queries, inheritance
internal/flexbox/        flexbox layout engine
internal/layout/         tree + styles → flexbox → positioned boxes → pagination
internal/render/         paint boxes to a PDF content stream (text, image, svg, canvas)
internal/imaging/ svgparse/ fontstore/   graphics + font support
examples/                runnable examples (examples/react is the headline)
testdata/                golden files and fixtures
```
