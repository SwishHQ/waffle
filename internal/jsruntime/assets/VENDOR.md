# Vendored assets

These files are embedded into the Go binary (`//go:embed`) and are the complete,
self-contained JS runtime waffle executes on goja. No npm install or Node is
needed to build or use waffle.

## `react.production.min.js`

- **Source:** the `react` npm package, UMD production build
  (`node_modules/react/umd/react.production.min.js`).
- **Version:** React **18.3.1**.
- **License:** MIT (Facebook/Meta) — the license header is preserved at the top
  of the file.
- **To update:** in a scratch dir, `npm install react@<version>`, then copy
  `node_modules/react/umd/react.production.min.js` here. Verify it has no
  `process`/DOM references (waffle runs it on goja with no shims) and that the Go
  tests still pass.

## `waffle-react.js`

- The `@waffle/react` runtime — component primitives, the single-pass renderer
  (hooks dispatcher + context stack), `Font`/`StyleSheet`, and the serializer
  that emits `waffle-tree/v1` (including `{$cb}` render-prop callbacks).
- **This is the source of truth.** It was previously mirrored from an
  `@waffle/react` npm package under `packages/react/`, which has been removed —
  waffle is a pure Go library and does not publish or depend on an npm package.
  The esbuild plugin in `jsruntime.go` resolves the bare import `@waffle/react`
  (and `react`) to these embedded strings, so user JSX that does
  `import { View } from '@waffle/react'` works with nothing installed.
