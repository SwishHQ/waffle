# Phase 5 build notes — pagination

## Done

- **Pagination core (block-level splitting). ✅** `internal/layout/pagination.go`:
  after a Page is laid out (children stack at natural Y, overflowing the page
  height), `paginate` distributes the top-level children across output pages by
  cumulative height — a child that doesn't fit the remaining space moves whole to
  the next page, its subtree shifted up. Each output page keeps the page's own
  background/padding. `wrap: false` on a Page disables splitting (single, clipped
  page). Multi-page output flows through the renderer unchanged. Verified: the
  `examples/multipage` demo (24 rows) renders 3 A7 pages, pdfcpu-valid.

- **Mid-child splitting. ✅** `paginate` is now a fill-and-split loop: a
  straddling Text splits at a line boundary, a straddling View splits its
  children recursively (`splitBox`/`splitTextBox`/`splitViewBox`), and
  unbreakable leaves move whole (with a progress guard for content taller than a
  page). Verified: a long justified paragraph splits across pages and renders
  pdfcpu-valid. Deferred: View padding/border behavior at the split seam,
  keeping Text baselines exact when a Text has its own padding.

- **Forced page break (`break: true`). ✅** `splitFlow` ends the current page
  before a break element (unless it's first on the page). Tested. Deferred: break
  inside a fitting View subtree (only flow-level breaks handled).

- **Absolute positioning + `fixed`. ✅** flexbox: `position:absolute` takes a
  child out of flow and positions it by width/height + top/right/bottom/left
  against the parent content box (excluded from flow measure/arrange). layout maps
  `position` + insets. pagination: `fixed` elements are excluded from the flow and
  appended (at their absolute position) to every output page — enabling repeating
  headers/footers. Tested. Deferred: relative-position offsets (treated as static);
  fixed-without-absolute reserves flow space on page 1.

- **`minPresenceAhead` + orphans/widows. ✅** `splitFlow` breaks before a fitting
  element that would leave < N pt after it (keep-with-next); `splitTextBox` keeps
  ≥ orphans lines on the page and carries ≥ widows to the next (defaults 2/2),
  moving the whole Text when both can't be met. Tested.

**Phase 5 (pagination) is now substantially complete** for static content:
overflow splitting, mid-child (Text/View) splitting, break, fixed, absolute,
minPresenceAhead, orphans/widows.

- **Static-mode page numbers. ✅** A Text with a string `render` template
  (`"{pageNumber} / {totalPages}"`) is substituted per page after pagination
  (global numbering); fixed footers are cloned per page so each shows its own
  number. Tested (1/2, 2/2). Deferred: function render props (need a VM),
  `subPageNumber` (==pageNumber for now).

**Phase 5 is complete for static content.** The engine now renders paginated,
text-flowing, header/footer documents from waffle-tree JSON.

## Next — remaining engine, then the React front-end

1. **Phase 6 — images.** `internal/imaging`: JPEG (DCTDecode passthrough) + PNG
   (decode → flate, alpha → SMask); Image XObjects in `internal/pdf`; wire into
   layout (objectFit) + render. Self-contained, high-value.
2. **Phase 6/7 — SVG parser + render, Canvas** (self-contained leaf work).
3. **The React front-end** — Phase R1 goja spike + `@waffle/react` npm package.
   The largest remaining chunk and the plan's gated milestone; **flag to the user
   before starting** (stands up the JS toolchain, React-on-goja risk).
