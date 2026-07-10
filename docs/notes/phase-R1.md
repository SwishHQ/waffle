# Phase R1 build notes — goja, the in-process JS engine ✅ SPIKE PASSED

**Decision (2026-07): goja is the only JS engine.** No Node sidecar, no v8go, no
cgo. React runs inside the Go process, so waffle is a single self-contained Go
library: `go get`, author in JSX, get a PDF — Node is never required.

## What landed

- **`internal/jsruntime`** — the engine. Pure Go, two libraries:
  - **esbuild** (`github.com/evanw/esbuild/pkg/api`) transpiles the user's
    JSX/TSX and bundles it, entirely in memory, to an ES2015 IIFE. An in-memory
    plugin resolves `react`, `react/jsx-runtime`, `react/jsx-dev-runtime` and
    `@waffle/react` from embedded strings — no filesystem, no node_modules.
  - **goja** (`github.com/dop251/goja`) executes the IIFE and calls
    `__waffle.render(propsJSON)` → `waffle-tree/v1` JSON.
  - `Compile(source, opts) → *Program`; `Program.Render(propsJSON) → treeJSON`.
    A `Program` compiles once and renders many times (fresh VM per render, so
    it's concurrency-safe).
- **Automatic JSX runtime** — user files need *no* React import. esbuild rewrites
  `<X/>` to `jsx(X, props)` from `react/jsx-runtime`; our 6-line shim forwards to
  `React.createElement` (which merges a config incl. children — classic
  semantics). Importing React explicitly still works.
- **Vendored React** — `assets/react.production.min.js` (10.7 KB, MIT). Verified
  it has no `process`/DOM references, so it loads on goja with zero shims.
- **Embedded `@waffle/react`** — `assets/waffle-react.js` (primitives + single-pass
  renderer + Font/StyleSheet + serializer) is the whole runtime as one module and
  the sole source of truth. (An earlier `packages/react/` npm mirror was removed
  2026-07 — waffle is a pure Go library; see `assets/VENDOR.md`.)
- **Public API** (`template.go`): `LoadTemplate`/`Template.Render`/`Template.Tree`
  and one-shot `RenderReact`. `Render` marshals Go props → JSON → goja → tree →
  the existing `RenderTree` pipeline → PDF.
- **CLI** (`cmd/waffle`): `waffle doc.jsx out.pdf [props.json]` runs the goja path;
  `.json` input still uses `RenderTree`.
- **Example** (`examples/react`): a data-driven invoice — JSX authored, Go
  supplies line items as props, renders `invoice.pdf` in-process.

## Proof

- `internal/jsruntime/jsruntime_test.go`: function components, props, `.map`,
  bare-element default export, and non-Document rejection — all on goja.
- `template_test.go`: a full react-pdf-style document (styles, `.map`, fixed
  page-number footer) compiled and rendered to a **pdfcpu-strict-valid** PDF,
  plus a second render with different props reusing the compiled template.
- `go run ./examples/react` and `waffle invoice.jsx …` both validate strict.

## Toolchain care

- goja `@latest` requires Go 1.25; pinned to `065cd970` (2026-03-11), the newest
  commit before its go.mod bumped to 1.25. esbuild 0.28.1 needs only Go 1.13.
  `go mod tidy` settles on `go 1.24`. CI sets `GOTOOLCHAIN=local`.

## Hooks + context (done ✅)

Rather than drag `react-reconciler` (and `scheduler` + an event loop) into goja,
waffle renders a document as a **pure function of props in one synchronous pass**
with a hooks dispatcher installed — the right model for a file (there is no
screen to update). Implemented in the embedded runtime `assets/waffle-react.js`:

- Sets `React …ReactCurrentDispatcher.current` to a dispatcher supporting
  `useState`, `useReducer`, `useMemo`, `useCallback`, `useRef`, `useContext`,
  `useId`, `useSyncExternalStore`, `useTransition`, `useDeferredValue`; effects
  and state setters are inert (single pass, no re-render — matches how react-pdf
  captures a tree).
- A context stack makes `<Ctx.Provider>` / `useContext` / `<Ctx.Consumer>`
  propagate (keyed by `provider._context`; the Consumer resolves via
  `type._context`).
- Unwraps the exotic element types `React.memo` and `React.forwardRef`.
- **Gotcha fixed:** the goja entry now wraps a function default export as
  `React.createElement(App, props)` instead of calling `App(props)` directly, so
  the *top-level* component runs inside the dispatcher scope (otherwise its hooks
  throw while nested components' hooks work).

Tested under goja (`internal/jsruntime/hooks_test.go`), plus a hooks+context
document rendered to a strict-valid PDF (`TestRenderReactHooksToPDF`).

## Callback bridge — foundation done ✅ (layout wiring next)

Function render-props (`render={({pageNumber,totalPages}) => …}`) are no longer
dropped. Built and tested this pass:

- The serializer registers a function `render` prop in a VM-side `callbacks` map
  and emits `{$cb:"cb_N"}` in the tree, declaring the ids in a top-level
  `callbacks` array (the contract already parsed `$cb` → `CallbackRef` and
  `tree.Node.Render`). String `render` templates are untouched.
- `jsruntime.Instance` keeps the goja VM alive after the initial render:
  `Program.Instantiate(props) → *Instance`; `Instance.Tree()` is the JSON;
  `Instance.EvalCallback(id, ctxJSON)` runs the closure with the page context and
  returns a JSON array of waffle-tree nodes. `Program.Render` is now a one-shot
  wrapper over it. Evaluating a callback keeps the registry intact so callbacks
  don't clobber each other; each is re-evaluable per page.
- Tested under goja (`internal/jsruntime/callbacks_test.go`) — string and element
  results, distinct ids, re-eval with different page contexts.

**Layout wiring — done ✅.** `render={fn}` now paints per-page content in the PDF:

- `layout.Evaluator` (`internal/layout/callback.go`) — `EvalText(id, PageContext)
  (string, error)`; `layout.Options.Eval` carries it. `PageContext` is
  `{pageNumber,totalPages,subPageNumber,subPageTotalPages}`.
- `resolveText` detects a `tree.Node.Render` callback, seeds initial content by
  evaluating it at page 1/1 (representative measurement), and stores the id on
  `TextInfo.CallbackID`. `applyPageNumbers` (post-pagination) re-evaluates per
  page — the exact dynamic analogue of the string-template path, in the same
  pass.
- `Template.Render` now keeps the VM alive: `Instantiate` → parse → the shared
  `renderContract(ct, eval, w)` with `instanceEvaluator{inst}`, which calls the
  VM and extracts the callback's text. The static `RenderTree` passes a nil
  evaluator (callback render-props resolve to empty, no crash).
- Tested: `internal/layout/callback_test.go` (stub evaluator, per-page footer
  across 2 pages, and the no-evaluator no-op) and `template_test.go`
  (`TestCallbackFooterRealEvaluatorPerPage` asserts "Page 1 / 2" then "Page 2 / 2"
  via the real VM; `TestRenderReactCallbackFooterPDF` renders a strict-valid
  multi-page PDF).

Scope note: covers the dominant Text→text idiom (page-number footers). A render
prop returning styled elements has only its text extracted so far (outer Text's
style governs); Canvas `paint` and hyphenation callbacks are still to come.

## Canvas `paint={fn}` — done ✅

`<Canvas paint={(painter, w, h) => …} />` now works the react-pdf way, in-process:

- `paint` joins `render` in `CALLBACK_PROPS`, so the serializer emits `{$cb}`.
- `makePainter()` (embedded runtime) is a fluent recorder: each call
  (`moveTo`/`rect`/`circle`/`fillColor`/`fill`/`save`/`rotate`/… and gradient/
  opacity no-ops) pushes `{op, args}` in the exact shape the Go replayer
  (`internal/render/canvas.go`) already reads, and returns the painter for
  chaining. `evalPaintString(id, w, h)` runs the callback against a fresh painter
  and returns the ops JSON.
- `Instance.EvalPaint(id, w, h)` bridges it; `layout.Evaluator` gains `EvalPaint`;
  `resolveCanvasPaint` (post-layout, once the canvas frame is known) evaluates and
  attaches `box.Canvas`. The width/height passed in are the resolved frame size.
- Tested: `jsruntime` (painter records `{op,args}`, w/h threaded), `layout`
  (stub → `box.Canvas` at frame size), and `waffle`
  (`TestCanvasPaintRealEvaluator` rect+circle via the real VM;
  `TestRenderReactCanvasPaintPDF` a strict-valid PDF).

## Next

1. Render props that return **styled element subtrees** (not just text) — lay out
   the returned nodes in place rather than text-extracting them.
2. **Optional, later:** a full `react-reconciler` host config *if* someone needs
   stateful re-render / effects before capture — currently a documented non-goal.
3. Remaining engine features: forms (AcroForm), encryption, SVG gradients,
   external font/image fetch.
