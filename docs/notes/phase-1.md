# Phase 1 build notes — PDF writer core

Status: complete. `internal/pdf` produces valid, deterministic PDFs and passes
`pdfcpu validate -m strict`.

## What exists

- `object.go` — PDF object model (Null, Boolean, Integer, Real, Name,
  LiteralString, HexString, `TextString` (ASCII or UTF-16BE), Array, Dict,
  Reference, Stream). Dictionaries serialize with sorted keys for determinism.
  `Real` formatting trims to ≤6 decimals with no scientific notation.
- `writer.go` — `Writer` manages the indirect object table (`Alloc`/`Set`/`Add`),
  serializes header + body + classic xref + trailer, and implements `io.WriterTo`.
  `FlateStream` zlib-compresses stream data.
- `content.go` — `Content` operator builder: graphics state, path construction
  and painting, clipping, DeviceRGB/Gray color, and the text operators. It records
  standard fonts used (in first-use order) and assigns `F1..Fn` resource names.
- `document.go` — `Document` assembles metadata, the page tree, and shared font
  objects; wires each page's `/Font` resources from the fonts its content used;
  derives a deterministic file `/ID` from metadata + object count; formats PDF
  dates in UTC.
- `font.go` + `afm/` — the 14 standard fonts. AFM metrics are embedded and parsed
  for text measurement; `afm/winansi.go` is the full WinAnsiEncoding
  code↔rune↔glyph-name table used for measurement and `Tj` encoding.

## Key decisions

- **Standard-14 fonts are emitted without a `/Widths` array or FontDescriptor.**
  This is valid (viewers carry built-in metrics for these fonts) and sidesteps the
  descriptor requirement. AFM widths are still used *internally* for measurement,
  and those match the viewer's built-in metrics exactly, so computed positions
  align with rendered glyphs. Embedded/subset fonts with explicit widths arrive
  with the Type0/CIDFontType2 work in the text phase.
- **Symbol/ZapfDingbats** are emitted without `/Encoding` (built-in encoding);
  WinAnsi measurement is Latin-oriented, so precise measurement of those two fonts
  is deferred (not needed for the standard text path).
- **Determinism**: sorted dict keys + ordered object allocation + metadata-derived
  `/ID` + UTC dates ⇒ byte-identical output for identical input. The committed
  golden (`internal/pdf/testdata/hello.golden.pdf`) is byte-exact; because the
  content stream is Flate-compressed it can drift across Go toolchain versions —
  regenerate with `WAFFLE_UPDATE=1` if that happens. The in-process determinism
  test does not depend on the golden.

## Still open before Phase 2

- **folio code read** (budgeted in Phase 0): evaluate `github.com/carlos7ags/folio`'s
  writer/layout patterns as a design reference before building the flexbox +
  layout pipeline. Not yet done.
- **Phase R1 (JS-runtime spike)** remains the gate before the `@waffle/react`
  package internals: prove React + react-reconciler run on goja. Phases 1–2 are
  engine-agnostic and proceed regardless.
