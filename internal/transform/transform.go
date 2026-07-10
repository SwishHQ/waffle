// Package transform parses CSS transform strings (translate, scale, rotate,
// skew, matrix) and composes them into a single 2D affine matrix expressed in
// PDF operand order [A B C D E F]. The renderer emits that matrix directly as a
// PDF `cm` operator to position and orient content.
//
// Matrix convention. A Matrix maps a point (x, y) to (x', y') as
//
//	x' = A*x + C*y + E
//	y' = B*x + D*y + F
//
// which is exactly the PDF `cm` operator's meaning. In 3x3 homogeneous form
// with row vectors [x y 1]:
//
//	[x' y' 1] = [x y 1] * | A B 0 |
//	                      | C D 0 |
//	                      | E F 1 |
//
// Coordinate space. waffle's layout space is y-DOWN (origin top-left, like CSS
// and the screen). All matrices here operate in that y-down space; the renderer
// applies them BEFORE its own y-flip into PDF's y-up page space. Rotate is
// defined so a positive angle rotates CLOCKWISE as seen on screen, matching the
// visual result of CSS `rotate()`. See Rotate for the exact formula.
package transform

import "math"

// Matrix is a 2D affine transform stored in PDF operand order: [A B C D E F].
// The zero value is NOT the identity; use Identity.
type Matrix struct{ A, B, C, D, E, F float64 }

// Identity returns the identity transform (maps every point to itself).
func Identity() Matrix { return Matrix{A: 1, B: 0, C: 0, D: 1, E: 0, F: 0} }

// Mul returns the composition m∘n: the transform that applies n first and then
// m, so (m.Mul(n)).Apply(p) == m.Apply(n.Apply(p)). Composition is associative
// but not commutative.
func (m Matrix) Mul(n Matrix) Matrix {
	return Matrix{
		A: m.A*n.A + m.C*n.B,
		B: m.B*n.A + m.D*n.B,
		C: m.A*n.C + m.C*n.D,
		D: m.B*n.C + m.D*n.D,
		E: m.A*n.E + m.C*n.F + m.E,
		F: m.B*n.E + m.D*n.F + m.F,
	}
}

// Apply maps the point (x, y) through m and returns the transformed point.
func (m Matrix) Apply(x, y float64) (float64, float64) {
	return m.A*x + m.C*y + m.E, m.B*x + m.D*y + m.F
}

// AboutOrigin returns m applied about the point (ox, oy) instead of about the
// coordinate origin, i.e. translate(ox,oy) ∘ m ∘ translate(-ox,-oy). This is
// what a renderer uses to honor CSS transform-origin: wrap an element's own
// transform M as element.AboutOrigin(originX, originY).
func (m Matrix) AboutOrigin(ox, oy float64) Matrix {
	return Translate(ox, oy).Mul(m).Mul(Translate(-ox, -oy))
}

// Translate returns a translation by (tx, ty) in points.
func Translate(tx, ty float64) Matrix { return Matrix{A: 1, B: 0, C: 0, D: 1, E: tx, F: ty} }

// Scale returns a scale by sx horizontally and sy vertically.
func Scale(sx, sy float64) Matrix { return Matrix{A: sx, B: 0, C: 0, D: sy, E: 0, F: 0} }

// Rotate returns a rotation by deg degrees. In waffle's y-down space a positive
// angle rotates CLOCKWISE as seen on screen, matching CSS `rotate()`. The matrix
// uses the standard trigonometric form
//
//	A = cos θ   B = sin θ   C = -sin θ   D = cos θ
//
// (θ = deg in radians); it appears clockwise because y increases downward. For
// example Rotate(90) maps (1,0) -> (0,1), i.e. the +x axis turns toward +y
// (down on screen).
func Rotate(deg float64) Matrix { return rotateRad(deg * math.Pi / 180) }

// Skew returns a skew by ax degrees along x and ay degrees along y, matching CSS
// `skew(ax, ay)` == matrix(1, tan(ay), tan(ax), 1, 0, 0).
func Skew(ax, ay float64) Matrix {
	return skewRad(ax*math.Pi/180, ay*math.Pi/180)
}

// rotateRad builds a rotation matrix from an angle in radians.
func rotateRad(rad float64) Matrix {
	s, c := math.Sincos(rad)
	return Matrix{A: c, B: s, C: -s, D: c, E: 0, F: 0}
}

// skewRad builds a skew matrix from angles in radians.
func skewRad(axRad, ayRad float64) Matrix {
	return Matrix{A: 1, B: math.Tan(ayRad), C: math.Tan(axRad), D: 1, E: 0, F: 0}
}
