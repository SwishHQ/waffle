package pdf

import (
	"bytes"
	"fmt"

	"github.com/SwishHQ/waffle/internal/pdf/afm"
)

// Content builds a page content stream from PDF drawing operators. Methods are
// chainable. It also records which standard fonts were used so the page can
// build a matching /Font resource dictionary.
type Content struct {
	buf        bytes.Buffer
	fontSeq    []string        // base fonts in first-use order
	fontRes    map[string]Name // base font -> resource name (F1, F2, ...)
	imageSeq   []*ImageSpec    // images in first-use order
	imageRes   map[*ImageSpec]Name
	links      []LinkAnnotation // hyperlink annotations for this page
	notes      []NoteAnnotation // text-note annotations for this page
	fields     []FormField      // AcroForm field widgets for this page
	alphaSeq   []float64        // constant-alpha values in first-use order
	alphaRes   map[float64]Name // alpha -> ExtGState resource name (GS1, GS2, ...)
	embSeq     []*EmbeddedFont  // embedded TrueType fonts in first-use order
	embRes     map[*EmbeddedFont]Name
	shadingSeq []*Shading // gradient shadings in first-use order
	shadingRes map[*Shading]Name
}

// LinkAnnotation is a clickable URI link over a rectangle, in page (default user)
// space — bottom-left origin, x0<x1, y0<y1. It is a page annotation, not a
// content-stream op, so it is unaffected by the content CTM.
type LinkAnnotation struct {
	X0, Y0, X1, Y1 float64
	URI            string
}

// AddLink records a hyperlink annotation over the given page-space rectangle.
func (c *Content) AddLink(x0, y0, x1, y1 float64, uri string) {
	c.links = append(c.links, LinkAnnotation{X0: x0, Y0: y0, X1: x1, Y1: y1, URI: uri})
}

// Links returns the hyperlink annotations recorded for this page.
func (c *Content) Links() []LinkAnnotation { return c.links }

// NoteAnnotation is a text (sticky-note) annotation anchored at a page-space
// point (its icon's top-left), carrying popup text. Coordinates are in default
// user space (bottom-left origin), so it is unaffected by the content CTM.
type NoteAnnotation struct {
	X, Y float64
	Text string
}

// AddNote records a text-note annotation anchored at the given page-space point.
func (c *Content) AddNote(x, y float64, text string) {
	c.notes = append(c.notes, NoteAnnotation{X: x, Y: y, Text: text})
}

// Notes returns the text-note annotations recorded for this page.
func (c *Content) Notes() []NoteAnnotation { return c.notes }

// SetAlpha sets the constant alpha for fills and strokes (ca/CA) via an
// ExtGState resource, recording the alpha so the page can build /ExtGState.
// Values are clamped to [0,1]. Requires PDF 1.4+.
func (c *Content) SetAlpha(a float64) *Content {
	if a < 0 {
		a = 0
	} else if a > 1 {
		a = 1
	}
	res, ok := c.alphaRes[a]
	if !ok {
		if c.alphaRes == nil {
			c.alphaRes = make(map[float64]Name)
		}
		res = Name(fmt.Sprintf("GS%d", len(c.alphaSeq)+1))
		c.alphaRes[a] = res
		c.alphaSeq = append(c.alphaSeq, a)
	}
	res.encode(&c.buf)
	return c.w(" gs\n")
}

// Alphas returns the alpha values used, in first-use order.
func (c *Content) Alphas() []float64 { return c.alphaSeq }

// AlphaResource returns the resource name assigned to an alpha value.
func (c *Content) AlphaResource(a float64) (Name, bool) {
	n, ok := c.alphaRes[a]
	return n, ok
}

// NewContent returns an empty content-stream builder.
func NewContent() *Content {
	return &Content{fontRes: make(map[string]Name)}
}

// Bytes returns the raw (uncompressed) content-stream bytes.
func (c *Content) Bytes() []byte { return c.buf.Bytes() }

// Fonts returns the base fonts used, in first-use order.
func (c *Content) Fonts() []string { return c.fontSeq }

// FontResource returns the resource name assigned to a base font (valid after
// SetFont has been called for it).
func (c *Content) FontResource(baseFont string) (Name, bool) {
	n, ok := c.fontRes[baseFont]
	return n, ok
}

func (c *Content) w(format string, args ...any) *Content {
	fmt.Fprintf(&c.buf, format, args...)
	return c
}

// --- graphics state ---

// Save pushes the graphics state (q).
func (c *Content) Save() *Content { c.buf.WriteString("q\n"); return c }

// Restore pops the graphics state (Q).
func (c *Content) Restore() *Content { c.buf.WriteString("Q\n"); return c }

// Transform concatenates the matrix [a b c d e f] to the CTM (cm).
func (c *Content) Transform(a, b, cc, d, e, f float64) *Content {
	return c.w("%s %s %s %s %s %s cm\n", num(a), num(b), num(cc), num(d), num(e), num(f))
}

// LineWidth sets the stroke line width (w).
func (c *Content) LineWidth(v float64) *Content { return c.w("%s w\n", num(v)) }

// LineCap sets the line cap style (J): 0 butt, 1 round, 2 square.
func (c *Content) LineCap(style int) *Content { return c.w("%d J\n", style) }

// LineJoin sets the line join style (j): 0 miter, 1 round, 2 bevel.
func (c *Content) LineJoin(style int) *Content { return c.w("%d j\n", style) }

// Dash sets the line dash pattern (d).
func (c *Content) Dash(phase float64, pattern ...float64) *Content {
	c.buf.WriteByte('[')
	for i, p := range pattern {
		if i > 0 {
			c.buf.WriteByte(' ')
		}
		c.buf.WriteString(num(p))
	}
	return c.w("] %s d\n", num(phase))
}

// --- path construction ---

// MoveTo begins a new subpath (m).
func (c *Content) MoveTo(x, y float64) *Content { return c.w("%s %s m\n", num(x), num(y)) }

// LineTo appends a straight segment (l).
func (c *Content) LineTo(x, y float64) *Content { return c.w("%s %s l\n", num(x), num(y)) }

// CurveTo appends a cubic Bézier segment (c).
func (c *Content) CurveTo(x1, y1, x2, y2, x3, y3 float64) *Content {
	return c.w("%s %s %s %s %s %s c\n", num(x1), num(y1), num(x2), num(y2), num(x3), num(y3))
}

// Rect appends a rectangle subpath (re).
func (c *Content) Rect(x, y, wid, hgt float64) *Content {
	return c.w("%s %s %s %s re\n", num(x), num(y), num(wid), num(hgt))
}

// ClosePath closes the current subpath (h).
func (c *Content) ClosePath() *Content { c.buf.WriteString("h\n"); return c }

// --- path painting ---

// Stroke strokes the path (S).
func (c *Content) Stroke() *Content { c.buf.WriteString("S\n"); return c }

// Fill fills the path with the nonzero winding rule (f).
func (c *Content) Fill() *Content { c.buf.WriteString("f\n"); return c }

// FillEvenOdd fills the path with the even-odd rule (f*).
func (c *Content) FillEvenOdd() *Content { c.buf.WriteString("f*\n"); return c }

// FillStroke fills then strokes the path (B).
func (c *Content) FillStroke() *Content { c.buf.WriteString("B\n"); return c }

// EndPath ends the path with no fill or stroke (n), e.g. after a clip.
func (c *Content) EndPath() *Content { c.buf.WriteString("n\n"); return c }

// Clip intersects the clip path using the nonzero rule (W).
func (c *Content) Clip() *Content { c.buf.WriteString("W\n"); return c }

// ClipEvenOdd intersects the clip path using the even-odd rule (W*).
func (c *Content) ClipEvenOdd() *Content { c.buf.WriteString("W*\n"); return c }

// --- color ---

// FillRGB sets the nonstroking color in DeviceRGB (rg). Components are 0..1.
func (c *Content) FillRGB(r, g, b float64) *Content {
	return c.w("%s %s %s rg\n", num(r), num(g), num(b))
}

// StrokeRGB sets the stroking color in DeviceRGB (RG). Components are 0..1.
func (c *Content) StrokeRGB(r, g, b float64) *Content {
	return c.w("%s %s %s RG\n", num(r), num(g), num(b))
}

// FillGray sets the nonstroking gray level (g). 0 is black, 1 is white.
func (c *Content) FillGray(v float64) *Content { return c.w("%s g\n", num(v)) }

// StrokeGray sets the stroking gray level (G).
func (c *Content) StrokeGray(v float64) *Content { return c.w("%s G\n", num(v)) }

// --- text ---

// BeginText begins a text object (BT).
func (c *Content) BeginText() *Content { c.buf.WriteString("BT\n"); return c }

// EndText ends a text object (ET).
func (c *Content) EndText() *Content { c.buf.WriteString("ET\n"); return c }

// SetFont selects a standard font and size (Tf). The base font is registered so
// the page can wire the matching resource name to a font object.
func (c *Content) SetFont(baseFont string, size float64) *Content {
	res, ok := c.fontRes[baseFont]
	if !ok {
		res = Name(fmt.Sprintf("F%d", len(c.fontSeq)+1))
		c.fontRes[baseFont] = res
		c.fontSeq = append(c.fontSeq, baseFont)
	}
	res.encode(&c.buf)
	return c.w(" %s Tf\n", num(size))
}

// TextPosition moves to the next line offset by (tx, ty) (Td).
func (c *Content) TextPosition(x, y float64) *Content {
	return c.w("%s %s Td\n", num(x), num(y))
}

// TextMatrix sets the text matrix (Tm).
func (c *Content) TextMatrix(a, b, cc, d, e, f float64) *Content {
	return c.w("%s %s %s %s %s %s Tm\n", num(a), num(b), num(cc), num(d), num(e), num(f))
}

// Leading sets the text leading (TL).
func (c *Content) Leading(v float64) *Content { return c.w("%s TL\n", num(v)) }

// NextLine moves to the start of the next line (T*).
func (c *Content) NextLine() *Content { c.buf.WriteString("T*\n"); return c }

// CharSpacing sets the character spacing (Tc).
func (c *Content) CharSpacing(v float64) *Content { return c.w("%s Tc\n", num(v)) }

// WordSpacing sets the word spacing (Tw).
func (c *Content) WordSpacing(v float64) *Content { return c.w("%s Tw\n", num(v)) }

// TextRise sets the text rise, used for super/subscript (Ts).
func (c *Content) TextRise(v float64) *Content { return c.w("%s Ts\n", num(v)) }

// HorizScale sets horizontal text scaling as a percentage (Tz).
func (c *Content) HorizScale(pct float64) *Content { return c.w("%s Tz\n", num(pct)) }

// ShowText shows a string, encoding it to WinAnsiEncoding (Tj). Runes not
// representable in WinAnsi are shown as '?'.
func (c *Content) ShowText(s string) *Content {
	return c.ShowTextRaw(afm.WinAnsiEncode(s))
}

// ShowTextRaw shows already-encoded bytes as a literal string (Tj).
func (c *Content) ShowTextRaw(enc []byte) *Content {
	LiteralString(string(enc)).encode(&c.buf)
	c.buf.WriteString(" Tj\n")
	return c
}

func num(f float64) string { return formatReal(f) }
