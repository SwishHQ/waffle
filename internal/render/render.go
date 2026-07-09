// Package render paints a laid-out document (layout.Result) onto the PDF writer,
// producing the final bytes. It is feast's analogue of react-pdf's render step,
// for the View-only subset: background fills and borders. Text, images, SVG,
// transforms, opacity, and border-radius arrive in later phases.
//
// Layout uses a top-left origin (y grows downward); PDF uses a bottom-left
// origin (y grows upward). Every box is Y-flipped on the way out.
package render

import (
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/swish/feast/internal/contract"
	"github.com/swish/feast/internal/layout"
	"github.com/swish/feast/internal/pdf"
	"github.com/swish/feast/internal/pdf/afm"
	"github.com/swish/feast/internal/stylesheet"
)

// Options configure rendering.
type Options struct {
	DPI     float64     // default 72
	RemBase float64     // default 18
	Doc     pdf.Options // document metadata passthrough
}

// Render paints every page of res and writes the PDF to w.
func Render(res *layout.Result, w io.Writer, opts Options) error {
	docOpts := opts.Doc
	if docOpts.Producer == "" {
		docOpts.Producer = "feast"
	}
	doc := pdf.New(docOpts)

	dpi := opts.DPI
	if dpi == 0 {
		dpi = 72
	}
	remBase := opts.RemBase
	if remBase == 0 {
		remBase = 18
	}

	for _, page := range res.Pages {
		ctx := stylesheet.Context{DPI: dpi, RemBase: remBase, PageW: page.Width, PageH: page.Height}
		c := pdf.NewContent()
		if page.Root != nil {
			paintBox(c, page.Root, page.Height, ctx, 1)
		}
		doc.AddPage(page.Width, page.Height, c)
	}

	_, err := doc.WriteTo(w)
	return err
}

// paintBox paints a box and then its children (so children render on top).
// parentOpacity is the inherited alpha; a box's own opacity multiplies into it
// (matching CSS nesting) and applies to the box and its whole subtree.
func paintBox(c *pdf.Content, box *layout.Box, pageH float64, ctx stylesheet.Context, parentOpacity float64) {
	op := parentOpacity * opacityOf(box.Style)
	fade := opacityOf(box.Style) < 1
	if fade {
		c.Save().SetAlpha(op)
	}
	paintBackground(c, box, pageH, ctx)
	paintImage(c, box, pageH, ctx)
	paintSVG(c, box, pageH)
	paintCanvas(c, box, pageH)
	paintBorders(c, box, pageH, ctx)
	paintText(c, box, pageH, ctx)
	paintLink(c, box, pageH)
	for _, child := range box.Children {
		paintBox(c, child, pageH, ctx, op)
	}
	if fade {
		c.Restore()
	}
}

// opacityOf reads the opacity style (0–1; percent supported), defaulting to 1.
func opacityOf(style map[string]any) float64 {
	v, ok := style["opacity"]
	if !ok {
		return 1
	}
	val, err := stylesheet.ParseValue(v)
	if err != nil || val.IsAuto() {
		return 1
	}
	a := val.Amount
	if val.Unit == stylesheet.UnitPercent {
		a /= 100
	}
	if a < 0 {
		return 0
	}
	if a > 1 {
		return 1
	}
	return a
}

// paintLink records a hyperlink annotation over a Link box's frame. The URI comes
// from the Link's src (or href) prop. Block-level Links (a <Link> wrapping
// content) get their own frame; inline links inside a Text await the text engine.
func paintLink(c *pdf.Content, box *layout.Box, pageH float64) {
	if box.Node == nil || box.Node.Type != contract.TypeLink {
		return
	}
	uri := linkURI(box.Node.Props)
	if uri == "" {
		return
	}
	f := box.Frame
	c.AddLink(f.X, pageH-(f.Y+f.H), f.X+f.W, pageH-f.Y, uri)
}

func linkURI(props map[string]any) string {
	if props == nil {
		return ""
	}
	for _, k := range []string{"src", "href"} {
		if s, ok := props[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// paintImage draws a decoded image into its box honoring objectFit. The image is
// centered (objectPosition default); "cover"/"none" that overflow are clipped to
// the box.
func paintImage(c *pdf.Content, box *layout.Box, pageH float64, ctx stylesheet.Context) {
	im := box.Image
	if im == nil || im.Spec == nil {
		return
	}
	f := box.Frame
	dw, dh, ox, oy, overflow := fitImage(im.ObjectFit, f.W, f.H, float64(im.Spec.Width), float64(im.Spec.Height))
	// Box bottom-left in PDF space; the fitted rect's bottom-left offsets from it.
	x := f.X + ox
	y := pageH - (f.Y + oy + dh)
	tl, tr, br, bl, rounded := cornerRadii(box, ctx)
	// Clip when the draw overflows the box (cover/none) or the corners are rounded.
	if overflow || rounded {
		bx, by := f.X, pageH-(f.Y+f.H)
		c.Save()
		if rounded {
			roundedRectPath(c, bx, by, f.W, f.H, tl, tr, br, bl)
		} else {
			c.Rect(bx, by, f.W, f.H)
		}
		c.Clip().EndPath()
		c.DrawImage(im.Spec, x, y, dw, dh)
		c.Restore()
		return
	}
	c.DrawImage(im.Spec, x, y, dw, dh)
}

// fitImage computes the draw size (dw,dh) and centered top-left offset (ox,oy)
// within a bw×bh box for an iw×ih image under the given objectFit, plus whether
// the draw may exceed the box (needing a clip). Unknown/empty fit means "fill".
func fitImage(fit string, bw, bh, iw, ih float64) (dw, dh, ox, oy float64, clip bool) {
	if iw <= 0 || ih <= 0 || bw <= 0 || bh <= 0 {
		return bw, bh, 0, 0, false
	}
	const eps = 0.01
	switch fit {
	case "contain":
		s := math.Min(bw/iw, bh/ih)
		dw, dh = iw*s, ih*s
	case "cover":
		s := math.Max(bw/iw, bh/ih)
		dw, dh = iw*s, ih*s
		clip = dw > bw+eps || dh > bh+eps
	case "none":
		dw, dh = iw, ih
		clip = dw > bw+eps || dh > bh+eps
	case "scale-down", "scaleDown":
		s := math.Min(1, math.Min(bw/iw, bh/ih))
		dw, dh = iw*s, ih*s
	default: // "fill" and anything unrecognized
		return bw, bh, 0, 0, false
	}
	return dw, dh, (bw - dw) / 2, (bh - dh) / 2, clip
}

// paintText draws a Text box's content at its baseline, inset by the box's own
// padding and border. Only standard-14 fonts are painted for now; embedded
// fonts (BaseFont == "") are skipped.
func paintText(c *pdf.Content, box *layout.Box, pageH float64, ctx stylesheet.Context) {
	t := box.Text
	if t == nil || len(t.Lines) == 0 || (t.BaseFont == "" && t.EmbeddedFont == nil) {
		return
	}
	col := stylesheet.Color{A: 1} // default black
	if t.Color != "" {
		if parsed, err := stylesheet.ParseColor(t.Color); err == nil {
			col = parsed
		}
	}
	r, g, b := col.RGB()

	// Line-width measurement for alignment/justify: an embedded font uses its own
	// WinAnsi advance widths; a standard font uses AFM metrics.
	var stringWidth func(string) float64
	if t.EmbeddedFont != nil {
		ef := t.EmbeddedFont
		stringWidth = func(s string) float64 { return embeddedWidth(ef, s, t.Size) }
	} else {
		metrics, err := afm.Load(t.BaseFont)
		if err != nil {
			return
		}
		stringWidth = func(s string) float64 { return metrics.StringWidth(s, t.Size) }
	}

	insetLeft := lengthPt(box.Style, "borderLeftWidth", ctx) + lengthPt(box.Style, "paddingLeft", ctx)
	insetRight := lengthPt(box.Style, "borderRightWidth", ctx) + lengthPt(box.Style, "paddingRight", ctx)
	insetTop := lengthPt(box.Style, "borderTopWidth", ctx) + lengthPt(box.Style, "paddingTop", ctx)
	contentW := box.Frame.W - insetLeft - insetRight
	x0 := box.Frame.X + insetLeft
	top := box.Frame.Y + insetTop
	align := str(box.Style["textAlign"])

	c.Save().FillRGB(r, g, b).BeginText()
	if t.EmbeddedFont != nil {
		c.SetEmbeddedFont(t.EmbeddedFont, t.Size)
	} else {
		c.SetFont(t.BaseFont, t.Size)
	}
	for i, line := range t.Lines {
		lineW := stringWidth(line)
		x := x0
		wordSpace := 0.0
		switch align {
		case "right":
			x = x0 + (contentW - lineW)
		case "center":
			x = x0 + (contentW-lineW)/2
		case "justify":
			// Justify all but the last line by widening inter-word gaps.
			if gaps := strings.Count(line, " "); i < len(t.Lines)-1 && gaps > 0 && contentW > lineW {
				wordSpace = (contentW - lineW) / float64(gaps)
			}
		}
		baseline := pageH - (top + float64(i)*t.LineHeight + t.Ascent)
		c.WordSpacing(wordSpace).TextMatrix(1, 0, 0, 1, x, baseline).ShowText(line)
	}
	c.EndText().Restore()
}

// embeddedWidth measures a string's advance in an embedded font from its WinAnsi
// advance widths (stored in 1000-unit text space).
func embeddedWidth(ef *pdf.EmbeddedFont, s string, size float64) float64 {
	total := 0.0
	for _, code := range afm.WinAnsiEncode(s) {
		if int(code) < len(ef.Widths) {
			total += float64(ef.Widths[code])
		}
	}
	return total / 1000 * size
}

func paintBackground(c *pdf.Content, box *layout.Box, pageH float64, ctx stylesheet.Context) {
	v := str(box.Style["backgroundColor"])
	if v == "" {
		return
	}
	col, err := stylesheet.ParseColor(v)
	if err != nil || col.A == 0 {
		return
	}
	f := box.Frame
	x, y := f.X, pageH-f.Y-f.H
	if tl, tr, br, bl, any := cornerRadii(box, ctx); any {
		r, g, b := col.RGB()
		c.Save().FillRGB(r, g, b)
		roundedRectPath(c, x, y, f.W, f.H, tl, tr, br, bl)
		c.Fill().Restore()
		return
	}
	fillRect(c, x, y, f.W, f.H, col)
}

func paintBorders(c *pdf.Content, box *layout.Box, pageH float64, ctx stylesheet.Context) {
	f := box.Frame

	// Rounded + uniform border: stroke a rounded path centered in the border.
	// Non-uniform borders with a radius fall through to square side strips.
	if tl, tr, br, bl, any := cornerRadii(box, ctx); any {
		if bw, col, ok := uniformBorder(box, ctx); ok {
			r, g, b := col.RGB()
			hw := bw / 2
			c.Save().StrokeRGB(r, g, b).LineWidth(bw)
			roundedRectPath(c, f.X+hw, pageH-(f.Y+f.H)+hw, f.W-bw, f.H-bw,
				math.Max(tl-hw, 0), math.Max(tr-hw, 0), math.Max(br-hw, 0), math.Max(bl-hw, 0))
			c.Stroke().Restore()
			return
		}
	}

	top := lengthPt(box.Style, "borderTopWidth", ctx)
	right := lengthPt(box.Style, "borderRightWidth", ctx)
	bottom := lengthPt(box.Style, "borderBottomWidth", ctx)
	left := lengthPt(box.Style, "borderLeftWidth", ctx)

	// Each side is a filled strip. Corners overlap; per-side colors resolve to
	// whichever side paints last there (miter joins are a later refinement).
	if top > 0 {
		fillRect(c, f.X, pageH-f.Y-top, f.W, top, borderColor(box, "Top"))
	}
	if bottom > 0 {
		fillRect(c, f.X, pageH-(f.Y+f.H), f.W, bottom, borderColor(box, "Bottom"))
	}
	if left > 0 {
		fillRect(c, f.X, pageH-(f.Y+f.H), left, f.H, borderColor(box, "Left"))
	}
	if right > 0 {
		fillRect(c, f.X+f.W-right, pageH-(f.Y+f.H), right, f.H, borderColor(box, "Right"))
	}
}

func fillRect(c *pdf.Content, x, y, w, h float64, col stylesheet.Color) {
	if w <= 0 || h <= 0 || col.A == 0 {
		return
	}
	r, g, b := col.RGB()
	c.Save().FillRGB(r, g, b).Rect(x, y, w, h).Fill().Restore()
}

// kappa is the bezier control-point ratio approximating a quarter circle.
const kappa = 0.5522847498307936

// cornerRadius resolves one border radius to points. Percent radii resolve
// against minDim (so borderRadius:"50%" on a square yields a circle); lengths use
// the normal unit resolution.
func cornerRadius(style map[string]any, key string, ctx stylesheet.Context, minDim float64) float64 {
	if v, ok := style[key]; ok {
		if s, ok := v.(string); ok && strings.HasSuffix(s, "%") {
			if f, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64); err == nil {
				return f / 100 * minDim
			}
		}
	}
	return lengthPt(style, key, ctx)
}

// cornerRadii returns the four corner radii (top-left, top-right, bottom-right,
// bottom-left) in points and whether any is positive.
func cornerRadii(box *layout.Box, ctx stylesheet.Context) (tl, tr, br, bl float64, any bool) {
	minDim := math.Min(box.Frame.W, box.Frame.H)
	tl = cornerRadius(box.Style, "borderTopLeftRadius", ctx, minDim)
	tr = cornerRadius(box.Style, "borderTopRightRadius", ctx, minDim)
	br = cornerRadius(box.Style, "borderBottomRightRadius", ctx, minDim)
	bl = cornerRadius(box.Style, "borderBottomLeftRadius", ctx, minDim)
	return tl, tr, br, bl, tl > 0 || tr > 0 || br > 0 || bl > 0
}

// clampRadii scales the radii down so adjacent corners never overlap on an edge.
func clampRadii(w, h, tl, tr, br, bl float64) (float64, float64, float64, float64) {
	for _, r := range []*float64{&tl, &tr, &br, &bl} {
		if *r < 0 {
			*r = 0
		}
	}
	s := 1.0
	clamp := func(sum, edge float64) {
		if sum > 0 && edge/sum < s {
			s = edge / sum
		}
	}
	clamp(tl+tr, w) // top
	clamp(bl+br, w) // bottom
	clamp(tl+bl, h) // left
	clamp(tr+br, h) // right
	if s < 1 {
		tl, tr, br, bl = tl*s, tr*s, br*s, bl*s
	}
	return tl, tr, br, bl
}

// roundedRectPath emits a closed rounded-rectangle path with (x,y) the PDF-space
// bottom-left corner and per-visual-corner radii. It only builds the path; the
// caller fills, strokes, or clips.
func roundedRectPath(c *pdf.Content, x, y, w, h, tl, tr, br, bl float64) {
	tl, tr, br, bl = clampRadii(w, h, tl, tr, br, bl)
	k := kappa
	c.MoveTo(x+tl, y+h)
	c.LineTo(x+w-tr, y+h)
	c.CurveTo(x+w-tr+k*tr, y+h, x+w, y+h-tr+k*tr, x+w, y+h-tr)
	c.LineTo(x+w, y+br)
	c.CurveTo(x+w, y+br-k*br, x+w-br+k*br, y, x+w-br, y)
	c.LineTo(x+bl, y)
	c.CurveTo(x+bl-k*bl, y, x, y+bl-k*bl, x, y+bl)
	c.LineTo(x, y+h-tl)
	c.CurveTo(x, y+h-tl+k*tl, x+tl-k*tl, y+h, x+tl, y+h)
	c.ClosePath()
}

// uniformBorder reports a single border width and color when all four sides
// share them (width > 0) — the case a rounded border can stroke as one path.
func uniformBorder(box *layout.Box, ctx stylesheet.Context) (width float64, col stylesheet.Color, ok bool) {
	t := lengthPt(box.Style, "borderTopWidth", ctx)
	r := lengthPt(box.Style, "borderRightWidth", ctx)
	b := lengthPt(box.Style, "borderBottomWidth", ctx)
	l := lengthPt(box.Style, "borderLeftWidth", ctx)
	if t <= 0 || t != r || t != b || t != l {
		return 0, stylesheet.Color{}, false
	}
	ct := borderColor(box, "Top")
	if ct != borderColor(box, "Right") || ct != borderColor(box, "Bottom") || ct != borderColor(box, "Left") {
		return 0, stylesheet.Color{}, false
	}
	return t, ct, true
}

// borderColor returns a side's border color, falling back to the element's
// current color, then black.
func borderColor(box *layout.Box, side string) stylesheet.Color {
	if v := str(box.Style["border"+side+"Color"]); v != "" {
		if col, err := stylesheet.ParseColor(v); err == nil {
			return col
		}
	}
	if v := str(box.Style["color"]); v != "" {
		if col, err := stylesheet.ParseColor(v); err == nil {
			return col
		}
	}
	return stylesheet.Color{A: 1} // black
}

func lengthPt(style map[string]any, key string, ctx stylesheet.Context) float64 {
	v, ok := style[key]
	if !ok {
		return 0
	}
	val, err := stylesheet.ParseValue(v)
	if err != nil || val.IsAuto() || val.Unit == stylesheet.UnitPercent {
		return 0
	}
	return val.Resolve(ctx, 0)
}

func str(v any) string {
	s, _ := v.(string)
	return s
}
