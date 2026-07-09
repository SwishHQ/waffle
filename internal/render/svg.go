package render

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/swish/feast/internal/contract"
	"github.com/swish/feast/internal/layout"
	"github.com/swish/feast/internal/pdf"
	"github.com/swish/feast/internal/stylesheet"
	"github.com/swish/feast/internal/svgparse"
	"github.com/swish/feast/internal/tree"
)

// paintSVG draws an Svg box: it maps the viewBox onto the box frame (with the
// Y-flip into PDF space) and draws each shape. Transforms, gradients, clip paths,
// and svg <text> are later refinements.
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

	c.Save()
	// Map SVG user space (y-down) onto the box in PDF space (y-up).
	c.Transform(sx, 0, 0, -sy, f.X-minX*sx, (pageH-f.Y)+minY*sy)
	for _, child := range node.Children {
		drawSVGNode(c, child)
	}
	c.Restore()
}

func drawSVGNode(c *pdf.Content, n *tree.Node) {
	switch n.Type {
	case contract.TypeG:
		for _, ch := range n.Children {
			drawSVGNode(c, ch)
		}
	case contract.TypePath:
		if d, ok := n.Props["d"].(string); ok {
			if p, err := svgparse.ParsePath(d); err == nil {
				emitAndPaint(c, p, n)
			}
		}
	case contract.TypeRect:
		emitAndPaint(c, svgparse.Rect(numP(n, "x"), numP(n, "y"), numP(n, "width"), numP(n, "height"), numP(n, "rx"), numP(n, "ry")), n)
	case contract.TypeCircle:
		emitAndPaint(c, svgparse.Circle(numP(n, "cx"), numP(n, "cy"), numP(n, "r")), n)
	case contract.TypeEllipse:
		emitAndPaint(c, svgparse.Ellipse(numP(n, "cx"), numP(n, "cy"), numP(n, "rx"), numP(n, "ry")), n)
	case contract.TypeLine:
		emitAndPaint(c, svgparse.Line(numP(n, "x1"), numP(n, "y1"), numP(n, "x2"), numP(n, "y2")), n)
	case contract.TypePolyline:
		emitAndPaint(c, svgparse.Polyline(pointList(n)), n)
	case contract.TypePolygon:
		emitAndPaint(c, svgparse.Polygon(pointList(n)), n)
	}
}

func emitAndPaint(c *pdf.Content, p svgparse.Path, n *tree.Node) {
	if len(p) == 0 {
		return
	}
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
	paintSVGShape(c, n)
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
