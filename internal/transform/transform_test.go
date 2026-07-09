package transform

import (
	"math"
	"testing"
)

const eps = 1e-9

func approx(a, b float64) bool { return math.Abs(a-b) <= eps }

func mustMatEq(t *testing.T, got, want Matrix, ctx string) {
	t.Helper()
	if !approx(got.A, want.A) || !approx(got.B, want.B) || !approx(got.C, want.C) ||
		!approx(got.D, want.D) || !approx(got.E, want.E) || !approx(got.F, want.F) {
		t.Errorf("%s: got %+v, want %+v", ctx, got, want)
	}
}

func mustPtEq(t *testing.T, gx, gy, wx, wy float64, ctx string) {
	t.Helper()
	if !approx(gx, wx) || !approx(gy, wy) {
		t.Errorf("%s: got (%v, %v), want (%v, %v)", ctx, gx, gy, wx, wy)
	}
}

func TestIdentity(t *testing.T) {
	mustMatEq(t, Identity(), Matrix{A: 1, D: 1}, "Identity")
	x, y := Identity().Apply(3, -7)
	mustPtEq(t, x, y, 3, -7, "Identity.Apply")
}

func TestTranslate(t *testing.T) {
	x, y := Translate(10, 20).Apply(1, 1)
	mustPtEq(t, x, y, 11, 21, "Translate")
}

func TestScale(t *testing.T) {
	x, y := Scale(2, 3).Apply(4, 5)
	mustPtEq(t, x, y, 8, 15, "Scale")
}

// Rotate(90) must map (1,0) -> (0,1) in feast's y-down space (clockwise on
// screen: the +x axis turns toward +y, i.e. downward).
func TestRotate90YDown(t *testing.T) {
	x, y := Rotate(90).Apply(1, 0)
	mustPtEq(t, x, y, 0, 1, "Rotate(90).Apply(1,0)")

	// A further sanity point: (0,1) -> (-1,0).
	x, y = Rotate(90).Apply(0, 1)
	mustPtEq(t, x, y, -1, 0, "Rotate(90).Apply(0,1)")
}

func TestSkew(t *testing.T) {
	// skew(45deg, 0): x' = x + tan(45)*y = x + y ; y' = y.
	x, y := Skew(45, 0).Apply(1, 1)
	mustPtEq(t, x, y, 2, 1, "Skew(45,0)")

	// skew(0, 45deg): x' = x ; y' = tan(45)*x + y = x + y.
	x, y = Skew(0, 45).Apply(1, 1)
	mustPtEq(t, x, y, 1, 2, "Skew(0,45)")
}

// Mul composes as m∘n (n applied first). Order must matter.
func TestMulCompositionOrder(t *testing.T) {
	// translate ∘ scale: scale first, then translate.
	m := Translate(10, 20).Mul(Scale(2, 2))
	x, y := m.Apply(1, 1)
	mustPtEq(t, x, y, 12, 22, "Translate∘Scale")

	// The composed matrix itself.
	mustMatEq(t, m, Matrix{A: 2, D: 2, E: 10, F: 20}, "Translate∘Scale matrix")

	// Reversed order gives a different result: translate first, then scale.
	rev := Scale(2, 2).Mul(Translate(10, 20))
	rx, ry := rev.Apply(1, 1)
	mustPtEq(t, rx, ry, 22, 42, "Scale∘Translate")
	if approx(rx, x) && approx(ry, y) {
		t.Errorf("composition order should matter, both gave (%v,%v)", x, y)
	}
}

// Parse composes left-to-right as CSS does: the last-listed function transforms
// the point first. "translate(10,20) scale(2)" on (1,1) -> scale to (2,2) then
// translate to (12,22).
func TestParseCompositionMatchesCSS(t *testing.T) {
	m, err := Parse("translate(10,20) scale(2)")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	x, y := m.Apply(1, 1)
	mustPtEq(t, x, y, 12, 22, "Parse(translate scale).Apply(1,1)")

	// Equivalent to composing the two ops in the right order by hand.
	mustMatEq(t, m, Translate(10, 20).Mul(Scale(2, 2)), "Parse == Translate∘Scale")
}

func TestParseMultiFunction(t *testing.T) {
	m, err := Parse("rotate(45deg) translate(10, 5px) scale(1.5)")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	want := Rotate(45).Mul(Translate(10, 5)).Mul(Scale(1.5, 1.5))
	mustMatEq(t, m, want, "Parse multi-function")
}

func TestParseMatrix(t *testing.T) {
	m, err := Parse("matrix(1, 2, 3, 4, 5, 6)")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	mustMatEq(t, m, Matrix{A: 1, B: 2, C: 3, D: 4, E: 5, F: 6}, "matrix()")
}

func TestAngleUnits(t *testing.T) {
	half := Rotate(180)
	cases := []string{"rotate(0.5turn)", "rotate(200grad)", "rotate(3.141592653589793rad)", "rotate(180deg)", "rotate(180)"}
	for _, c := range cases {
		m, err := Parse(c)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", c, err)
		}
		mustMatEq(t, m, half, c+" == rotate(180deg)")
	}
}

func TestParseAngleHelper(t *testing.T) {
	cases := []struct {
		in   string
		want float64 // radians
	}{
		{"180deg", math.Pi},
		{"0.5turn", math.Pi},
		{"200grad", math.Pi},
		{"1rad", 1},
		{"90", math.Pi / 2}, // bare number == degrees
	}
	for _, c := range cases {
		got, err := ParseAngle(c.in)
		if err != nil {
			t.Fatalf("ParseAngle(%q) error: %v", c.in, err)
		}
		if !approx(got, c.want) {
			t.Errorf("ParseAngle(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseEmptyIsIdentity(t *testing.T) {
	m, err := Parse("")
	if err != nil {
		t.Fatalf("Parse(\"\") error: %v", err)
	}
	mustMatEq(t, m, Identity(), "Parse(\"\")")

	// Whitespace-only is also identity.
	m, err = Parse("   \t ")
	if err != nil {
		t.Fatalf("Parse(spaces) error: %v", err)
	}
	mustMatEq(t, m, Identity(), "Parse(spaces)")
}

func TestParseUnknownIgnored(t *testing.T) {
	// Unknown/unsupported functions are ignored (treated as identity).
	m, err := Parse("translate3d(1, 2, 3)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustMatEq(t, m, Identity(), "unknown translate3d ignored")

	// Unknown function mixed with a known one keeps only the known effect.
	m, err = Parse("wobble(5) translate(3, 4)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustMatEq(t, m, Translate(3, 4), "unknown wobble ignored, translate kept")
}

func TestParseGarbageErrors(t *testing.T) {
	// Malformed input returns (Identity, error) and must never panic.
	bad := []string{
		"garbage!!!",      // name not followed by '('
		"rotate(",         // unclosed paren
		"rotate(90",       // unclosed paren
		"matrix(1, 2, 3)", // wrong argument count
		"translate(50%)",  // percentage length unsupported
		"scale(abc)",      // non-numeric
		"rotate(45xyz)",   // unknown angle unit -> bad number
		"()",              // missing name
	}
	for _, s := range bad {
		m, err := Parse(s)
		if err == nil {
			t.Errorf("Parse(%q) expected error, got nil (matrix %+v)", s, m)
		}
		mustMatEq(t, m, Identity(), "Parse("+s+") returns Identity on error")
	}
}

func TestAxisFunctions(t *testing.T) {
	m, _ := Parse("translateX(7)")
	mustMatEq(t, m, Translate(7, 0), "translateX")

	m, _ = Parse("translateY(7pt)")
	mustMatEq(t, m, Translate(0, 7), "translateY")

	m, _ = Parse("scaleX(3)")
	mustMatEq(t, m, Scale(3, 1), "scaleX")

	m, _ = Parse("scaleY(3)")
	mustMatEq(t, m, Scale(1, 3), "scaleY")

	m, _ = Parse("scale(2)")
	mustMatEq(t, m, Scale(2, 2), "scale single arg is uniform")
}

// AboutOrigin applies a transform about a pivot (CSS transform-origin): the
// pivot is a fixed point, and rotation is clockwise in y-down space.
func TestAboutOrigin(t *testing.T) {
	r := Rotate(90).AboutOrigin(5, 5)

	// The pivot is fixed.
	x, y := r.Apply(5, 5)
	mustPtEq(t, x, y, 5, 5, "AboutOrigin pivot fixed")

	// A point to the right of the pivot rotates to below it (clockwise, y-down).
	x, y = r.Apply(6, 5)
	mustPtEq(t, x, y, 5, 6, "AboutOrigin clockwise")
}
