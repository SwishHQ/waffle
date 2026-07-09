package pdf

import (
	"fmt"
	"math"
)

// FontDescriptor holds embedded-font metrics in 1000-unit text space.
type FontDescriptor struct {
	Ascent, Descent, CapHeight float64
	BBox                       [4]float64 // xMin, yMin, xMax, yMax
	ItalicAngle                float64
	Flags                      int
	StemV                      float64
}

// EmbeddedFont is a TrueType font program embedded as a simple (WinAnsi) PDF
// font. Widths is indexed by WinAnsi code (0..255), in 1000-unit advances. It is
// compared by pointer identity, so reuse the same *EmbeddedFont per face.
type EmbeddedFont struct {
	Name       string // PDF /BaseFont name (unique per face)
	Program    []byte // raw TrueType (sfnt) bytes
	Descriptor FontDescriptor
	Widths     []int
}

// SetEmbeddedFont selects an embedded TrueType font and size (Tf), registering
// it so the page can wire a matching /Font resource.
func (c *Content) SetEmbeddedFont(ef *EmbeddedFont, size float64) *Content {
	res, ok := c.embRes[ef]
	if !ok {
		if c.embRes == nil {
			c.embRes = make(map[*EmbeddedFont]Name)
		}
		res = Name(fmt.Sprintf("TT%d", len(c.embSeq)+1))
		c.embRes[ef] = res
		c.embSeq = append(c.embSeq, ef)
	}
	res.encode(&c.buf)
	return c.w(" %s Tf\n", num(size))
}

// EmbeddedFonts returns the embedded fonts used, in first-use order.
func (c *Content) EmbeddedFonts() []*EmbeddedFont { return c.embSeq }

// EmbeddedFontResource returns the resource name assigned to an embedded font.
func (c *Content) EmbeddedFontResource(ef *EmbeddedFont) (Name, bool) {
	n, ok := c.embRes[ef]
	return n, ok
}

// embFontRef builds (once, cached by pointer) the PDF font object for an
// embedded TrueType font: the simple font dict, its FontDescriptor, and the
// FontFile2 program stream.
func (d *Document) embFontRef(ef *EmbeddedFont) Reference {
	if r, ok := d.embFontObjs[ef]; ok {
		return r
	}
	ffRef := d.w.Add(FlateStream(Dict{Name("Length1"): Integer(len(ef.Program))}, ef.Program))

	fd := ef.Descriptor
	descRef := d.w.Add(Dict{
		Name("Type"):        Name("FontDescriptor"),
		Name("FontName"):    Name(ef.Name),
		Name("Flags"):       Integer(fd.Flags),
		Name("FontBBox"):    Array{iround(fd.BBox[0]), iround(fd.BBox[1]), iround(fd.BBox[2]), iround(fd.BBox[3])},
		Name("ItalicAngle"): iround(fd.ItalicAngle),
		Name("Ascent"):      iround(fd.Ascent),
		Name("Descent"):     iround(fd.Descent),
		Name("CapHeight"):   iround(fd.CapHeight),
		Name("StemV"):       iround(fd.StemV),
		Name("FontFile2"):   ffRef,
	})

	const first, last = 32, 255
	widths := make(Array, 0, last-first+1)
	for code := first; code <= last; code++ {
		w := 0
		if code < len(ef.Widths) {
			w = ef.Widths[code]
		}
		widths = append(widths, Integer(w))
	}

	fontRef := d.w.Add(Dict{
		Name("Type"):           Name("Font"),
		Name("Subtype"):        Name("TrueType"),
		Name("BaseFont"):       Name(ef.Name),
		Name("FirstChar"):      Integer(first),
		Name("LastChar"):       Integer(last),
		Name("Widths"):         widths,
		Name("FontDescriptor"): descRef,
		Name("Encoding"):       Name("WinAnsiEncoding"),
	})
	d.embFontObjs[ef] = fontRef
	return fontRef
}

func iround(f float64) Integer { return Integer(int(math.Round(f))) }
