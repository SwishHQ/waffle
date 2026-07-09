// Package svgparse parses SVG path data and basic shapes into a vector IR of
// move/line/cubic/close segments, ready to emit as PDF path operators. It parses
// geometry only (not rasterizing); presentation attributes and the wider SVG
// document model are handled by the SVG renderer.
package svgparse

import (
	"fmt"
	"strconv"
)

// Op is a path segment operator.
type Op int

const (
	MoveTo Op = iota
	LineTo
	CubicTo // Args: c1x, c1y, c2x, c2y, x, y
	Close
)

// Segment is one path operation. MoveTo/LineTo use Args[0:2]; CubicTo uses all six.
type Segment struct {
	Op   Op
	Args [6]float64
}

// Path is a sequence of segments in absolute user coordinates.
type Path []Segment

type ptoken struct {
	isCmd bool
	cmd   byte
	num   float64
}

// ParsePath parses an SVG path "d" string. Quadratic curves are converted to
// cubics; smooth curves (S/T) reflect the previous control point. Arcs (A/a) are
// approximated by a line to the endpoint for now (proper arc→cubic is a TODO).
func ParsePath(d string) (Path, error) {
	toks := tokenizePath(d)
	var (
		path           Path
		cx, cy, sx, sy float64
		ctrlX, ctrlY   float64
		cmd, prev      byte
		i              int
	)
	num := func() (float64, bool) {
		if i < len(toks) && !toks[i].isCmd {
			v := toks[i].num
			i++
			return v, true
		}
		return 0, false
	}
	need := func(vs ...*float64) bool {
		for _, p := range vs {
			v, ok := num()
			if !ok {
				return false
			}
			*p = v
		}
		return true
	}

	for i < len(toks) {
		if toks[i].isCmd {
			cmd = toks[i].cmd
			i++
		}
		abs := cmd >= 'A' && cmd <= 'Z'
		switch lower(cmd) {
		case 'z':
			path = append(path, Segment{Op: Close})
			cx, cy = sx, sy
		case 'm':
			var x, y float64
			if !need(&x, &y) {
				return path, errShort(cmd)
			}
			if !abs {
				x, y = x+cx, y+cy
			}
			cx, cy, sx, sy = x, y, x, y
			path = append(path, Segment{Op: MoveTo, Args: [6]float64{x, y}})
			if abs {
				cmd = 'L'
			} else {
				cmd = 'l'
			}
		case 'l':
			var x, y float64
			if !need(&x, &y) {
				return path, errShort(cmd)
			}
			if !abs {
				x, y = x+cx, y+cy
			}
			cx, cy = x, y
			path = append(path, Segment{Op: LineTo, Args: [6]float64{x, y}})
		case 'h':
			var x float64
			if !need(&x) {
				return path, errShort(cmd)
			}
			if !abs {
				x += cx
			}
			cx = x
			path = append(path, Segment{Op: LineTo, Args: [6]float64{x, cy}})
		case 'v':
			var y float64
			if !need(&y) {
				return path, errShort(cmd)
			}
			if !abs {
				y += cy
			}
			cy = y
			path = append(path, Segment{Op: LineTo, Args: [6]float64{cx, y}})
		case 'c':
			var x1, y1, x2, y2, x, y float64
			if !need(&x1, &y1, &x2, &y2, &x, &y) {
				return path, errShort(cmd)
			}
			if !abs {
				x1, y1, x2, y2, x, y = x1+cx, y1+cy, x2+cx, y2+cy, x+cx, y+cy
			}
			path = append(path, Segment{Op: CubicTo, Args: [6]float64{x1, y1, x2, y2, x, y}})
			ctrlX, ctrlY, cx, cy = x2, y2, x, y
		case 's':
			var x2, y2, x, y float64
			if !need(&x2, &y2, &x, &y) {
				return path, errShort(cmd)
			}
			if !abs {
				x2, y2, x, y = x2+cx, y2+cy, x+cx, y+cy
			}
			x1, y1 := cx, cy
			if p := lower(prev); p == 'c' || p == 's' {
				x1, y1 = 2*cx-ctrlX, 2*cy-ctrlY
			}
			path = append(path, Segment{Op: CubicTo, Args: [6]float64{x1, y1, x2, y2, x, y}})
			ctrlX, ctrlY, cx, cy = x2, y2, x, y
		case 'q':
			var qx, qy, x, y float64
			if !need(&qx, &qy, &x, &y) {
				return path, errShort(cmd)
			}
			if !abs {
				qx, qy, x, y = qx+cx, qy+cy, x+cx, y+cy
			}
			path = append(path, quadToCubic(cx, cy, qx, qy, x, y))
			ctrlX, ctrlY, cx, cy = qx, qy, x, y
		case 't':
			var x, y float64
			if !need(&x, &y) {
				return path, errShort(cmd)
			}
			if !abs {
				x, y = x+cx, y+cy
			}
			qx, qy := cx, cy
			if p := lower(prev); p == 'q' || p == 't' {
				qx, qy = 2*cx-ctrlX, 2*cy-ctrlY
			}
			path = append(path, quadToCubic(cx, cy, qx, qy, x, y))
			ctrlX, ctrlY, cx, cy = qx, qy, x, y
		case 'a':
			// Arc: rx ry rot large sweep x y. TODO: convert to cubics; for now a line.
			var rx, ry, rot, large, sweep, x, y float64
			if !need(&rx, &ry, &rot, &large, &sweep, &x, &y) {
				return path, errShort(cmd)
			}
			if !abs {
				x, y = x+cx, y+cy
			}
			cx, cy = x, y
			path = append(path, Segment{Op: LineTo, Args: [6]float64{x, y}})
		default:
			return path, fmt.Errorf("svgparse: unknown path command %q", cmd)
		}
		prev = cmd
	}
	return path, nil
}

func quadToCubic(x0, y0, qx, qy, x, y float64) Segment {
	c1x := x0 + 2.0/3*(qx-x0)
	c1y := y0 + 2.0/3*(qy-y0)
	c2x := x + 2.0/3*(qx-x)
	c2y := y + 2.0/3*(qy-y)
	return Segment{Op: CubicTo, Args: [6]float64{c1x, c1y, c2x, c2y, x, y}}
}

func tokenizePath(d string) []ptoken {
	var toks []ptoken
	i := 0
	for i < len(d) {
		c := d[i]
		switch {
		case c == ' ' || c == ',' || c == '\t' || c == '\n' || c == '\r':
			i++
		case (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z'):
			toks = append(toks, ptoken{isCmd: true, cmd: c})
			i++
		default:
			start := i
			if c == '+' || c == '-' {
				i++
			}
			dot := false
			for i < len(d) {
				if d[i] >= '0' && d[i] <= '9' {
					i++
				} else if d[i] == '.' && !dot {
					dot = true
					i++
				} else {
					break
				}
			}
			if i < len(d) && (d[i] == 'e' || d[i] == 'E') {
				i++
				if i < len(d) && (d[i] == '+' || d[i] == '-') {
					i++
				}
				for i < len(d) && d[i] >= '0' && d[i] <= '9' {
					i++
				}
			}
			if i > start {
				if f, err := strconv.ParseFloat(d[start:i], 64); err == nil {
					toks = append(toks, ptoken{num: f})
				}
			} else {
				i++ // skip unrecognized character
			}
		}
	}
	return toks
}

func lower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

func errShort(cmd byte) error {
	return fmt.Errorf("svgparse: not enough arguments for path command %q", cmd)
}
