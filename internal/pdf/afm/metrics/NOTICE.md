# Adobe Core 14 AFM metrics

The `*.afm` files in this directory are the Adobe Font Metrics for the 14
standard PDF fonts (Courier, Helvetica, Times families, Symbol, ZapfDingbats).

They originate from Adobe Systems and are redistributed here as obtained from the
Apache PDFBox project (`pdfbox/src/main/resources/org/apache/pdfbox/resources/afm`).
Adobe's accompanying notice permits redistribution of the metric data provided the
copyright notice is retained; each file keeps its original
`Comment Copyright (c) 1985–1997 Adobe Systems Incorporated. All Rights Reserved.`
header.

These files contain only font *metrics* (glyph advance widths and bounding
boxes), not font outlines. They are embedded into the binary via `go:embed` and
used solely for text measurement; the 14 standard fonts are assumed present in
every conforming PDF viewer, so no outlines are embedded in output documents.
