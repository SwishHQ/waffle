# Phase 2 build notes — contract, styles, flexbox, static paint

Running log; updated per work chunk.

## Done

- **`internal/contract` — waffle-tree/v1 parser.** Wire types (`Tree`, `Document`,
  `Node`, `FontRegistration`/`FontFace`, `EmojiSource`), node-type constants
  mirroring react-pdf primitives, and `Parse([]byte)`:
  - Version gating: accepts `waffle-tree/v1` and minor `v1.x` (forward-compatible),
    rejects other majors and missing/garbage versions.
  - `$cb` → `CallbackRef`, `$inline` → `InlineAsset`, resolved recursively through
    props/maps/slices. Numbers preserved as `json.Number` (exact style values).
  - Unknown node types and props tolerated → `Tree.Warnings`, never errors, so a
    newer producer degrades gracefully.
  - `RequiresEvaluator()` reports whether any function-valued prop (render, paint,
    hyphenation, image-src callback) is present → distinguishes static-tree-mode
    documents from ones needing a VM execution mode.
  - Callback consistency check (referenced-but-not-declared / declared-but-not-
    referenced) → warnings.
  - Tests cover the §3.2 sample doc, version gating table, unknown-type warning,
    static-tree detection, callback consistency.

- **`internal/tree` — node model + validation.** Typed `Node` built from the
  contract with structural props parsed (wrap/fixed/break/minPresenceAhead/debug/
  id/render/style); `Validate` enforces react-pdf's tree rules with element-path
  messages (Page only under Document; Text children ∈ {text, Text, Link}; SVG
  primitives only inside Svg; Note text-only; TEXT_INSTANCE parent). `Clone()`
  deep-copy for pagination; `Path()` for error locations. Tested.
- **`internal/stylesheet` — primitives.** `Value` + unit parsing/resolution
  (pt/in/mm/cm/px/rem absolute; %/vw/vh resolved with a basis/page-dims at layout;
  `auto`); color parsing (hex 3/4/6/8, rgb/rgba incl. %, hsl/hsla, full CSS named
  set, transparent) → `Color`; `ParseFontWeight` keyword table; `Flatten`
  (object/array-of-styles, later-wins, nested, nil-safe). Tested. Note: px uses
  ×72/dpi without rounding — flagged in code to verify vs react-pdf units.ts.

- **`internal/stylesheet` — media queries + shorthands.** `MatchMedia`/
  `ApplyMedia` for `@media` keys (min/max-width, min/max-height, orientation,
  `and`-combined, optional parens; bare `@media` matches all) evaluated against
  page dims, merged deterministically (sorted key order). `ExpandShorthands`:
  margin/padding (box 1–4 values, Horizontal/Vertical, explicit-longhand wins),
  border (generic + per-side "1pt solid red", functional colors preserved,
  per-side overrides), single-value borderRadius + per-corner, gap (row/column).
  Tested. Deferred: multi-value borderRadius, flex + transform-string shorthands.

- **`internal/stylesheet` — inheritance + resolution pipeline.** `Inherit(parent,
  child, isText)` cascades the react-pdf inherited property set (color, font*,
  letterSpacing, opacity, textTransform, lineHeight, textAlign, visibility,
  wordSpacing; Text-only backgroundColor), child-wins, with `textDecoration`
  merged (nested underline/line-through combine; child `none` suppresses).
  `Resolve(style, mediaCtx)` = Flatten → ApplyMedia → ExpandShorthands. Tested.
  **The typed `Style` struct (PLAN §3.3) is deliberately deferred to the layout
  phase**, so it's shaped to what the flexbox/render engines actually consume
  rather than built speculatively; flex + transform-string shorthands fold in
  there too.

- **`internal/flexbox` — own flexbox engine (core, no-wrap).** Recursive
  `measure` (intrinsic content sizing) + `arrange` (grow/shrink main
  distribution, justify-content, align-items/self incl. stretch, full box model
  margin/padding/border, gap), row + column, border-box parent-relative coords.
  **Decision: written from scratch, not vendored** — the plan's named source
  (kjk/flex) has a deleted upstream and carries Facebook's BSD+PATENTS rider,
  which I won't bake into a distributable library; owning it also gives
  pagination the re-layout control it needs. Tested against hand-computed
  layouts. Deferred: flex-wrap, %, aspect-ratio, min/max, absolute positioning,
  reversed directions.

## Directive change: "till it is complete"

The user switched the loop to run **until the library is built**, resolving the
earlier pause points — I now drive the critical path and make the architectural
calls myself (documenting each). The flexbox decision above is the first of
these. The R1 JS-runtime spike + `@waffle/react` npm package remain later phases.

- **`internal/layout` — the layout pipeline.** Page-size resolution (named
  subset + numeric/array/object + orientation swap), per-node style
  resolution+inheritance, mapping resolved styles → `flexbox.Style` (unit
  resolution; % treated as auto for now), flexbox `Calculate`, and a `Box` tree
  with absolute frames + resolved styles. View-only (TEXT_INSTANCE skipped until
  the text phase). Tested end-to-end contract→tree→layout: padding/fixed child,
  row flex-grow, named/oriented page size, in→pt margins, multi-page.

- **`internal/render` + `waffle.RenderTree` — the first end-to-end PDF. ✅
  MILESTONE.** Paints the `layout.Box` tree (background fills, per-side borders
  with color/current-color fallback, Y-flip to PDF coordinates) through the
  Phase 1 writer. `waffle.RenderTree(ctx, treeJSON, w)` runs the whole pipeline
  (contract → tree → layout → render → PDF) and wires Document props →
  PDF metadata (title/author/pdfVersion/lang/pageMode/pageLayout). Verified: the
  `examples/boxes` demo and the end-to-end test render a styled View tree to a
  **pdfcpu-strict-valid** PDF; a unit test inflates the content stream to confirm
  the Y-flip coordinates. Deferred: opacity (needs ExtGState), border-radius,
  dashed/dotted borders, corner miter joins.

**Phase 2's View-only vertical slice is complete** — a React-authored tree
renders to a real PDF. 8 Go packages, all green.

- **Flexbox percentages.** `Dim` gained a percent kind; `width`/`height`/
  `flexBasis` percentages resolve against the parent's content size during
  layout (also fixed `measure` to always receive (contentW, contentH) so % picks
  the right axis regardless of flex-direction). Wired through `layout.dim`.
  Tested at both levels. Deferred still: min/max, aspect-ratio, flex-wrap,
  percentage margins/padding.

## Next (critical path)

1. **Remaining flexbox refinements** (min/max, aspect-ratio, flex-wrap) — smaller,
   can slot in opportunistically.
2. **Phase 4 — text (the big one, next focus).** `internal/fontstore` (register/resolve fonts; go-text/
   typesetting parse+shape; tdewolff/font subsetting) and `internal/textkit`
   (attributed strings → runs → shape → line-break), wired into layout as measure
   funcs, with Type0 subset embedding in `internal/pdf`. This unblocks real
   documents (text is most of any PDF).
3. **Phase 5 — pagination**; then images/SVG/canvas; then the R1 JS-runtime spike
   + `@waffle/react` npm package.

## PAUSE for user visibility before

- **Vendoring the Yoga/flexbox port** (`internal/flexbox`) — a large external code
  drop (licensing/provenance); surface before importing.
- **Phase R1 — the JS-runtime spike** (React + react-reconciler on goja) — stands
  up the npm/React/esbuild toolchain; a bigger architectural commitment worth a
  check-in. Phases 1–2 remain engine-agnostic and proceed without it.
