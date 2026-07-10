package fontstore

import (
	"strings"

	"github.com/go-text/typesetting/font"

	"github.com/swish/waffle/internal/pdf"
	"github.com/swish/waffle/internal/pdf/afm"
)

// Descriptor holds the metrics a PDF FontDescriptor needs for an embedded font,
// in 1000-unit text space (font units scaled by 1000/upem).
type Descriptor struct {
	Ascent, Descent, CapHeight float64
	BBox                       [4]float64 // xMin, yMin, xMax, yMax
	ItalicAngle                float64
	Flags                      int
	StemV                      float64
}

// PDF FontDescriptor flag bits (PDF 32000-1 Table 121).
const (
	flagFixedPitch  = 1 << 0
	flagNonsymbolic = 1 << 5
	flagItalic      = 1 << 6
)

func (f *Face) scale() float64 {
	if f.upem == 0 {
		return 0
	}
	return 1000.0 / f.upem
}

// Descriptor computes the FontDescriptor metrics for embedding this face.
func (f *Face) Descriptor() Descriptor {
	s := f.scale()
	d := Descriptor{Flags: flagNonsymbolic, StemV: 80}
	if ext, ok := f.parsed.FontHExtents(); ok {
		d.Ascent = float64(ext.Ascender) * s
		d.Descent = float64(ext.Descender) * s
	}
	if ch := f.parsed.LineMetric(font.CapHeight); ch != 0 {
		d.CapHeight = float64(ch) * s
	} else {
		d.CapHeight = d.Ascent * 0.7
	}
	d.BBox = f.bbox(s)
	if f.Style == StyleItalic {
		d.Flags |= flagItalic
		d.ItalicAngle = -12
	}
	if f.isFixedPitch() {
		d.Flags |= flagFixedPitch
	}
	if f.Weight >= 600 {
		d.StemV = 120
	}
	return d
}

// bbox returns a conservative font bounding box [0, descent, maxAdvance, ascent]
// in 1000-unit space — loose but valid, which is all a FontDescriptor needs.
func (f *Face) bbox(s float64) [4]float64 {
	asc, desc := 800.0, -200.0
	if ext, ok := f.parsed.FontHExtents(); ok {
		asc = float64(ext.Ascender) * s
		desc = float64(ext.Descender) * s
	}
	maxAdv := 0.0
	for c := 0; c < 256; c++ {
		r := afm.WinAnsiRune(byte(c))
		if r == 0 {
			continue
		}
		if gid, ok := f.parsed.NominalGlyph(r); ok {
			if w := float64(f.parsed.HorizontalAdvance(gid)) * s; w > maxAdv {
				maxAdv = w
			}
		}
	}
	if maxAdv == 0 {
		maxAdv = 1000
	}
	return [4]float64{0, desc, maxAdv, asc}
}

// WinAnsiWidths returns advance widths in 1000-unit space for WinAnsi codes
// 0..255 (0 for unmapped codes), for the PDF simple-font /Widths array.
func (f *Face) WinAnsiWidths() []int {
	s := f.scale()
	w := make([]int, 256)
	for c := 0; c < 256; c++ {
		r := afm.WinAnsiRune(byte(c))
		if r == 0 {
			continue
		}
		if gid, ok := f.parsed.NominalGlyph(r); ok {
			w[c] = int(float64(f.parsed.HorizontalAdvance(gid))*s + 0.5)
		}
	}
	return w
}

// FontName returns a PDF-safe /BaseFont name derived from the family and
// weight/style. Embedders may prefix a subset tag (e.g. "ABCDEF+").
func (f *Face) FontName() string {
	name := strings.Map(func(r rune) rune {
		if strings.ContainsRune(" \t()<>[]{}/%#", r) {
			return -1
		}
		return r
	}, f.Family)
	if name == "" {
		name = "Font"
	}
	switch {
	case f.Weight >= 600 && f.Style == StyleItalic:
		return name + "-BoldItalic"
	case f.Weight >= 600:
		return name + "-Bold"
	case f.Style == StyleItalic:
		return name + "-Italic"
	}
	return name
}

// EmbeddedFont returns the pdf.EmbeddedFont for this face (program bytes +
// descriptor + WinAnsi widths), ready to embed as a simple TrueType PDF font.
// The value is built once and cached: returning the same pointer on every call
// lets the PDF writer dedupe the font program to a single FontFile2 stream no
// matter how many text boxes use the face.
func (f *Face) EmbeddedFont() *pdf.EmbeddedFont {
	f.embedOnce.Do(func() {
		d := f.Descriptor()
		f.embedded = &pdf.EmbeddedFont{
			Name:    f.FontName(),
			Program: f.data,
			Descriptor: pdf.FontDescriptor{
				Ascent:      d.Ascent,
				Descent:     d.Descent,
				CapHeight:   d.CapHeight,
				BBox:        d.BBox,
				ItalicAngle: d.ItalicAngle,
				Flags:       d.Flags,
				StemV:       d.StemV,
			},
			Widths: f.WinAnsiWidths(),
		}
	})
	return f.embedded
}

// isFixedPitch heuristically detects a monospace font by comparing the advances
// of two glyphs that differ in width in proportional fonts.
func (f *Face) isFixedPitch() bool {
	gi, oki := f.parsed.NominalGlyph('i')
	gm, okm := f.parsed.NominalGlyph('M')
	if !oki || !okm {
		return false
	}
	return f.parsed.HorizontalAdvance(gi) == f.parsed.HorizontalAdvance(gm)
}
