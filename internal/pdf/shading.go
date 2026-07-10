package pdf

import (
	"fmt"
	"sort"
)

// ShadingStop is a gradient color stop: Offset in [0,1], RGB components in [0,1].
type ShadingStop struct {
	Offset, R, G, B float64
}

// Shading describes an axial (linear) or radial gradient in DeviceRGB, expressed
// in the current user space — it is painted with the `sh` operator, which honors
// the current CTM and clip (so it needs no pattern matrix). Coords are
// [x0 y0 x1 y1] for axial and [cx0 cy0 r0 cx1 cy1 r1] for radial. Compared by
// pointer identity; reuse the same *Shading per gradient instance.
type Shading struct {
	Radial bool
	Coords []float64
	Stops  []ShadingStop
}

// Shade registers a shading and emits `sh` to paint it, subject to the current
// clip path. Wrap in Save → path → Clip → EndPath → Shade → Restore to confine
// the gradient to a shape.
func (c *Content) Shade(sh *Shading) *Content {
	res, ok := c.shadingRes[sh]
	if !ok {
		if c.shadingRes == nil {
			c.shadingRes = make(map[*Shading]Name)
		}
		res = Name(fmt.Sprintf("Sh%d", len(c.shadingSeq)+1))
		c.shadingRes[sh] = res
		c.shadingSeq = append(c.shadingSeq, sh)
	}
	res.encode(&c.buf)
	return c.w(" sh\n")
}

// Shadings returns the shadings used, in first-use order.
func (c *Content) Shadings() []*Shading { return c.shadingSeq }

// ShadingResource returns the resource name assigned to a shading.
func (c *Content) ShadingResource(sh *Shading) (Name, bool) {
	n, ok := c.shadingRes[sh]
	return n, ok
}

// shadingRef builds (once, cached by pointer) the PDF shading dictionary and its
// color-interpolation function.
func (d *Document) shadingRef(sh *Shading) Reference {
	if r, ok := d.shadingObjs[sh]; ok {
		return r
	}
	coords := make(Array, len(sh.Coords))
	for i, v := range sh.Coords {
		coords[i] = Real(v)
	}
	shadingType := 2
	if sh.Radial {
		shadingType = 3
	}
	r := d.w.Add(Dict{
		Name("ShadingType"): Integer(shadingType),
		Name("ColorSpace"):  Name("DeviceRGB"),
		Name("Coords"):      coords,
		Name("Function"):    d.shadingFunction(sh.Stops),
		Name("Extend"):      Array{Boolean(true), Boolean(true)},
	})
	d.shadingObjs[sh] = r
	return r
}

// shadingFunction builds the color function over the stops: a single Type 2
// exponential for two stops, or a Type 3 stitching function over per-segment
// Type 2 functions for three or more.
func (d *Document) shadingFunction(stops []ShadingStop) Object {
	s := normalizeStops(stops)
	segs := make(Array, 0, len(s)-1)
	var bounds, encode Array
	for i := 0; i < len(s)-1; i++ {
		a, b := s[i], s[i+1]
		segs = append(segs, d.w.Add(Dict{
			Name("FunctionType"): Integer(2),
			Name("Domain"):       Array{Integer(0), Integer(1)},
			Name("C0"):           Array{Real(a.R), Real(a.G), Real(a.B)},
			Name("C1"):           Array{Real(b.R), Real(b.G), Real(b.B)},
			Name("N"):            Integer(1),
		}))
		if i > 0 {
			bounds = append(bounds, Real(s[i].Offset))
		}
		encode = append(encode, Integer(0), Integer(1))
	}
	if len(segs) == 1 {
		return segs[0]
	}
	return d.w.Add(Dict{
		Name("FunctionType"): Integer(3),
		Name("Domain"):       Array{Integer(0), Integer(1)},
		Name("Functions"):    segs,
		Name("Bounds"):       bounds,
		Name("Encode"):       encode,
	})
}

// normalizeStops sorts stops by offset, clamps offsets to [0,1], and guarantees
// at least two stops with the ends pinned to 0 and 1 so the function's domain is
// fully covered and Bounds stay interior.
func normalizeStops(stops []ShadingStop) []ShadingStop {
	out := append([]ShadingStop(nil), stops...)
	for i := range out {
		if out[i].Offset < 0 {
			out[i].Offset = 0
		} else if out[i].Offset > 1 {
			out[i].Offset = 1
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Offset < out[j].Offset })
	switch len(out) {
	case 0:
		return []ShadingStop{{0, 0, 0, 0}, {1, 0, 0, 0}}
	case 1:
		s := out[0]
		return []ShadingStop{{0, s.R, s.G, s.B}, {1, s.R, s.G, s.B}}
	}
	out[0].Offset = 0
	out[len(out)-1].Offset = 1
	return out
}
