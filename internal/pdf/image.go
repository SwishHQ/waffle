package pdf

import "fmt"

// ImageSpec describes an image ready to embed as an XObject: its dimensions,
// color space, and already-filtered stream data (plus an optional alpha SMask).
type ImageSpec struct {
	Width, Height    int
	ColorSpace       string // "DeviceRGB", "DeviceGray", "DeviceCMYK"
	BitsPerComponent int
	Filter           string // "DCTDecode" or "FlateDecode"
	Data             []byte // filtered stream bytes
	SMask            []byte // optional 8-bit gray alpha, FlateDecode-compressed
}

// DrawImage draws an image, scaled to w×h at (x, y) in the current coordinate
// system (bottom-left origin). Images are drawn from a unit square, so the CTM
// maps them to the target rectangle.
func (c *Content) DrawImage(spec *ImageSpec, x, y, w, h float64) *Content {
	res, ok := c.imageRes[spec]
	if !ok {
		res = Name(fmt.Sprintf("Im%d", len(c.imageSeq)+1))
		if c.imageRes == nil {
			c.imageRes = make(map[*ImageSpec]Name)
		}
		c.imageRes[spec] = res
		c.imageSeq = append(c.imageSeq, spec)
	}
	c.buf.WriteString("q\n")
	fmt.Fprintf(&c.buf, "%s 0 0 %s %s %s cm\n", num(w), num(h), num(x), num(y))
	res.encode(&c.buf)
	c.buf.WriteString(" Do\nQ\n")
	return c
}

// Images returns the images used, in first-use order.
func (c *Content) Images() []*ImageSpec { return c.imageSeq }

// ImageResource returns the resource name assigned to an image.
func (c *Content) ImageResource(spec *ImageSpec) (Name, bool) {
	n, ok := c.imageRes[spec]
	return n, ok
}

// imageRef creates (once per spec) the image XObject and its optional SMask,
// returning the XObject reference.
func (d *Document) imageRef(spec *ImageSpec) Reference {
	if r, ok := d.imageObjs[spec]; ok {
		return r
	}
	dict := Dict{
		Name("Type"):             Name("XObject"),
		Name("Subtype"):          Name("Image"),
		Name("Width"):            Integer(spec.Width),
		Name("Height"):           Integer(spec.Height),
		Name("ColorSpace"):       Name(spec.ColorSpace),
		Name("BitsPerComponent"): Integer(spec.BitsPerComponent),
		Name("Filter"):           Name(spec.Filter),
	}
	if len(spec.SMask) > 0 {
		smask := &Stream{
			Dict: Dict{
				Name("Type"):             Name("XObject"),
				Name("Subtype"):          Name("Image"),
				Name("Width"):            Integer(spec.Width),
				Name("Height"):           Integer(spec.Height),
				Name("ColorSpace"):       Name("DeviceGray"),
				Name("BitsPerComponent"): Integer(8),
				Name("Filter"):           Name("FlateDecode"),
			},
			Data: spec.SMask,
		}
		dict[Name("SMask")] = d.w.Add(smask)
	}
	r := d.w.Add(&Stream{Dict: dict, Data: spec.Data})
	d.imageObjs[spec] = r
	return r
}
