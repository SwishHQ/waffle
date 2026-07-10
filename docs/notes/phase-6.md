# Phase 6 build notes — images (and later SVG, canvas)

## Done

- **`internal/imaging` — decoder. ✅** `Decode` sniffs format. JPEG is embedded
  losslessly as `DCTDecode` (original bytes passed through) after reading the SOF
  header for width/height/components (Gray/RGB/CMYK). PNG is decoded via stdlib
  (handles interlaced/16-bit/palette), re-encoded as 8-bit `DeviceRGB`
  `FlateDecode`, with any alpha channel becoming an 8-bit gray `SMask`.
  `DecodeDataURI` handles base64 data URIs. Tested. Deferred: CMYK Adobe
  `Decode`-array inversion; raw-IDAT fast path; EXIF orientation.

- **PDF image XObject + DrawImage. ✅** `pdf.ImageSpec`; `Content.DrawImage`
  (`q <w> 0 0 <h> <x> <y> cm /Im Do Q`, unit-square scaling); `Document.imageRef`
  builds the color XObject and an optional DeviceGray `/SMask` XObject; `AddPage`
  wires `/XObject` into page resources. Validated (pdfcpu strict, RGB + SMask).

- **Image layout/render wiring + aspect-ratio. ✅** flexbox gained `AspectRatio`
  (derives the auto dimension). The Image node decodes `src` (data URI / inline)
  to a `pdf.ImageSpec` on `Box.Image`, measures intrinsic size, sets natural
  aspect; `render.paintImage` draws it (Y-flip, objectFit "fill"). Tested
  (`width:100` on a 2:1 image → height 50) + alpha demo validates. Deferred:
  URL/file fetch, objectFit contain/cover, ImageBackground.

**Images now render end-to-end from waffle-tree JSON.**

- **SVG path parser + shapes. ✅** `internal/svgparse`: path-`d` tokenizer +
  parser (M/L/H/V/C/S/Q/T/Z, abs/rel, implicit repeats, tight number packing,
  quad→cubic, smooth-curve reflection) → MoveTo/LineTo/CubicTo/Close IR; shapes
  (rect incl. rounded, circle, ellipse, line, poly*). Tested. Deferred: arc (A)→
  cubic (line fallback for now).

- **SVG element renderer. ✅** Svg node is a measured leaf (viewBox intrinsic
  size + natural aspect); `Box.SVG` carries the subtree. `render/svg.go` maps
  viewBox→box (CTM with Y-flip) and draws G/Path/Rect/Circle/Ellipse/Line/Poly*
  → svgparse geometry → PDF path ops with fill/stroke/strokeWidth/fillRule
  (defaults fill=black, stroke=none). Tested + demo (rect/circle/polygon/quad
  path) validates. Deferred: transform attr, gradients, clipPath, svg `<text>`,
  style-based (vs prop) paint.

**SVG now renders end-to-end from waffle-tree JSON.**

## Next

1. **Canvas** — the painter API (`paint(painter, w, h)`): a recording painter on
   the JS side (later) or, for the static path, Canvas with explicit ops. Shares
   the vector-drawing code with SVG.
2. **Gradients / clipPath** for SVG (shading dictionaries in the pdf writer).
3. **The React front-end** — Phase R1 goja spike + `@waffle/react`. The last big
   chunk / the plan's gated milestone; **flag to the user before starting**.
2. **The React front-end** — Phase R1 goja spike + `@waffle/react`. Largest
   remaining chunk / the plan's gated milestone; **flag to the user first**.
3. Async asset resolution (URL/file), objectFit, flexbox min/max + flex-wrap.
