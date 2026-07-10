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
	"github.com/swish/feast/internal/transform"
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

	defs := collectDefs(node)

	c.Save()
	// Map SVG user space (y-down) onto the box in PDF space (y-up).
	c.Transform(sx, 0, 0, -sy, f.X-minX*sx, (pageH-f.Y)+minY*sy)
	for _, child := range node.Children {
		drawSVGNode(c, child, defs)
	}
	c.Restore()
}

// svgDefs indexes referenceable SVG definitions (gradients, clip paths) by id.
type svgDefs struct {
	gradients map[string]*tree.Node
	clipPaths map[string]*tree.Node
}

func drawSVGNode(c *pdf.Content, n *tree.Node, defs svgDefs) {
	// A transform attribute establishes a new coordinate system for this node and
	// its descendants; a clip-path restricts painting to a shape. Both wrap the
	// node in graphics-state saves that compose under the viewBox CTM.
	saves := 0
	if tf := svgAttr(n, "transform", ""); tf != "" {
		if m, ok := parseSVGTransform(tf); ok {
			c.Save()
			c.Transform(m.A, m.B, m.C, m.D, m.E, m.F)
			saves++
		}
	}
	if cp := svgAttr(n, "clipPath", ""); cp != "" {
		if id, ok := gradientID(cp); ok {
			if clip := defs.clipPaths[id]; clip != nil {
				c.Save()
				emitClip(c, clip)
				saves++
			}
		}
	}

	switch n.Type {
	case contract.TypeG:
		for _, ch := range n.Children {
			drawSVGNode(c, ch, defs)
		}
	case contract.TypeText, contract.TypeTspan:
		drawSVGText(c, n)
	default:
		if p, ok := nodePath(n); ok {
			emitAndPaint(c, p, n, defs.gradients)
		}
	}

	for i := 0; i < saves; i++ {
		c.Restore()
	}
}

// nodePath returns the outline path for an SVG shape node (path/rect/circle/
// ellipse/line/polyline/polygon), or ok=false for non-shape nodes.
func nodePath(n *tree.Node) (svgparse.Path, bool) {
	switch n.Type {
	case contract.TypePath:
		if d, ok := n.Props["d"].(string); ok {
			if p, err := svgparse.ParsePath(d); err == nil {
				return p, true
			}
		}
	case contract.TypeRect:
		return svgparse.Rect(numP(n, "x"), numP(n, "y"), numP(n, "width"), numP(n, "height"), numP(n, "rx"), numP(n, "ry")), true
	case contract.TypeCircle:
		return svgparse.Circle(numP(n, "cx"), numP(n, "cy"), numP(n, "r")), true
	case contract.TypeEllipse:
		return svgparse.Ellipse(numP(n, "cx"), numP(n, "cy"), numP(n, "rx"), numP(n, "ry")), true
	case contract.TypeLine:
		return svgparse.Line(numP(n, "x1"), numP(n, "y1"), numP(n, "x2"), numP(n, "y2")), true
	case contract.TypePolyline:
		return svgparse.Polyline(pointList(n)), true
	case contract.TypePolygon:
		return svgparse.Polygon(pointList(n)), true
	}
	return nil, false
}

// emitClip intersects the clip region with the union of a clipPath's child shapes.
func emitClip(c *pdf.Content, clip *tree.Node) {
	drew := false
	for _, ch := range clip.Children {
		if p, ok := nodePath(ch); ok {
			emitPath(c, p)
			drew = true
		}
	}
	if drew {
		c.Clip().EndPath()
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
	// Scope solid-shape painting so per-shape stroke styling and opacity do not
	// leak into sibling shapes.
	c.Save()
	if a := svgShapeAlpha(n); a < 1 {
		c.SetAlpha(a)
	}
	emitPath(c, p)
	paintSVGShape(c, n)
	c.Restore()
}

// svgShapeAlpha resolves a shape's constant alpha from opacity and
// fill-/stroke-opacity. A single alpha is applied: the fill's when the shape is
// filled (the common case), otherwise the stroke's, otherwise the group opacity.
func svgShapeAlpha(n *tree.Node) float64 {
	op := opacityAttr(n, "opacity", 1)
	if fill := svgAttr(n, "fill", "black"); fill != "" && fill != "none" {
		return op * opacityAttr(n, "fillOpacity", 1)
	}
	if stroke := svgAttr(n, "stroke", "none"); stroke != "" && stroke != "none" {
		return op * opacityAttr(n, "strokeOpacity", 1)
	}
	return op
}

// opacityAttr reads a 0..1 opacity attribute, returning def when absent.
func opacityAttr(n *tree.Node, key string, def float64) float64 {
	if _, ok := n.Props[key]; !ok {
		return def
	}
	v := numP(n, key)
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// applySVGStrokeStyle sets stroke dash pattern, line cap, and line join from a
// shape's SVG attributes. Defaults (butt cap, miter join, solid) are left as-is.
func applySVGStrokeStyle(c *pdf.Content, n *tree.Node) {
	if da := svgAttr(n, "strokeDasharray", ""); da != "" && da != "none" {
		if dashes := svgTransformArgs(da); len(dashes) > 0 {
			c.Dash(0, dashes...)
		}
	}
	switch svgAttr(n, "strokeLinecap", "") {
	case "round":
		c.LineCap(1)
	case "square":
		c.LineCap(2)
	}
	switch svgAttr(n, "strokeLinejoin", "") {
	case "round":
		c.LineJoin(1)
	case "bevel":
		c.LineJoin(2)
	}
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
			applySVGStrokeStyle(c, n)
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
		applySVGStrokeStyle(c, n)
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

// parseSVGTransform parses an SVG transform attribute (a whitespace/comma list of
// translate/scale/rotate/skewX/skewY/matrix functions with unitless numbers) into
// a single matrix. Reports ok=false when nothing parsed.
func parseSVGTransform(s string) (transform.Matrix, bool) {
	m := transform.Identity()
	any := false
	for {
		s = strings.TrimSpace(s)
		open := strings.IndexByte(s, '(')
		if open < 0 {
			break
		}
		fn := strings.TrimSpace(s[:open])
		closeIdx := strings.IndexByte(s, ')')
		if closeIdx < 0 || closeIdx < open {
			break
		}
		args := svgTransformArgs(s[open+1 : closeIdx])
		s = s[closeIdx+1:]

		var t transform.Matrix
		switch fn {
		case "translate":
			t = transform.Translate(svgArg(args, 0, 0), svgArg(args, 1, 0))
		case "scale":
			sx := svgArg(args, 0, 1)
			t = transform.Scale(sx, svgArg(args, 1, sx))
		case "rotate":
			t = transform.Rotate(svgArg(args, 0, 0))
			if len(args) >= 3 {
				t = t.AboutOrigin(args[1], args[2])
			}
		case "skewX":
			t = transform.Skew(svgArg(args, 0, 0), 0)
		case "skewY":
			t = transform.Skew(0, svgArg(args, 0, 0))
		case "matrix":
			if len(args) != 6 {
				continue
			}
			t = transform.Matrix{A: args[0], B: args[1], C: args[2], D: args[3], E: args[4], F: args[5]}
		default:
			continue
		}
		m = m.Mul(t)
		any = true
	}
	return m, any
}

// svgTransformArgs parses a function's numeric arguments (space/comma separated).
func svgTransformArgs(s string) []float64 {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' || r == '\n' })
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		if v, err := strconv.ParseFloat(f, 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

func svgArg(args []float64, i int, def float64) float64 {
	if i < len(args) {
		return args[i]
	}
	return def
}

// collectDefs indexes referenceable definitions (gradients, clip paths) by id
// across the Svg subtree (typically under <Defs>), so url(#id) references resolve.
func collectDefs(n *tree.Node) svgDefs {
	defs := svgDefs{gradients: map[string]*tree.Node{}, clipPaths: map[string]*tree.Node{}}
	var walk func(*tree.Node)
	walk = func(nd *tree.Node) {
		if id, ok := nd.Props["id"].(string); ok && id != "" {
			switch nd.Type {
			case contract.TypeLinearGradient, contract.TypeRadialGradient:
				defs.gradients[id] = nd
			case contract.TypeClipPath:
				defs.clipPaths[id] = nd
			}
		}
		for _, c := range nd.Children {
			walk(c)
		}
	}
	walk(n)
	return defs
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
