package render

import (
	"encoding/json"
	"math"

	"github.com/swish/waffle/internal/layout"
	"github.com/swish/waffle/internal/pdf"
	"github.com/swish/waffle/internal/stylesheet"
	"github.com/swish/waffle/internal/svgparse"
)

// paintCanvas replays a Canvas node's recorded painter ops. Canvas coordinates
// are top-left origin (y-down), like the rest of waffle; a CTM flips them into PDF
// space. Each op is {"op": name, "args": [...]}. Text and gradients are TODO.
func paintCanvas(c *pdf.Content, box *layout.Box, pageH float64) {
	if len(box.Canvas) == 0 {
		return
	}
	f := box.Frame
	c.Save()
	c.Transform(1, 0, 0, -1, f.X, pageH-f.Y)
	replayCanvas(c, box.Canvas)
	c.Restore()
}

func replayCanvas(c *pdf.Content, ops []any) {
	var cx, cy float64
	for _, raw := range ops {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		op, _ := m["op"].(string)
		args, _ := m["args"].([]any)

		switch op {
		case "moveTo":
			cx, cy = fArg(args, 0), fArg(args, 1)
			c.MoveTo(cx, cy)
		case "lineTo":
			cx, cy = fArg(args, 0), fArg(args, 1)
			c.LineTo(cx, cy)
		case "bezierCurveTo":
			c.CurveTo(fArg(args, 0), fArg(args, 1), fArg(args, 2), fArg(args, 3), fArg(args, 4), fArg(args, 5))
			cx, cy = fArg(args, 4), fArg(args, 5)
		case "quadraticCurveTo":
			qx, qy, x, y := fArg(args, 0), fArg(args, 1), fArg(args, 2), fArg(args, 3)
			c.CurveTo(cx+2.0/3*(qx-cx), cy+2.0/3*(qy-cy), x+2.0/3*(qx-x), y+2.0/3*(qy-y), x, y)
			cx, cy = x, y
		case "rect":
			c.Rect(fArg(args, 0), fArg(args, 1), fArg(args, 2), fArg(args, 3))
		case "circle":
			emitCanvasPath(c, svgparse.Circle(fArg(args, 0), fArg(args, 1), fArg(args, 2)))
		case "ellipse":
			emitCanvasPath(c, svgparse.Ellipse(fArg(args, 0), fArg(args, 1), fArg(args, 2), fArg(args, 3)))
		case "polygon":
			emitCanvasPath(c, svgparse.Polygon(fArgs(args)))
		case "path":
			if p, err := svgparse.ParsePath(sArg(args, 0)); err == nil {
				emitCanvasPath(c, p)
			}
		case "closePath":
			c.ClosePath()
		case "fill":
			if col := sArg(args, 0); col != "" {
				setPaint(c, col, true)
			}
			c.Fill()
		case "stroke":
			if col := sArg(args, 0); col != "" {
				setPaint(c, col, false)
			}
			c.Stroke()
		case "fillAndStroke":
			c.FillStroke()
		case "lineWidth":
			c.LineWidth(fArg(args, 0))
		case "lineCap":
			c.LineCap(int(fArg(args, 0)))
		case "lineJoin":
			c.LineJoin(int(fArg(args, 0)))
		case "fillColor":
			setPaint(c, sArg(args, 0), true)
		case "strokeColor":
			setPaint(c, sArg(args, 0), false)
		case "save":
			c.Save()
		case "restore":
			c.Restore()
		case "translate":
			c.Transform(1, 0, 0, 1, fArg(args, 0), fArg(args, 1))
		case "scale":
			sx := fArg(args, 0)
			sy := sx
			if len(args) >= 2 {
				sy = fArg(args, 1)
			}
			c.Transform(sx, 0, 0, sy, 0, 0)
		case "rotate":
			rad := fArg(args, 0) * math.Pi / 180
			s, co := math.Sin(rad), math.Cos(rad)
			c.Transform(co, s, -s, co, 0, 0)
		}
	}
}

func emitCanvasPath(c *pdf.Content, p svgparse.Path) {
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

func setPaint(c *pdf.Content, color string, fill bool) {
	col, err := stylesheet.ParseColor(color)
	if err != nil {
		return
	}
	r, g, b := col.RGB()
	if fill {
		c.FillRGB(r, g, b)
	} else {
		c.StrokeRGB(r, g, b)
	}
}

func fArg(args []any, i int) float64 {
	if i >= len(args) {
		return 0
	}
	switch v := args[i].(type) {
	case json.Number:
		f, _ := v.Float64()
		return f
	case float64:
		return v
	}
	return 0
}

func fArgs(args []any) []float64 {
	out := make([]float64, len(args))
	for i := range args {
		out[i] = fArg(args, i)
	}
	return out
}

func sArg(args []any, i int) string {
	if i < len(args) {
		if s, ok := args[i].(string); ok {
			return s
		}
	}
	return ""
}
