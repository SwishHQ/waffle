# Phase 4 build notes — text

Running log for the text phase (fonts, shaping, line breaking, embedding).

## Done

- **`internal/fontstore` — registry + resolution + measurement.** `Store.Register`
  parses embedded fonts with go-text/typesetting (`font.ParseTTF`); `Resolve`
  matches by family, then style, then nearest weight (ties → lighter), and falls
  back to the 14 standard PDF fonts by family name (Helvetica/Times/Courier +
  Symbol/ZapfDingbats → afm) so standard families work unregistered. `Face`
  measures via nominal glyph advances (`NominalGlyph`→`HorizontalAdvance`/`Upem`).
  A common `Font` interface (`StringWidth(text, size)`) is satisfied by both
  `Face` and `afm.Metrics`. Tested with x/image's `goregular`.

## Dependencies (first external deps in the project)

- `github.com/go-text/typesetting v0.3.4` — font parsing + (later) HarfBuzz-port
  shaping + UAX segmentation. BSD-3 + Unlicense.
- `golang.org/x/image v0.23.0` — **required transitively by typesetting** (pinned
  there via MVS), and used directly for the `goregular` test font. NOTE: x/image
  ≥ v0.24 requires Go 1.25; because typesetting pins v0.23.0, `go mod tidy`
  correctly settles on v0.23.0 and the module stays on **Go 1.24**. Don't add a
  direct requirement on a newer x/image without also bumping the go directive.

- **Text layout + render (standard fonts, single line). ✅ First visible text.**
  Text nodes are measured leaves: `collectText` flattens TEXT_INSTANCE
  descendants; `resolveText` picks the standard base font (family/weight/style),
  size (default 18), ascent, line-height, color; the flexbox measure func returns
  (advance width, line height). `render.paintText` draws the run at the baseline
  (Y-flipped, inset by the box's padding/border). Also fixed the flexbox measured-
  leaf border-box to include the node's own padding/border. The `examples/text`
  demo renders title/subtitle/tags/mono across Helvetica/Times/Courier — pdfcpu-
  valid, text extractable. Deferred: line wrapping, rich inline runs (nested
  Text/Link with per-run styles), text-align, embedded (non-standard) fonts,
  Text padding on the measure vs. paint (basic case works).
- **Text line wrapping (greedy). ✅** `textResolve.measure` greedily word-wraps
  to the available width using real advance widths, stores the lines, and reports
  height = lines × line-height; `render.paintText` paints each line via a text
  matrix at its baseline. The `examples/text` demo now wraps a paragraph across
  4 lines. Deferred: long-word char breaking, explicit `\n`, Knuth-Plass total-
  fit, wrap width minus the Text's own padding.
- **Text-align (left/center/right/justify). ✅** `render.paintText` offsets each
  line within the content width; justify widens inter-word gaps via the PDF `Tw`
  operator (last line and single-word/overflow lines excepted). Tested (center
  offset + positive justify Tw).

## Next

1. **`internal/textkit`** — attributed strings → runs → shaping (typesetting
   HarfBuzz) → line breaking (UAX#14 + Knuth-Plass). Wraps text to the box width;
   feeds the measure func.
3. **Type0/CIDFontType2 subset embedding** in `internal/pdf` (+ tdewolff/font for
   subsetting) so registered TTFs render, with ToUnicode CMaps.
