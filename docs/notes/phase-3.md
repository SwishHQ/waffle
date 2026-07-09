# Phase 3 build notes — the React front-end (@feast/react)

> **Superseded (2026-07):** the standalone `packages/react/` npm package described
> below was removed. Its runtime now lives embedded at
> `internal/jsruntime/assets/feast-react.js` and runs in-process on goja (see
> phase-R1). feast is a pure Go library with no npm package. Kept as a historical
> log of how the authoring surface was first built.

## Done — the static path (React → feast-tree JSON → Go PDF) ✅ MILESTONE

- **`packages/react`** (npm, ESM):
  - Components are the react-pdf primitive strings (`View = 'VIEW'`, …), so JSX
    `<View>` creates a `{type:'VIEW'}` host element.
  - A one-shot renderer (`render.js`) resolves function/class components with
    their props and turns string/number children into `TEXT_INSTANCE` nodes.
  - `serialize.js` → `feast-tree/v1` JSON; `Font`/`StyleSheet` shims (Font.register
    collected into the doc header; function props dropped — closures can't cross
    the JSON boundary — while string `render` templates survive for page numbers).
  - Node tests (`node --test`) cover components, styles, fonts, template props,
    root validation. Added a `react` job to CI.
- **`cmd/feast`** — CLI: `feast tree.json out.pdf` (feast-tree JSON → PDF via
  `feast.RenderTree`).
- **End-to-end proven**: `examples/hello.mjs` authors a document in React (a
  function component + `.map` over data + StyleSheet + a fixed page-number
  footer); `node hello.mjs | feast → PDF` renders and passes pdfcpu strict. This
  is the project's premise working: **write JSX, get a Go-rendered PDF.**

## Next — full React fidelity + in-process runtime

1. **react-reconciler host config** — replace the one-shot renderer so hooks
   (`useState`, `useContext`, `useMemo`) and context work, i.e. *real* React.
   Serialize the committed tree the same way.
2. **esbuild CLI (`feast-react build`)** — bundle a user's JSX/TSX (React +
   reconciler + user code) to a single file; `--emit-tree` for the static path;
   `--inline-assets` to base64 local fonts/images.
3. **Phase R1 — goja in-process runtime** — embed the bundle in Go
   (`feast.LoadTemplate`/`Template.Render`) so no Node is needed at render time,
   and function render props / canvas paint evaluate via the VM. The plan's gated
   spike; do after the reconciler works under Node.
4. Forms + encryption, gradients, async asset fetch — remaining engine features.
