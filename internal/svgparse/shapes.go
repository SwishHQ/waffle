package svgparse

// kappa is the cubic-Bézier control offset ratio that approximates a quarter
// ellipse (4/3·(√2−1)).
const kappa = 0.5522847498307936

// Rect returns the path for a rectangle. rx/ry round the corners with cubic arcs
// (each clamped to half the corresponding side).
func Rect(x, y, w, h, rx, ry float64) Path {
	if rx <= 0 && ry <= 0 {
		return Path{
			{Op: MoveTo, Args: [6]float64{x, y}},
			{Op: LineTo, Args: [6]float64{x + w, y}},
			{Op: LineTo, Args: [6]float64{x + w, y + h}},
			{Op: LineTo, Args: [6]float64{x, y + h}},
			{Op: Close},
		}
	}
	if rx <= 0 {
		rx = ry
	}
	if ry <= 0 {
		ry = rx
	}
	if rx > w/2 {
		rx = w / 2
	}
	if ry > h/2 {
		ry = h / 2
	}
	ox, oy := rx*kappa, ry*kappa
	return Path{
		{Op: MoveTo, Args: [6]float64{x + rx, y}},
		{Op: LineTo, Args: [6]float64{x + w - rx, y}},
		{Op: CubicTo, Args: [6]float64{x + w - rx + ox, y, x + w, y + ry - oy, x + w, y + ry}},
		{Op: LineTo, Args: [6]float64{x + w, y + h - ry}},
		{Op: CubicTo, Args: [6]float64{x + w, y + h - ry + oy, x + w - rx + ox, y + h, x + w - rx, y + h}},
		{Op: LineTo, Args: [6]float64{x + rx, y + h}},
		{Op: CubicTo, Args: [6]float64{x + rx - ox, y + h, x, y + h - ry + oy, x, y + h - ry}},
		{Op: LineTo, Args: [6]float64{x, y + ry}},
		{Op: CubicTo, Args: [6]float64{x, y + ry - oy, x + rx - ox, y, x + rx, y}},
		{Op: Close},
	}
}

// Ellipse returns the path for an ellipse as four cubic arcs.
func Ellipse(cx, cy, rx, ry float64) Path {
	ox, oy := rx*kappa, ry*kappa
	return Path{
		{Op: MoveTo, Args: [6]float64{cx + rx, cy}},
		{Op: CubicTo, Args: [6]float64{cx + rx, cy + oy, cx + ox, cy + ry, cx, cy + ry}},
		{Op: CubicTo, Args: [6]float64{cx - ox, cy + ry, cx - rx, cy + oy, cx - rx, cy}},
		{Op: CubicTo, Args: [6]float64{cx - rx, cy - oy, cx - ox, cy - ry, cx, cy - ry}},
		{Op: CubicTo, Args: [6]float64{cx + ox, cy - ry, cx + rx, cy - oy, cx + rx, cy}},
		{Op: Close},
	}
}

// Circle returns the path for a circle.
func Circle(cx, cy, r float64) Path { return Ellipse(cx, cy, r, r) }

// Line returns the path for a line segment.
func Line(x1, y1, x2, y2 float64) Path {
	return Path{
		{Op: MoveTo, Args: [6]float64{x1, y1}},
		{Op: LineTo, Args: [6]float64{x2, y2}},
	}
}

// Polyline returns an open path through the given [x0,y0,x1,y1,…] points.
func Polyline(pts []float64) Path { return polyPath(pts, false) }

// Polygon returns a closed path through the given points.
func Polygon(pts []float64) Path { return polyPath(pts, true) }

func polyPath(pts []float64, closed bool) Path {
	if len(pts) < 4 {
		return nil
	}
	path := Path{{Op: MoveTo, Args: [6]float64{pts[0], pts[1]}}}
	for i := 2; i+1 < len(pts); i += 2 {
		path = append(path, Segment{Op: LineTo, Args: [6]float64{pts[i], pts[i+1]}})
	}
	if closed {
		path = append(path, Segment{Op: Close})
	}
	return path
}
