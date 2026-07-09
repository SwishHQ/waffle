# feast — write PDFs in React, render them from Go

**Build plan v2 — for review before implementation.**
Documents are authored in **ReactJS exactly as react-pdf users write them today** (JSX/TSX, hooks, composition). Rendering happens in a **Go library**: layout, text, pagination, and PDF generation are pure Go. Target: feature parity with `@react-pdf/renderer` 4.5.1. This document is the implementation spec, written to be executed phase-by-phase by an implementing agent (Opus 4.8) with minimal additional research.

**Changes from plan v1:** the authoring surface is now React itself (an npm reconciler package + a versioned tree contract + an embedded JS runtime in Go), not a Go fluent API. The Go fluent API is demoted to a secondary/testing surface. A templ evaluation was added (§4) — requested, assessed, not used. Everything else (layout engine, text engine, PDF writer, pagination, testing) carries over from v1 unchanged in substance.

---

## 0. Architecture summary

react-pdf's pipeline:

```
React element tree ──react-reconciler──▶ node tree ──layout (Yoga+textkit)──▶ render ──▶ pdfkit ──▶ PDF
└────────────────── all JavaScript ──────────────────────────────────────────────────────────────┘
```

feast splits that pipeline at the natural seam — the node tree. React's only job in react-pdf is producing that tree; layout and painting are already React-free. feast makes the tree a **versioned serialized contract** and moves everything after it to Go:

```
  JS side (authoring)                        │  Go side (the library)
                                             │
  user's JSX/TSX  ──@feast/react────────────▶│──▶ tree ingest ──▶ layout ──▶ render ──▶ PDF writer ──▶ PDF
  (real React 18/19,  (custom reconciler     │      │                │
   hooks, context)     + serializer)         │      │           flexbox engine (vendored Yoga port)
                                             │      │           text engine (textkit port, HarfBuzz-port shaping)
        feast tree contract (JSON, v1) ──────┘      │           font/image/svg engines
                                                    │
                             closures (render props etc.) called back
                             across the boundary during pagination (§3.4)
```

**The JS engine is goja, and only goja** (decided 2026-07; no Node sidecar, no v8go). React runs *inside the Go process*, so feast is a single self-contained Go library. There are two ways to feed it a document, both producing/consuming the same contract:

1. **In-process (primary — "it's just a Go library"):** `LoadTemplate(source, opts)` transpiles a JSX/TSX React source with esbuild (pure Go) and compiles it; `tpl.Render(ctx, props, w)` runs real React on goja (pure-Go JS engine, MIT, no cgo) with Go-supplied props and renders the PDF. `RenderReact` is the one-shot form. React itself and the `@feast/react` runtime are embedded and resolved from strings — no Node.js, no filesystem, no node_modules at render time. Single binary. **Implemented in `internal/jsruntime`.**
2. **Static tree ingest:** a caller that prefers to serialize the element tree in a separate build step (e.g. via the `@feast/react` npm serializer) hands the JSON to Go (`feast.RenderTree`). This is not a second *engine* — it is the same contract, produced elsewhere; it is also the internal format the in-process path compiles to.

There is deliberately **no react-reconciler reimplementation in Go** — we use real React, in JS, on goja.

---

## 1. Goals and non-goals

### Goals
- **Author in React, as-is:** existing react-pdf knowledge, docs and most code transfer — same components, same props, same style objects; migration = change the import from `@react-pdf/renderer` to `@feast/react` and drop web-only components.
- **Consume as a Go library:** `go get`, pass data as props from Go, get PDF bytes. **No Node required, ever** — React runs in-process on goja.
- Everything react-pdf can express, with the **same layout semantics** (flexbox, default `flexDirection: column`, same pagination rules, same style property set, same units).
- Go core is pure Go, permissive licenses only (MIT/BSD/Apache/CC0), no cgo in the default build.
- Deterministic output (byte-identical PDFs for identical input + fixed timestamps) for golden-file testing.

### Non-goals (explicit, permanent)
- Browser-only components (`PDFViewer`, `PDFDownloadLink`, `BlobProvider`, `usePDF`) — DOM/React-web artifacts, not PDF features (§8). The npm package exports nothing for them; dev preview is handled by the CLI instead.
- Companion packages `@react-pdf/math`, `@react-pdf/mermaid` — out of scope.
- PDF reading/manipulation — write-only, like react-pdf.
- Re-render loops / state-driven regeneration: a render is one shot, like react-pdf's `renderToFile`. Hooks and state work while the tree is being built, then the document is done.

### Naming
Working name **feast**; Go module placeholder `github.com/swish/feast`; npm package placeholder `@feast/react`. Confirm at review.

---

## 2. Reference: what react-pdf 4.5.1 does (parity target)

This section is the checklist the implementation must satisfy. Compiled from react-pdf.org docs and the monorepo source (2026-07). **The `@feast/react` npm package must accept exactly these components and props; the Go engine must implement exactly these semantics.**

### 2.1 Components
| Component | Key props |
|---|---|
| `Document` | title, author, subject, keywords, creator (default "react-pdf"), producer, creationDate, modificationDate, pdfVersion (1.3–1.7ext3, default 1.3), language, pageMode, pageLayout, ownerPassword, userPassword, permissions, onRender |
| `Page` | size (47 named sizes / number / [w,h] / {width,height?}; height omitted ⇒ auto-grow), orientation, wrap (default true), style, debug, dpi (default 72, affects `px` only), id, bookmark |
| `View` | wrap, style, render, debug, fixed, break, minPresenceAhead, id, bookmark |
| `Text` | wrap, render, style, debug, fixed, break, minPresenceAhead, hyphenationCallback, orphans (2), widows (2), id, bookmark; nestable (Text/Link inside Text) |
| `Image` | src/source (URL, file path, data URI, bytes, {data,format}, callback returning any of these), style, debug, fixed, break, cache (true), srcSet, sizes, bookmark; unbreakable |
| `ImageBackground` | Image props + imageStyle; children render on top |
| `Link` | src/href (external URL or `#id` internal), wrap, style, debug, fixed, hitSlop, bookmark |
| `Note` | style, fixed, string children → text annotation |
| `Canvas` | style, paint(painter, w, h), debug, fixed, bookmark; unbreakable |
| SVG | Svg, G, Path, Rect, Circle, Ellipse, Line, Polyline, Polygon, Text, Tspan, Defs, ClipPath, LinearGradient, RadialGradient, Stop, Marker |
| Forms (v4.2+) | TextInput, Checkbox, Select, List, FieldSet — AcroForm widgets; common attrs name/required/noExport/readOnly/value/defaultValue |
| Web-only | PDFViewer, PDFDownloadLink, BlobProvider, usePDF — **not ported** (§8) |

### 2.2 Pagination semantics (must match exactly)
- `wrap` on Page (default true): overflow flows onto new "sub-pages" of the same Page element; `wrap={false}` clips.
- View/Text/Link breakable (split at overflow, text at line granularity); Image/Canvas unbreakable (moved whole); `wrap={false}` on any element makes it unbreakable.
- `break`: force page break before element (flag resets after honored once).
- `fixed`: element repeats on every sub-page (headers/footers).
- `minPresenceAhead: N`: if less than N pt of following content fits below the element, move it to the next page (keep-with-next).
- `orphans`/`widows` (Text, default 2/2): min lines kept at page bottom / carried to next page.
- Render props: `Text render={({pageNumber, totalPages, subPageNumber, subPageTotalPages}) => …}`, `View render={({pageNumber, subPageNumber}) => …}`. totalPages available on Text only; render functions execute twice (layout pass, then final pass once totalPages is known); documented as needing to be pure.

### 2.3 Style system
- **Flexbox:** alignContent, alignItems, alignSelf, flex, flexDirection (**default `column`**), flexWrap, flexFlow, flexGrow, flexShrink, flexBasis, justifyContent, gap/rowGap/columnGap (percent allowed).
- **Layout:** aspectRatio, top/right/bottom/left, display (flex|none), position (absolute|relative|static), overflow (`hidden` only), zIndex.
- **Dimensions:** width/height, min/max variants.
- **Color:** color, backgroundColor, opacity (hex/rgb/hsl/named strings).
- **Typography:** direction (ltr|rtl), fontSize, fontFamily (string **or fallback list**), fontStyle, fontWeight (keywords + 0–1000), letterSpacing, lineHeight (unitless × fontSize, or absolute), maxLines, textAlign (left|right|center|justify), textDecoration (underline/line-through/combos), textDecorationColor/Style, textIndent, textOverflow (ellipsis), textTransform (capitalize|lowercase|uppercase|upperfirst), verticalAlign (sub|super), wordSpacing.
- **Image:** objectFit, objectPosition(X/Y).
- **Margin/padding:** all sides + Horizontal/Vertical shorthands + CSS-string shorthand expansion ("10 20").
- **Borders:** shorthand ("1pt solid red"), per-side width/color/style (solid|dashed|dotted), borderRadius per-corner (percent allowed).
- **Transforms:** rotate, scale(X/Y), translate(X/Y), skew(X/Y), matrix; transformOrigin (keywords/percent/length).
- **SVG presentation:** fill, stroke, strokeWidth, strokeDasharray, fillOpacity, strokeOpacity, fillRule, strokeLinecap, strokeLinejoin, textAnchor, visibility, clipPath, dominantBaseline.
- **Units:** pt (default), in, mm, cm, px (×72/dpi), vw, vh, % (per-property basis), rem (×18 base).
- **Media queries:** `@media max-width/min-width/orientation` keys inside style objects, evaluated against page dimensions.
- **Inheritance:** color, fontFamily, fontSize, fontStyle, fontWeight, letterSpacing, opacity, textDecoration, textTransform, lineHeight, textAlign, visibility, wordSpacing inherit; Text additionally inherits backgroundColor; SVG subtrees skip this pass.
- `StyleSheet.create` is an identity function; `style` accepts object or array (later wins).

### 2.4 Fonts
- `Font.register({family, src, fontStyle, fontWeight, fonts[]})`; formats TTF + WOFF (WOFF2 unofficial); no variable fonts; nearest-weight resolution per browser rules; no synthetic bold/italic.
- 12 built-in standard fonts (Courier/Helvetica/Times × 4 faces; Symbol/ZapfDingbats not wired in react-pdf).
- `Font.registerHyphenationCallback(word => parts)`; default hyphenation: soft hyphens, else Knuth-Liang en-US patterns.
- `Font.registerEmojiSource({url|builder, format, withVariationSelectors})` — emoji replaced by CDN images.
- Automatic subsetting on embed.

### 2.5 Text engine (textkit)
Attributed strings → runs; engines: **Knuth-Plass linebreaker with best-fit greedy fallback**, justification (word/letter spacing), scriptItemizer, fontSubstitution (per-codepoint fallback across fontFamily list), textDecoration rect computation, wordHyphenation, bidi (partial upstream).

### 2.6 Everything else
- **Images:** JPEG (DCT passthrough), PNG (incl. alpha), SVG-as-image (vector); EXIF orientation; resolved-image cache (~30 entries); srcSet/sizes selection.
- **Links/anchors:** `id` on any element → `#id` internal destinations; hitSlop.
- **Bookmarks:** string or `{title, top, left, zoom, fit, expanded}`; nesting mirrors component tree.
- **Canvas painter methods:** dash, clip, save, restore, path, fill, stroke, font, fontSize, text, rect, roundedRect, circle, ellipse, polygon, moveTo, lineTo, bezierCurveTo, quadraticCurveTo, lineWidth, lineCap, lineJoin, miterLimit, opacity, fillColor, strokeColor, fillOpacity, strokeOpacity, scale, rotate, translate, linearGradient, radialGradient.
- **Encryption (v4.4+):** user/owner passwords, permissions object; cipher follows pdfVersion (1.3→RC4-40, 1.4/1.5→RC4-128, 1.6/1.7→AES-128, 1.7ext3→AES-256).
- **Output (Node):** renderToFile, renderToStream, renderToBuffer.
- **Debug mode:** per-element layout-box overlay.
- **Layout pipeline steps (mirror these):** resolveStyles → resolveInheritance → resolvePageSizes → resolvePagePaddings → resolveDimensions (flexbox + text/image measurement) → resolvePercentHeight → resolvePercentRadius → resolveTextLayout → resolveSvg → resolveAssets → resolveOrigins → resolveZIndex → resolveBookmarks → resolveLinkSubstitution → resolvePagination.

---

## 3. The React authoring layer, the tree contract, and the Go API

### 3.1 `@feast/react` — the npm package

A thin package (~2–3k LOC TypeScript) that is API-compatible with `@react-pdf/renderer`'s document components:

- **Exports:** `Document, Page, View, Text, Image, ImageBackground, Link, Note, Canvas, Svg, G, Path, Rect, Circle, Ellipse, Line, Polyline, Polygon, Tspan, Defs, ClipPath, LinearGradient, RadialGradient, Stop, Marker, TextInput, Checkbox, Select, List, FieldSet, Font, StyleSheet` — with TypeScript prop types copied to match react-pdf's (§2.1). No web components, no `renderToFile` (rendering is Go's job).
- **Reconciler:** the real `react-reconciler` in sync/legacy mode (same as react-pdf) with a host config whose instances are plain JS objects `{type, props, children}`. Hooks, context, `React.memo`, composition — all real React, all work.
- **Serializer:** walks the committed tree → feast tree contract JSON (§3.2). Function-valued props (render, paint, hyphenationCallback, src-callbacks) are registered in a **callback table** and serialized as `{"$cb": "cb_7"}` references (§3.4).
- **`Font` / `StyleSheet` shims:** `StyleSheet.create` = identity (as upstream). `Font.register`/`registerHyphenationCallback`/`registerEmojiSource` write into the serialized document header instead of a JS font store.
- **Entry points:** `serialize(element): FeastTree` (for static mode / tests) and `__feastMain(propsJSON): FeastTree` — the well-known export the Go runtime calls (§3.3).
- **CLI (`feast-react`):** wraps esbuild. `feast-react build src/Invoice.tsx -o invoice.bundle.js` produces a single self-contained IIFE/CJS bundle (React + reconciler + user code, target pinned to what goja supports, no code splitting, `--inline-assets` optionally base64-inlines local fonts/images). `feast-react dev` = watch mode + preview via the feast CLI (§3.6).

### 3.2 The feast tree contract (`feast-tree/v1`)

The versioned JSON boundary between JS and Go. Design rules: stay **as close to react-pdf's internal node shape as possible** (so the reconciler is a pass-through and react-pdf's docs describe our contract), and make Go do all interpretation (style parsing, unit resolution, asset fetching).

```jsonc
{
  "version": "feast-tree/v1",
  "document": {
    "props": { "title": "Invoice", "pdfVersion": "1.4", ... },
    "fonts": [ { "family": "Roboto", "fonts": [ {"src": "https://…/Roboto.ttf", "fontWeight": 700}, {"src": {"$inline": "base64…"}} ] } ],
    "emojiSource": { "url": "https://…/", "format": "png" },
    "hyphenationCallback": { "$cb": "cb_0" },          // optional
    "children": [
      { "type": "PAGE", "props": { "size": "A4", "style": [ {"flexDirection": "row"}, {"padding": "10mm"} ] },
        "children": [
          { "type": "VIEW", "props": { "style": {"flexGrow": 1} }, "children": [
            { "type": "TEXT", "props": {}, "children": [ { "type": "TEXT_INSTANCE", "value": "Hello" } ] },
            { "type": "TEXT", "props": { "fixed": true, "render": { "$cb": "cb_1" }, "style": {...} } }
          ]}
        ]}
    ]
  },
  "callbacks": ["cb_0", "cb_1"]                        // ids that require an evaluator (§3.4)
}
```

- Node `type` values = react-pdf's primitive strings (DOCUMENT, PAGE, VIEW, TEXT, TEXT_INSTANCE, IMAGE, IMAGE_BACKGROUND, LINK, NOTE, CANVAS, SVG, G, PATH, …, TEXT_INPUT, CHECKBOX, SELECT, LIST, FIELD_SET).
- Styles pass through untouched as react-pdf-shaped values: numbers, CSS-ish strings (`"10mm"`, `"1pt solid red"`, `"45deg"`), arrays, `@media` keys. **The Go stylesheet subsystem is the single parser for all of it** (this was the "CSS escape hatch" in plan v1; it is now the primary path).
- Binary assets: URLs and absolute paths pass through (Go fetches/reads them at `resolveAssets`; paths must be readable by the Go process — the CLI's `--inline-assets` converts local refs to `{"$inline": base64}` at build time to avoid path coupling).
- Forward compatibility: unknown props → structured warning list in `RenderInfo`, not errors. `version` gates parsing.
- The contract is a **public, documented, semver'd format** (JSON Schema shipped in-repo). Anything that can produce it is a valid front-end — this is what keeps execution modes and future front-ends interchangeable.

### 3.3 Go public API

```go
// Mode 2 — static tree (produced by `feast-react build --emit-tree`, tests, or any other producer)
func RenderTree(ctx context.Context, tree []byte, w io.Writer, opts ...Option) (*RenderInfo, error)

// Mode 1 — embedded JS runtime (primary): bundle from `feast-react build`, e.g. via go:embed
tpl, err := feast.LoadTemplate(bundleJS)          // parses + type-checks the bundle once, reusable
info, err := tpl.Render(ctx, props, w, opts...)   // props: any Go value, JSON-marshaled into the component's props

// Mode 3 — Node sidecar (same Template interface; opt-in)
tpl, err := feast.NewSidecarTemplate(ctx, feast.SidecarConfig{Command: "node", Bundle: "invoice.bundle.js"})

// Options: WithFontStore, WithHTTPClient, WithCreationDate (determinism), WithDebug, WithOnPage(...)
```

```go
// invoice.go — what using it looks like
//go:embed invoice.bundle.js
var invoiceTpl []byte

func handler(w http.ResponseWriter, r *http.Request) {
    tpl, _ := feast.LoadTemplate(invoiceTpl) // cache this
    tpl.Render(r.Context(), InvoiceProps{Customer: "ACME", Lines: lines}, w)
}
```

`RenderInfo` carries page count, warnings, and timing — the equivalent of `onRender` (which is otherwise a no-op prop, accepted for compatibility).

A **secondary Go-native builder API** (`feast.NewView().Style(...)` fluent constructors, full spec in plan v1 §3) is retained because the engine's internal tree model makes it nearly free; it is the natural way to write engine tests and serves Go-only consumers. It compiles to the same contract. It is not the headline API and ships without doc-site prominence.

### 3.4 Closures across the boundary (render props, paint, callbacks)

react-pdf has four function-valued props. The engine defines one interface and each execution mode supplies it:

```go
type CallbackEvaluator interface {
    // EvalRender resolves a render prop: returns a serialized subtree for the given ctx.
    EvalRender(cb CallbackID, ctx RenderCtx) (TreeNode, error)   // RenderCtx: PageNumber, TotalPages, SubPageNumber, SubPageTotalPages
    EvalHyphenation(cb CallbackID, word string) ([]string, error)
    EvalImageSrc(cb CallbackID) (ImageSource, error)
    EvalCanvasPaint(cb CallbackID, w, h float64) ([]PaintOp, error) // JS paint fn runs against a recording painter
}
```

| Callback | Embedded runtime / sidecar (VM modes) | Static tree mode |
|---|---|---|
| `render` props | Full fidelity: pagination calls `EvalRender` per sub-page (twice for Text, matching react-pdf's two-pass totalPages) | Template strings supported: `render="{pageNumber} / {totalPages}"` (accepted by the npm package as an extension and serialized as data). Arbitrary per-page subtrees **unavailable — flagged gap** |
| `Canvas paint` | JS closure runs against a **recording painter** (a JS object mirroring §2.6's method list that logs ops); ops replayed in Go | Recorded at serialization time — requires explicit width/height styles on the Canvas (serializer errors otherwise, since final layout size isn't known) |
| `hyphenationCallback` | Full fidelity via `EvalHyphenation` (results cached per word) | Unavailable → built-in hyphenation used; serializer warns |
| `Image src` callback | Evaluated via `EvalImageSrc` at resolveAssets | Evaluated at serialization time (it takes no page context, so this is lossless) |

VM-mode mechanics: goja VMs are not goroutine-safe → each `Template` owns a `sync.Pool` of VM instances; callback evaluation is dispatched onto the VM's goroutine; `ctx` cancellation via goja's `Interrupt`. Callback results are the same contract node shape, ingested through the same parser. Purity requirement documented exactly as react-pdf documents it (closures run multiple times).

### 3.5 Data flow

Props are the data channel: Go value → JSON → the root component's props. Everything data-driven (looping invoice lines, conditional sections) is ordinary React over props. Convention: the bundle's default export is the root component (a `<Document>`-returning function); the CLI wires it to `__feastMain`. TypeScript users share prop types with the Go side via generated types or hand-mirroring (out of scope to automate in v1; note in docs).

### 3.6 JS engines and the spike gate

- **The one and only engine: goja** (`github.com/dop251/goja`, MIT, pure Go). Supports ES5 fully + most modern JS. **Decision (2026-07): goja only — no v8go, no Node sidecar.** feast bundles the user source with esbuild (`github.com/evanw/esbuild/pkg/api`, pure Go) targeting ES2015, resolving `react`, `react/jsx-runtime` and `@feast/react` from embedded strings via an in-memory plugin, and runs the resulting IIFE on goja. React's production build loads on goja with no shims (verified: no `process`/DOM references). **Implemented in `internal/jsruntime`** (`Compile` → `*Program`; `Program.Render(propsJSON) → treeJSON`).
- **Status: the R1 spike passed, and hooks + context work.** Real React (automatic JSX runtime, function/class components, props, `.map`, **hooks** — `useState`/`useMemo`/`useContext`/`useRef`/`useReducer`/…, **context** providers/consumers, `React.memo`/`forwardRef`) executes on goja and renders to a pdfcpu-strict-valid PDF — see `internal/jsruntime/{jsruntime,hooks}_test.go` and `template_test.go`. Hooks run via a **synchronous single-pass dispatcher** (see §3.5a), not `react-reconciler`: a document is a pure function of props, so state setters and effects are intentionally inert. Remaining goja work: the callback bridge for function render props (§3.5) evaluated on the VM. A full `react-reconciler` host config (for stateful re-render/effects before capture) is a documented non-goal unless a real need appears.

### 3.7 Dev experience & migration

- Migration from react-pdf: change import to `@feast/react`; delete `PDFViewer`/`PDFDownloadLink`/`BlobProvider`/`usePDF` usage (rendering moves to Go); everything else — components, styles, `Font.register`, render props — is intended to work unchanged. Ship a migration guide with the divergence list (§8).
- Because authoring is literally React, the **parity test corpus uses identical TSX** rendered through both `@react-pdf/renderer` and feast, diffed (§7). This is also the honest answer to "does it really behave the same" — CI proves it per feature.
- Preview during development: `feast-react dev` (sidecar → feast CLI → PDF in a viewer with reload). Using real react-pdf's `PDFViewer` for preview also works since the source is compatible, with a documented caveat that the preview engine ≠ the production engine.

---

## 4. templ — evaluated as requested, not used

[templ](https://github.com/a-h/templ) (a-h/templ, MIT, active) compiles `.templ` files — a JSX-*looking* syntax — into Go functions. Verified against its docs: components implement `templ.Component` with `Render(ctx, io.Writer) error` and the output is **HTML text streamed to a writer**; templ explicitly requires **no JavaScript and involves no React and no JSX execution**. So it cannot fill any role in this architecture:

1. It can't be the authoring layer — the requirement is documents **written in ReactJS**; templ doesn't run React or JSX, it only borrows the syntax for Go HTML templating.
2. It can't be the bridge/ingest — it produces flat HTML strings, not a typed element tree; we'd have to invent an HTML-ish dialect and re-parse it, which is strictly worse than the JSON contract the reconciler emits directly.

What templ *is*, is precedent for a possible **future third front-end**: a templ-style compiler letting Go-only teams write JSX-like `.feast` files that compile to contract-producing Go code. The contract (§3.2) keeps that door open at zero cost. Not in scope for v1; the secondary Go builder API (§3.3) covers Go-native authoring until then.

---

## 5. Repository layout & dependencies

Monorepo: one Go module + one npm package.

```
feast/
├── go.mod
├── feast.go                 // public API: RenderTree, LoadTemplate, Template, options
├── builder/                 // secondary Go-native element builders (compile to the contract)
├── cmd/feast/               // CLI: `feast render tree.json -o out.pdf` (used by dev preview & CI corpus)
├── internal/
│   ├── contract/            // tree contract types, JSON (de)serialization, schema, versioning
│   ├── tree/                // internal node model + validation (element paths in errors)
│   ├── jsruntime/           // THE JS engine: esbuild transpile/bundle (embedded React + @feast/react) → goja execute → tree JSON
│   ├── stylesheet/          // parse react-pdf-shaped style values: units, shorthands, colors, media queries, inheritance
│   ├── flexbox/             // vendored Yoga port + gap/fixes backport
│   ├── textkit/             // attributed strings, KP linebreak, justify, decorate, itemize, substitute, bidi, hyphenate
│   ├── fontstore/           // registration, resolution, standard-14 AFMs, emoji source
│   ├── imaging/             // src resolution, format sniffing, EXIF, cache
│   ├── svgparse/            // path-d parser, shapes, transforms → vector IR
│   ├── layout/              // the step pipeline incl. pagination + CallbackEvaluator hooks
│   ├── render/              // paint laid-out tree: ops, gradients, clipping, debug overlays
│   └── pdf/                 // PDF writer (see §6.7)
│   // NB: the @feast/react runtime (components, single-pass renderer, Font/StyleSheet,
│   // serializer) lives embedded at internal/jsruntime/assets/feast-react.js — there is
│   // no separate npm package (removed 2026-07; feast is a pure Go library).
├── testdata/                // golden PDFs, fixture fonts/images, Yoga fixtures, contract fixtures, shared TSX corpus
└── examples/                // ported react-pdf examples as TSX + the Go programs that render them
```

### Dependencies (all verified permissive, 2026-07)

| Dependency | License | Used for |
|---|---|---|
| `github.com/dop251/goja` | MIT | **The** in-process JS engine (goja only — no other engines) |
| `github.com/evanw/esbuild` | MIT | Pure-Go JSX/TSX transpile + in-memory bundling of the React document |
| `github.com/go-text/typesetting` (pin exact version) | BSD-3 | Font parsing (TTF/OTF/CFF/TTC/variable), HarfBuzz-port shaping, glyph metrics/outlines, UAX#14/#29 segmenter |
| `github.com/tdewolff/font` | MIT | Font subsetting (TrueType + CFF, GID-based), WOFF/WOFF2 decoding |
| `golang.org/x/text` | BSD-3 | UAX#9 bidi reordering |
| `golang.org/x/image` | BSD-3 | Extra image codecs if needed later |
| `github.com/speedata/hyphenation` | CC0 | Knuth-Liang hyphenation; vendor hyph-en-us patterns (audit per added language) |
| vendored Yoga port (from kjk/flex BSD-3 snapshot; new code derived from current MIT facebook/yoga) | BSD-3/MIT | Flexbox engine (§6.2) |
| `github.com/pdfcpu/pdfcpu` | Apache-2.0 | **Test-only:** validate generated PDFs in CI |
| `react` (vendored, not a dependency) | MIT | `react.production.min.js` 18.3.1 is vendored + embedded for goja (see `internal/jsruntime/assets/VENDOR.md`); no npm package is published or required |

Explicitly rejected: unidoc/unipdf (commercial), seehuhn.de/go/pdf (GPL — do not read its code), oksvg (stale), gopdf (insufficient control), **templ (§4 — no React/JSX execution, HTML-string output)**, **v8go (cgo — goja is enough and keeps feast pure-Go/cgo-free)**, **Node sidecar (dropped 2026-07 — goja runs React in-process, so no second engine is needed)**.

**folio note (carried from v1):** `github.com/carlos7ags/folio` (Apache-2.0, active) already does flexbox→paginated-PDF in Go with its own writer. Its API is HTML/CSS-first, single-maintainer, pre-1.0, and not react-pdf-shaped. Decision stands: greenfield with folio as a design reference; 1-day code read budgeted in Phase 0; lifting patterns (not code wholesale) allowed.

---

## 6. Go engine subsystem specifications

(Carried from plan v1 with ingest-related adjustments; unchanged sections kept in full because this is the build spec.)

### 6.1 Contract ingest & tree model (`internal/contract`, `internal/tree`)
- Parse `feast-tree/v1` JSON → internal typed nodes; prop parsing per primitive (booleans, numbers, unions like Page `size`); style values kept raw for the stylesheet subsystem.
- `{"$cb": id}` values become `CallbackRef`s; documents listing callbacks **require** an evaluator — `RenderTree` with callbacks and no evaluator fails with a clear error naming the offending elements (except template-string render props, which are data).
- Structural validation with element paths (e.g. `Document > Page[0] > View[2] > Text: fontFamily "Roboto" not registered`): Text children are strings/Text/Link; Page only under Document; SVG children only under Svg; Note children string-only.
- `Clone()` deep-copy (pagination splits trees; callback results graft subtrees).
- Unknown props/types → warnings in `RenderInfo` (forward compat).

### 6.2 Flexbox engine (`internal/flexbox`)
- Start from the kjk/flex snapshot (Go port of Yoga ~2017, BSD-3; measure funcs, flexWrap, %, min/max, aspectRatio, absolute positioning).
- **Must add (ported fresh from current MIT facebook/yoga):** `gap`/`rowGap`/`columnGap` (incl. percent), `space-evenly` (justify + alignContent), `position: static`, post-2017 aspect-ratio fixes.
- API needed: node create/free, style setters for every §2.3 flex/dimension/position prop, `SetMeasureFunc`, `CalculateLayout(w, h, direction)`, layout getters per edge.
- Config: point scale factor 0 (match react-pdf's `setPointScaleFactor(0)`).
- Test with Yoga's generated fixture suite for the props we expose + new gap fixtures.

### 6.3 Style system (`internal/stylesheet`)
- Input = react-pdf-shaped JS values from the contract (numbers, strings, arrays, @media keys). Flatten arrays (later wins) → expand shorthands (margin/padding/border/flex/gap/transform strings) → resolve media queries against page box → convert units → typed resolved style.
- Units per react-pdf `units.ts`: pt default; in ×72; mm ×72/25.4; cm ×72/2.54; px ×72/dpi rounded; rem ×18; %/vw/vh resolved during layout (basis-dependent).
- Colors: hex 3/6/8, rgb()/rgba(), hsl()/hsla(), CSS named → RGB (match `color-string`).
- Inheritance exactly per §2.3; textDecoration values merge unless `none`; SVG subtrees excluded.
- fontWeight keywords: thin 100 … black 900 (react-pdf's table).

### 6.4 Text engine (`internal/textkit`) — port textkit's design
Data model: `AttributedString{string, runs []Run}`, `Run{start, end, attrs}`; glyph runs after shaping `{glyphs, positions, glyphIndices, font}`.

Pipeline per paragraph (mirror textkit): 1 preprocess (textTransform, whitespace) → 2 scriptItemizer (Unicode script runs) → 3 bidi (UAX#9 levels via x/text; visual reorder per line after breaking; `direction: rtl` base) → 4 fontSubstitution (per-codepoint fallback across fontFamily list; emoji → image-replacement runs when an emoji source is registered) → 5 shape (typesetting HarfBuzz per run; letterSpacing/wordSpacing applied post-shaping as textkit does) → 6 linebreak (UAX#14 candidates + hyphenation points — soft hyphens, else Knuth-Liang en-US, else callback via evaluator; **Knuth-Plass total-fit with best-fit greedy fallback — port `knuthPlass.ts`/`bestFit.ts` logic and constants verbatim**) → 7 justification (word gaps then letter spacing, textkit ratios) → 8 truncation (maxLines + ellipsis) → 9 decoration rects (typesetting `LineMetric`; solid/dashed/dotted) → 10 verticalAlign sub/super baseline shifts (textkit 4.2 ratios).

Yoga integration: Text measure func runs steps 1–6 at the constraint width; cached by (content hash, resolved style hash, width bucket). Line-height: unitless × fontSize, absolute passes through; **copy textkit's exact `layoutParagraph` baseline math, don't improvise**.

Parity-plus (§8): HarfBuzz shaping + UAX#14 mean Arabic and CJK can exceed react-pdf. Keep compatible defaults; document divergence.

### 6.5 Font engine (`internal/fontstore`)
- Registry keyed by family → faces sorted by (style, weight); resolution = exact style, then CSS nearest-weight; missing style = error (no synthetic faces, matches react-pdf).
- Sources: URL (ctx-aware fetch honoring method/headers), file path, inline base64 from the contract, raw bytes (Go API). WOFF/WOFF2 → tdewolff/font decompress → typesetting parse.
- Standard 14: embed Adobe Core AFM metrics (redistribution permitted — include license file); expose react-pdf's 12 + Symbol/ZapfDingbats (parity-plus); WinAnsi glyph set like pdfkit.
- Subsetting at embed: used GIDs per font → tdewolff/font `Subset()` → CIDFontType2 (TTF) / Type0-CFF, ToUnicode CMap from the shaping cmap, `ABCDEF+` subset tags. Subset failure → fall back to full embed (correct, larger).
- Emoji source: template URL or builder; fetched (ctx, cached) as PNG; replaces emoji runs with inline images sized to the line. Parity-plus: allow local directory source.
- Store is mutex-guarded; Render mutates nothing but the load cache.

### 6.6 Image engine (`internal/imaging`)
- Source forms per §2.1 (URL/path/data-URI/bytes/{data,format}/callback-via-evaluator); magic-byte sniffing (JPEG/PNG/SVG).
- JPEG: parse SOF for dims/components; **embed original bytes as DCTDecode**; CMYK+Adobe APP14 → Decode inversion; EXIF orientation baked into placement transform (match react-pdf's jpeg.ts).
- PNG: stdlib decode (handles interlaced + 16-bit) → FlateDecode raw samples; alpha/tRNS → 8-bit gray SMask XObject. (Later optimization: raw-IDAT reuse for simple PNGs.)
- SVG source: svgparse → vector render; measured from width/height attrs, falling back to viewBox.
- Cache: LRU cap 30 (match react-pdf), keyed by URI/content hash; `cache: false` bypasses.
- objectFit/objectPosition math in layout; srcSet/sizes per react-pdf v4.4 selection rules.

### 6.7 PDF writer (`internal/pdf`) — the pdfkit replacement
Write-only PDF 1.3–1.7 serializer, ~5–10k LOC:

- **Object model:** indirect objects, dicts/arrays/names/strings/streams; classic xref (+ xref streams for 1.5+ when beneficial); deterministic object numbering (tree-walk order); trailer /ID from content hash + fixed seed (unless encryption randomness — below).
- **Content streams:** full operator builder — paths (m/l/c/v/y/re/h), painting (f/f*/S/B/B*/n/W/W*), graphics state (q/Q/cm/w/J/j/M/d/gs), color (rg/RG/k/K/cs/scn), text (BT/ET/Tf/Td/TJ/Tj/Tz/Tc/Tw/Ts/Tr), XObjects (Do), shading (sh). FlateDecode everywhere (level configurable; off for debugging).
- **Fonts:** Type1 standard-14 (AFM widths, WinAnsiEncoding); Type0/CIDFontType2 (subset TTF) and Type0/CIDFontType0 (subset CFF) with CIDToGIDMap, W arrays from shaped advances, ToUnicode CMaps.
- **Images:** /Image XObjects (DCTDecode | FlateDecode), /SMask, DeviceRGB/Gray/CMYK.
- **Transparency:** ExtGState /CA /ca.
- **Shading:** types 2 (axial) & 3 (radial), function type 2/3 stitching for multi-stop; pattern fills for SVG gradient paints (gradientTransform, gradientUnits objectBoundingBox|userSpaceOnUse).
- **Annotations:** /Link (URI + GoTo dest), /Text (Note), /Widget (forms); named destinations (Dests name tree) from `id` props.
- **Outlines:** nested bookmarks, dest coords, zoom/fit, open state.
- **AcroForm:** /AcroForm dict, field hierarchy (FieldSet → non-terminal fields), text fields (multiline/password/maxLen/quadding + /AA format JavaScript exactly as pdfkit emits), checkboxes (on/off appearance states, ZapfDingbats check), choice fields (combo/list, /Opt, edit/sort/multi flags), /DA from resolved font/size/color.
- **Metadata:** /Info, catalog /Lang, /PageMode, /PageLayout, header version.
- **Encryption:** standard security handler — R2 (RC4-40, 1.3), R3 (RC4-128, 1.4/1.5), R4/AESV2 (AES-128-CBC, 1.6/1.7), R6/AESV3 (AES-256, "1.7ext3"); permission bits per §2.1; stdlib crypto; mirror pdfkit's algorithms incl. the R6 hardened hash; seeded-RNG option for test determinism.
- Out of scope now (leave seams): PDF/A, XMP, tagged PDF (§8).
- Emission streams to `io.Writer` with offset tracking; page content built in memory per page.

### 6.8 SVG (`internal/svgparse` + svg render)
- Element set per §2.1 incl. Marker, Defs/ClipPath/gradients/Stop, nested Svg Text/Tspan.
- Fresh path-`d` parser (~500–1000 LOC) → IR MoveTo/LineTo/CubicTo/QuadTo/Close; arcs → cubic Béziers; W3C path corpus tests. Shapes (rect rx/ry, circle, ellipse, line, polyline, polygon) lower to the same IR.
- viewBox + preserveAspectRatio → root transform; transform attrs parsed to CTMs.
- SVG-scoped style/presentation-attr inheritance (separate from document inheritance, as react-pdf does).
- Paint servers: url(#id) from Defs; gradients → §6.7 shadings; clipPath → clip ops; markers at path/line/poly vertices.
- SVG Text: text engine for shaping, x/y/textAnchor/dominantBaseline positioning, no wrapping.

### 6.9 Layout pipeline (`internal/layout`)
Ordered steps over the tree, same names/order as react-pdf (§2.6). Notes:

- **resolveDimensions:** parallel flexbox tree; text/image measure funcs; CalculateLayout per page (auto-height pages: unconstrained height, then size page).
- **resolveTextLayout:** full paragraph layout at final widths; line boxes stored.
- **resolveAssets:** the only async step — errgroup-concurrent fetch/parse of fonts, images, emoji; all I/O behind Render ctx; image-src callbacks via evaluator.
- **resolveOrigins / resolveZIndex:** transform origins from boxes; stable sibling sort (zIndex among siblings only).
- **resolvePagination — the heart, port react-pdf's algorithm:**
  1. Per Page: wrap=false → clip, done.
  2. Else iterate: remaining tree + content height H → find break walking flow order accumulating heights; honor `break` (force, reset after), unbreakables (wrap=false/Image/Canvas: move whole; taller than an empty page → place and clip), `minPresenceAhead` lookahead, Text split at line granularity with orphans/widows, View box splitting (borders/padding at split edges **exactly as react-pdf does — verify against resolvePagination.ts during implementation**).
  3. Emit sub-page: fixed elements cloned onto every sub-page; remainder re-laid-out via flexbox per split.
  4. Render props: evaluate per sub-page with {pageNumber, subPageNumber} via CallbackEvaluator (or template-string substitution); after all pages, second pass re-evaluates Text render callbacks with totalPages/subPageTotalPages and re-renders those nodes only. Document react-pdf's caveat: dynamic text that changes width between passes can shift.
- **resolveLinkSubstitution:** `#id` → named destinations.

### 6.10 Paint (`internal/render`)
Per page: backgrounds (borderRadius-aware rounded rects), borders (per-side widths/colors/styles — port react-pdf's drawing incl. dashed/dotted and corner joins), clipping (overflow hidden + borderRadius), opacity (ExtGState), transforms (cm around transformOrigin), text (BT/TJ with shaped advances, decoration rects, baseline shifts), images (Do + objectFit transform), SVG IR → ops, Canvas ops replayed from the recording painter, debug overlays (react-pdf's colors), notes/links/widgets → annotations, bookmarks → outlines.

---

## 7. Testing & verification strategy

1. **Unit tests per subsystem:** stylesheet (units, shorthands, colors, inheritance), flexbox (Yoga fixtures + gap), textkit (textkit's own test strings, hyphenation, justification ratios, bidi order), svgparse (W3C corpus), pdf writer (spec-valid dicts per feature), contract (round-trip, versioning, unknown-prop warnings).
2. **Golden PDF tests:** deterministic output (fixed CreationDate, seeded IDs) → byte-compare ~40 scenario docs (one per feature cluster), fed as hand-authored contract JSON so they run before the npm package exists. Regeneration script with review diff.
3. **pdfcpu validate in CI:** every generated PDF must pass strict validation.
4. **Shared-TSX parity corpus (the headline test):** the same TSX files (ported react-pdf examples: resume, page-wrap, fractals, svg, form, page numbers) rendered through **both** `@react-pdf/renderer` 4.5.1 (Node job) and feast (`@feast/react` → engine). Diff (a) extracted text + positions (pdfcpu/pdftotext), (b) rasterized pages (pdftoppm) with perceptual diff, tolerance-based. Separate CI job (Node + poppler); non-blocking initially, promoted to blocking once stable.
5. **JS-side tests:** reconciler serialization snapshots (JSX → contract JSON), callback-table behavior, CLI bundle output runs on goja in CI (the compatibility canary).
6. **Cross-mode equivalence:** the same template rendered via embedded runtime, static tree, and sidecar must produce byte-identical PDFs (for docs without VM-only features).
7. **Fuzzing:** style parser, svg path parser, contract parser, PDF string/name escaping.
8. **Viewer smoke matrix (manual, per release):** Preview.app, Acrobat, Chrome, Firefox pdf.js — especially forms, encryption, outlines.

---

## 8. Parity matrix & flagged gaps

### Full parity (same code works)
All §2.1 components incl. forms and SVG; all §2.3 style props/units/media queries/inheritance; pagination semantics incl. render props (VM modes), fixed/break/minPresenceAhead/orphans/widows; `Font.register` + hyphenation callback + emoji source; JPEG/PNG/SVG images incl. srcSet; links/ids/hitSlop/nested bookmarks; encryption + permissions; metadata; debug mode. **Hooks, context, composition: full parity — it's real React.**

### Adapted (same capability, different mechanism)
| react-pdf | feast | Why |
|---|---|---|
| `renderToFile/Buffer/Stream` (Node) | `tpl.Render(ctx, props, io.Writer)` / `RenderTree` in Go | Rendering moved to Go — the point of the project |
| `onRender` | Accepted, no-op; `RenderInfo` return | No JS render loop to call back into |
| Data via JS closures over app state | Data via props from Go (JSON) | The Go/JS boundary |
| Web Worker advice for big docs | Goroutines; Render is synchronous | Runtime difference |

### Gaps — flagged (decide at review)
| Gap | Detail | Mitigation |
|---|---|---|
| **PDFViewer / PDFDownloadLink / BlobProvider / usePDF** | Browser React components; rendering doesn't happen in a browser | `feast-react dev` preview; source-compatible docs can be previewed in real react-pdf during development (engine-divergence caveat documented) |
| **Static-tree mode limits** | Arbitrary render-prop subtrees, hyphenation callbacks and auto-sized Canvas paint need a VM mode; static mode gets template-string render props only | Embedded runtime is the primary mode precisely so these work; static mode limits documented + serializer warnings |
| **React-on-goja risk** | React/reconciler compatibility with goja not yet proven for our exact stack | Phase R1 spike gate; fallback ladder goja → v8go (cgo tag) → sidecar; contract makes engine choice a config change |
| **Tagged PDF / accessibility, XMP, PDF/A** | react-pdf lacks these too | Writer leaves seams; post-v1 |
| **Emoji need network** | Same as react-pdf (CDN images) | Local emoji dir source (parity-plus) |
| **`@react-pdf/math` / `mermaid`** | Companion packages | Out of scope |

### Parity-plus (we exceed react-pdf — document as intentional divergence)
- Font formats: OTF/CFF, WOFF2, TTC (react-pdf: TTF/WOFF). Variable-font static instancing: stretch goal, off by default.
- Symbol + ZapfDingbats standard fonts exposed.
- CJK line breaking correct out of the box (UAX#14) vs react-pdf's callback hack (#692/#1662/#2917).
- RTL/Arabic via HarfBuzz-port shaping vs react-pdf's broken Arabic (#2900) — still **staged**: v1 ships LTR + script shaping + basic RTL paragraphs; complex mixed-direction justification may trail (§9).
- Deterministic byte-stable output.
- A stable, documented tree contract — anything (any language) can target the renderer.

---

## 9. Risks

| Risk | Severity | Mitigation |
|---|---|---|
| React/reconciler doesn't run (or runs too slowly) on goja | High | Phase R1 spike is a gate before dependent work; v8go build-tag fallback; sidecar fallback; contract isolates the choice |
| Pagination edge cases diverge from react-pdf (box splitting, borders at breaks, nested wraps) | High | Port while reading `resolvePagination.ts` side-by-side; shared-TSX corpus targets exactly these |
| Knuth-Plass/justification constants differ → different line breaks | Medium | Copy textkit constants verbatim; corpus text-position diffing |
| Yoga port is 2017-era; drift vs Yoga 3.x | Medium | Current Yoga fixture suite for exposed props; gap implemented fresh from Yoga 3 |
| Callback round-trips (VM ↔ layout) complicate pagination control flow | Medium | Single CallbackEvaluator seam; sync dispatch to VM goroutine; evaluated lazily per sub-page exactly where react-pdf calls render props |
| Bidi/RTL complexity balloons | High | Staged (§8); LTR paths never blocked |
| AcroForm viewer quirks | Medium | Emit what pdfkit emits; viewer smoke matrix |
| Encryption R6 correctness | Medium | PDF 2.0 spec test vectors; pdfcpu decrypt check |
| go-text/typesetting pre-1.0 churn | Low | Pin version; thin wrapper |
| tdewolff/font subsetting bugs on exotic fonts | Medium | Fall back to full embed |
| Contract version drift between npm package and Go engine | Medium | Semver'd contract + JSON Schema; engine warns on unknown, errors on major mismatch; CI renders npm output against engine on every PR |

---

## 10. Build phases (execution order for the implementing agent)

Each phase ends with green CI (unit + golden + pdfcpu-validate) and a runnable example. JS-side and Go-side work parallelize after Phase 2 freezes the contract.

**Phase 0 — Scaffolding.** Monorepo (Go module + npm workspace), CI for both toolchains, testdata layout, golden harness with determinism options, pdfcpu validate hook. Read folio + react-pdf sources; notes to `docs/notes/`.

**Phase R1 — JS runtime spike (GATE, timeboxed ~3 days).** React 18/19 production build + react-reconciler + a minimal host config running on goja: build a 1k-node tree using hooks + context; serialize; round-trip a callback invocation. Measure time/memory. Outcome recorded as an ADR: goja | v8go | sidecar-primary. **Do not proceed to Phase 3's npm package internals until this ADR exists** (everything else is engine-agnostic).

**Phase 1 — PDF writer core.** Objects/xref/streams/Info/catalog/page tree; content-stream builder (paths, colors, gstate, transforms); standard-14 Type1 text with AFM widths; Flate. ✅ *Accept: "Hello world A4" golden passes; validates; opens in Preview/Acrobat/pdf.js.*

**Phase 2 — Contract + styles + flexbox + static paint.** `feast-tree/v1` schema + parser (callbacks parsed but evaluator optional); internal tree + validation; stylesheet (react-pdf-shaped values: units, shorthands, colors, media queries, inheritance); vendored flexbox + gap backport + fixture tests; layout steps resolveStyles→resolveDimensions for View-only trees; paint backgrounds/borders/radius/opacity/transforms/debug overlays; `RenderTree` API v0. Tests feed hand-authored JSON. ✅ *Accept: flexbox scenario goldens (gap, %, absolute, aspectRatio) match Yoga-fixture expectations.*

**Phase 3 — `@feast/react` npm package.** Components + TS types mirroring react-pdf; reconciler host config; serializer + callback table; Font/StyleSheet shims; esbuild CLI (`build`, `--emit-tree`, `--inline-assets`). Snapshot tests: react-pdf example JSX → contract JSON → `RenderTree`. ✅ *Accept: resume-example TSX (text-free parts) renders via the full JS→Go path.* (Parallelizable with Phase 4 after the contract freezes at the end of Phase 2.)

**Phase 4 — Text.** Fontstore (TTF/OTF/WOFF/WOFF2, weights, fallback lists); textkit port (§6.4 full pipeline); measure-func integration; Type0 subset embedding + ToUnicode; spacing/lineHeight/maxLines/ellipsis/textTransform/sub-super/decoration; hyphenation (soft hyphen + en-US). ✅ *Accept: typography goldens; text-extraction diff vs react-pdf corpus for the resume example ≤ tolerance.*

**Phase 5 — Pagination + dynamic content.** Full resolvePagination port (wrap/sub-pages, break, fixed, minPresenceAhead, orphans/widows, unbreakables, auto-height, two-pass totalPages, zIndex); `CallbackEvaluator` interface; template-string render props (static mode); **jsruntime**: goja VM pool, bundle loading, `LoadTemplate`/`Template.Render`, callback bridge incl. hyphenation callback; sidecar client implementing the same interfaces. ✅ *Accept: page-wrap + page-numbers corpus docs paginate identically (page count + per-page text) to react-pdf, via the embedded runtime end-to-end.*

**Phase 6 — Images & navigation.** Imaging (§6.6 all src forms incl. callback-via-evaluator, JPEG DCT+EXIF+CMYK, PNG+SMask, cache, srcSet), objectFit/position, ImageBackground; links/ids/hitSlop; nested bookmarks/outlines; Note annotations. ✅ *Accept: image goldens incl. alpha PNG + CMYK JPEG; links/outlines work in 3 viewers.*

**Phase 7 — SVG.** Parser + IR + all elements incl. gradients, clipPath, markers, svg text/tspan; SVG-as-Image. ✅ *Accept: svg example corpus renders with perceptual diff ≤ tolerance.*

**Phase 8 — Canvas + gradients.** Recording painter (JS side + Go replay), full method list §2.6; axial/radial shadings shared with SVG gradients. ✅ *Accept: fractals example (Canvas-based) matches corpus via VM mode.*

**Phase 9 — Forms, encryption, polish.** AcroForm widgets; passwords + permissions across all four cipher levels; emoji source; RTL staged support; `feast-react dev` preview loop; viewer smoke matrix; docs (component/style/font reference mirroring react-pdf.org structure) + migration guide. ✅ *Accept: parity matrix §8 checked off.*

**Phase 10 — Parity promotion.** Shared-TSX corpus job promoted to blocking; cross-mode equivalence tests (§7.6) blocking; cut v0.1.

**Estimated totals:** Go ~27–37k LOC (incl. ~1–2k jsruntime) + vendored flexbox (~5k); TypeScript ~2–3k. Critical path: 0 → R1 → 1 → 2 → 4 → 5; phases 3, 6, 7, 8 parallelize.

---

## 11. Open questions for review (answer before Phase 0)

1. Names: Go module `github.com/swish/feast`? npm `@feast/react`? Public or internal?
2. Confirm goja-first engine strategy (pure Go, spike-gated) vs sidecar-first (zero engine risk, but production needs Node) — plan assumes goja-first with the fallback ladder.
3. Is RTL/Arabic a v1 blocker for your documents, or acceptable staged (§8)?
4. Forms + encryption (Phase 9): needed for v1, or can they slip post-v1?
5. TypeScript-only authoring guidance, or first-class plain-JS support in docs/CLI too? (TS recommended.)
6. Minimum Go version (propose 1.24); minimum React version to support in `@feast/react` (propose 18+).
7. folio: confirm greenfield-with-reference after the Phase 0 code read.
