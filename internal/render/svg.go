package render

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/swish/feast/internal/contract"
	"github.com/swish/feast/internal/fontstore"
	"github.com/swish/feast/internal/layout"
	"github.com/swish/feast/internal/pdf"
	"github.com/swish/feast/internal/pdf/afm"
	"github.com/swish/feast/internal/stylesheet"
	"github.com/swish/feast/internal/svgparse"
	"github.com/swish/feast/internal/tree"
)

// paintSVG draws an Svg box: it maps the viewBox onto the box frame (with the
// Y-flip into PDF space) and draws each shape and text run. Transform attributes
// and clip paths are later refinements.
func paintSVG(c *pdf.Content, box *layout.Box, pageH float64) {
	node := box.SVG
	if node == nil {
		return
	}
	f := box.Frame
	minX, minY, vbW, vbH := viewBox(node)
	if vbW <= 0 {
		vbW = f.W
	}
	if vbH <= 0 {
		vbH = f.H
	}
	if vbW <= 0 || vbH <= 0 {
		return
	}
	sx, sy := f.W/vbW, f.H/vbH

	grads := collectGradients(node)

	c.Save()
	// Map SVG user space (y-down) onto the box in PDF space (y-up).
	c.Transform(sx, 0, 0, -sy, f.X-minX*sx, (pageH-f.Y)+minY*sy)
	for _, child := range node.Children {
		drawSVGNode(c, child, grads)
	}
	c.Restore()
}

func drawSVGNode(c *pdf.Content, n *tree.Node, grads map[string]*tree.Node) {
	switch n.Type {
	case contract.TypeG:
		for _, ch := range n.Children {
			drawSVGNode(c, ch, grads)
		}
	case contract.TypePath:
		if d, ok := n.Props["d"].(string); ok {
			if p, err := svgparse.ParsePath(d); err == nil {
				emitAndPaint(c, p, n, grads)
			}
		}
	case contract.TypeRect:
		emitAndPaint(c, svgparse.Rect(numP(n, "x"), numP(n, "y"), numP(n, "width"), numP(n, "height"), numP(n, "rx"), numP(n, "ry")), n, grads)
	case contract.TypeCircle:
		emitAndPaint(c, svgparse.Circle(numP(n, "cx"), numP(n, "cy"), numP(n, "r")), n, grads)
	case contract.TypeEllipse:
		emitAndPaint(c, svgparse.Ellipse(numP(n, "cx"), numP(n, "cy"), numP(n, "rx"), numP(n, "ry")), n, grads)
	case contract.TypeLine:
		emitAndPaint(c, svgparse.Line(numP(n, "x1"), numP(n, "y1"), numP(n, "x2"), numP(n, "y2")), n, grads)
	case contract.TypePolyline:
		emitAndPaint(c, svgparse.Polyline(pointList(n)), n, grads)
	case contract.TypePolygon:
		emitAndPaint(c, svgparse.Polygon(pointList(n)), n, grads)
	case contract.TypeText, contract.TypeTspan:
		drawSVGText(c, n)
	}
}

// drawSVGText paints an SVG <Text> (or bare <Tspan>) run at its (x,y) anchor
// using a standard font. Because the SVG CTM flips Y, the text matrix carries a
// local [1 0 0 -1] flip so glyphs render upright; fontSize is in user units and
// is scaled by the viewBox mapping like any other coordinate.
func drawSVGText(c *pdf.Content, n *tree.Node) {
	s := svgText(n)
	if s == "" {
		return
	}
	fill := svgAttr(n, "fill", "black")
	if fill == "none" {
		return
	}
	col, err := stylesheet.ParseColor(fill)
	if err != nil {
		col = stylesheet.Color{A: 1}
	}

	size := numP(n, "fontSize")
	if size <= 0 {
		size = 16 // SVG default font-size
	}
	weight := 400
	if w, ok := stylesheet.ParseFontWeight(n.Props["fontWeight"]); ok {
		weight = w
	}
	base, ok := fontstore.StandardBaseFont(svgAttr(n, "fontFamily", "Helvetica"), weight, fontstore.ParseStyle(svgAttr(n, "fontStyle", "")))
	if !ok {
		base = "Helvetica"
	}

	x, y := numP(n, "x"), numP(n, "y")
	// text-anchor shifts the origin: middle centers, end right-aligns.
	if anchor := svgAttr(n, "textAnchor", "start"); anchor == "middle" || anchor == "end" {
		if m, err := afm.Load(base); err == nil {
			w := m.StringWidth(s, size)
			if anchor == "middle" {
				x -= w / 2
			} else {
				x -= w
			}
		}
	}

	r, g, b := col.RGB()
	c.Save().FillRGB(r, g, b).BeginText()
	c.SetFont(base, size)
	c.TextMatrix(1, 0, 0, -1, x, y)
	c.ShowText(s)
	c.EndText().Restore()
}

// svgText concatenates the text of all TEXT_INSTANCE descendants (including those
// inside nested <Tspan>) in document order.
func svgText(n *tree.Node) string {
	var b strings.Builder
	var walk func(*tree.Node)
	walk = func(nd *tree.Node) {
		if nd.Type == contract.TypeTextInstance {
			b.WriteString(nd.Value)
			return
		}
		for _, c := range nd.Children {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func emitAndPaint(c *pdf.Content, p svgparse.Path, n *tree.Node, grads map[string]*tree.Node) {
	if len(p) == 0 {
		return
	}
	// A gradient fill (fill="url(#id)") is painted by clipping to the shape and
	// running the shading; solid fills take the ordinary path.
	if id, ok := gradientID(svgAttr(n, "fill", "")); ok {
		if grad := grads[id]; grad != nil {
			if sh := buildShading(grad, pathBBox(p)); sh != nil {
				paintGradientShape(c, p, n, sh)
				return
			}
		}
	}
	emitPath(c, p)
	paintSVGShape(c, n)
}

// emitPath writes a path's segments as content operators.
func emitPath(c *pdf.Content, p svgparse.Path) {
	for _, s := range p {
		switch s.Op {
		case svgparse.MoveTo:
			c.MoveTo(s.Args[0], s.Args[1])
		case svgparse.LineTo:
			c.LineTo(s.Args[0], s.Args[1])
		case svgparse.CubicTo:
			c.CurveTo(s.Args[0], s.Args[1], s.Args[2], s.Args[3], s.Args[4], s.Args[5])
		case svgparse.Close:
			c.ClosePath()
		}
	}
}

// paintGradientShape clips to the shape and paints the shading (sh honors the
// current SVG CTM), then strokes on top if the shape also has a stroke.
func paintGradientShape(c *pdf.Content, p svgparse.Path, n *tree.Node, sh *pdf.Shading) {
	c.Save()
	emitPath(c, p)
	if svgAttr(n, "fillRule", "") == "evenodd" {
		c.ClipEvenOdd()
	} else {
		c.Clip()
	}
	c.EndPath().Shade(sh).Restore()

	stroke := svgAttr(n, "stroke", "none")
	if stroke != "" && stroke != "none" {
		if col, err := stylesheet.ParseColor(stroke); err == nil {
			r, g, b := col.RGB()
			c.StrokeRGB(r, g, b)
			sw := numP(n, "strokeWidth")
			if sw <= 0 {
				sw = 1
			}
			c.LineWidth(sw)
			emitPath(c, p)
			c.Stroke()
		}
	}
}

// paintSVGShape resolves fill/stroke and emits the painting operator. SVG
// defaults: fill black, stroke none.
func paintSVGShape(c *pdf.Content, n *tree.Node) {
	fill := svgAttr(n, "fill", "black")
	stroke := svgAttr(n, "stroke", "none")
	fillOn := fill != "" && fill != "none"
	strokeOn := stroke != "" && stroke != "none"

	if fillOn {
		if col, err := stylesheet.ParseColor(fill); err == nil {
			r, g, b := col.RGB()
			c.FillRGB(r, g, b)
		}
	}
	if strokeOn {
		if col, err := stylesheet.ParseColor(stroke); err == nil {
			r, g, b := col.RGB()
			c.StrokeRGB(r, g, b)
		}
		sw := numP(n, "strokeWidth")
		if sw <= 0 {
			sw = 1
		}
		c.LineWidth(sw)
	}

	evenOdd := svgAttr(n, "fillRule", "") == "evenodd"
	switch {
	case fillOn && strokeOn:
		c.FillStroke()
	case fillOn:
		if evenOdd {
			c.FillEvenOdd()
		} else {
			c.Fill()
		}
	case strokeOn:
		c.Stroke()
	default:
		c.EndPath()
	}
}

func viewBox(n *tree.Node) (minX, minY, w, h float64) {
	vb, ok := n.Props["viewBox"].(string)
	if !ok {
		return 0, 0, 0, 0
	}
	f := strings.FieldsFunc(vb, func(r rune) bool { return r == ' ' || r == ',' })
	if len(f) != 4 {
		return 0, 0, 0, 0
	}
	minX, _ = strconv.ParseFloat(f[0], 64)
	minY, _ = strconv.ParseFloat(f[1], 64)
	w, _ = strconv.ParseFloat(f[2], 64)
	h, _ = strconv.ParseFloat(f[3], 64)
	return
}

func pointList(n *tree.Node) []float64 {
	s, _ := n.Props["points"].(string)
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\n' || r == '\t' || r == '\r'
	})
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		if v, err := strconv.ParseFloat(f, 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

func numP(n *tree.Node, key string) float64 {
	switch t := n.Props[key].(type) {
	case json.Number:
		f, _ := t.Float64()
		return f
	case float64:
		return t
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	}
	return 0
}

func svgAttr(n *tree.Node, key, def string) string {
	if s, ok := n.Props[key].(string); ok && s != "" {
		return s
	}
	return def
}

// collectGradients indexes LinearGradient/RadialGradient nodes by id across the
// Svg subtree (typically under <Defs>), so fill="url(#id)" can resolve them.
func collectGradients(n *tree.Node) map[string]*tree.Node {
	m := map[string]*tree.Node{}
	var walk func(*tree.Node)
	walk = func(nd *tree.Node) {
		if nd.Type == contract.TypeLinearGradient || nd.Type == contract.TypeRadialGradient {
			if id, ok := nd.Props["id"].(string); ok && id != "" {
				m[id] = nd
			}
		}
		for _, c := range nd.Children {
			walk(c)
		}
	}
	walk(n)
	return m
}

// gradientID extracts the id from a fill/stroke of the form url(#id).
func gradientID(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "url(") || !strings.HasSuffix(v, ")") {
		return "", false
	}
	id := strings.Trim(v[len("url("):len(v)-1], " \"'")
	id = strings.TrimPrefix(id, "#")
	return id, id != ""
}

// pathBBox returns the bounding box [minX, minY, maxX, maxY] of a path's points
// (Bézier control points included — a conservative superset, fine for mapping a
// gradient's objectBoundingBox coordinates).
func pathBBox(p svgparse.Path) [4]float64 {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	upd := func(x, y float64) {
		minX, maxX = math.Min(minX, x), math.Max(maxX, x)
		minY, maxY = math.Min(minY, y), math.Max(maxY, y)
	}
	for _, s := range p {
		switch s.Op {
		case svgparse.MoveTo, svgparse.LineTo:
			upd(s.Args[0], s.Args[1])
		case svgparse.CubicTo:
			upd(s.Args[0], s.Args[1])
			upd(s.Args[2], s.Args[3])
			upd(s.Args[4], s.Args[5])
		}
	}
	if math.IsInf(minX, 1) {
		return [4]float64{0, 0, 0, 0}
	}
	return [4]float64{minX, minY, maxX, maxY}
}

// buildShading converts an SVG gradient node into a pdf.Shading, mapping its
// coordinates into user space per gradientUnits (objectBoundingBox default →
// relative to the shape bbox; userSpaceOnUse → as authored).
func buildShading(grad *tree.Node, bbox [4]float64) *pdf.Shading {
	stops := collectStops(grad)
	if len(stops) == 0 {
		return nil
	}
	userSpace := svgAttr(grad, "gradientUnits", "objectBoundingBox") == "userSpaceOnUse"
	bw, bh := bbox[2]-bbox[0], bbox[3]-bbox[1]
	mapX := func(v float64) float64 {
		if userSpace {
			return v
		}
		return bbox[0] + v*bw
	}
	mapY := func(v float64) float64 {
		if userSpace {
			return v
		}
		return bbox[1] + v*bh
	}

	if grad.Type == contract.TypeRadialGradient {
		cx := svgGradCoord(grad, "cx", 0.5)
		cy := svgGradCoord(grad, "cy", 0.5)
		r := svgGradCoord(grad, "r", 0.5)
		fx := svgGradCoord(grad, "fx", cx)
		fy := svgGradCoord(grad, "fy", cy)
		rr := r
		if !userSpace {
			// objectBoundingBox radius is relative to the normalized diagonal.
			rr = r * math.Sqrt((bw*bw+bh*bh)/2)
		}
		return &pdf.Shading{
			Radial: true,
			Coords: []float64{mapX(fx), mapY(fy), 0, mapX(cx), mapY(cy), rr},
			Stops:  stops,
		}
	}

	x1 := svgGradCoord(grad, "x1", 0)
	y1 := svgGradCoord(grad, "y1", 0)
	x2 := svgGradCoord(grad, "x2", 1)
	y2 := svgGradCoord(grad, "y2", 0)
	return &pdf.Shading{
		Coords: []float64{mapX(x1), mapY(y1), mapX(x2), mapY(y2)},
		Stops:  stops,
	}
}

// collectStops reads a gradient's <Stop> children into shading stops.
func collectStops(grad *tree.Node) []pdf.ShadingStop {
	var stops []pdf.ShadingStop
	for _, s := range grad.Children {
		if s.Type != contract.TypeStop {
			continue
		}
		col, err := stylesheet.ParseColor(svgAttr(s, "stopColor", "black"))
		if err != nil {
			col = stylesheet.Color{A: 1}
		}
		r, g, b := col.RGB()
		stops = append(stops, pdf.ShadingStop{Offset: svgGradCoord(s, "offset", 0), R: r, G: g, B: b})
	}
	return stops
}

// svgGradCoord reads a gradient coordinate/offset as a number, treating a
// trailing "%" as a fraction (so "50%" → 0.5).
func svgGradCoord(n *tree.Node, key string, def float64) float64 {
	v, ok := n.Props[key]
	if !ok {
		return def
	}
	switch t := v.(type) {
	case json.Number:
		f, _ := t.Float64()
		return f
	case float64:
		return t
	case string:
		if strings.HasSuffix(t, "%") {
			if f, err := strconv.ParseFloat(strings.TrimSuffix(t, "%"), 64); err == nil {
				return f / 100
			}
		}
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			return f
		}
	}
	return def
}
