package svgparse

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestParsePathBasic(t *testing.T) {
	p, err := ParsePath("M10 10 L20 20 C30 30 40 40 50 50 Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 4 {
		t.Fatalf("segments = %d, want 4", len(p))
	}
	if p[0].Op != MoveTo || !approx(p[0].Args[0], 10) || !approx(p[0].Args[1], 10) {
		t.Errorf("seg0 = %+v", p[0])
	}
	if p[1].Op != LineTo || !approx(p[1].Args[0], 20) {
		t.Errorf("seg1 = %+v", p[1])
	}
	if p[2].Op != CubicTo || !approx(p[2].Args[4], 50) || !approx(p[2].Args[5], 50) {
		t.Errorf("seg2 = %+v", p[2])
	}
	if p[3].Op != Close {
		t.Errorf("seg3 = %+v", p[3])
	}
}

func TestParsePathRelative(t *testing.T) {
	p, _ := ParsePath("m10 10 l5 0")
	if !approx(p[1].Args[0], 15) || !approx(p[1].Args[1], 10) {
		t.Errorf("relative lineto = %+v, want (15,10)", p[1].Args)
	}
}

func TestParsePathImplicitLineto(t *testing.T) {
	// After M, extra coordinate pairs are implicit line-tos.
	p, _ := ParsePath("M0 0 10 10 20 20")
	if len(p) != 3 || p[0].Op != MoveTo || p[1].Op != LineTo || p[2].Op != LineTo {
		t.Fatalf("implicit lineto: %+v", p)
	}
	if !approx(p[2].Args[0], 20) || !approx(p[2].Args[1], 20) {
		t.Errorf("seg2 = %+v", p[2])
	}
}

func TestParsePathHV(t *testing.T) {
	p, _ := ParsePath("M0 0 H10 V10")
	if !approx(p[1].Args[0], 10) || !approx(p[1].Args[1], 0) {
		t.Errorf("H = %+v", p[1].Args)
	}
	if !approx(p[2].Args[0], 10) || !approx(p[2].Args[1], 10) {
		t.Errorf("V = %+v", p[2].Args)
	}
}

func TestQuadToCubicConversion(t *testing.T) {
	p, _ := ParsePath("M0 0 Q10 10 20 0")
	seg := p[1]
	if seg.Op != CubicTo {
		t.Fatalf("want cubic, got %+v", seg)
	}
	// c1 = (0,0)+2/3*(10,10) = (6.667,6.667); c2 = (20,0)+2/3*(-10,10) = (13.333,6.667).
	if !approx(seg.Args[0], 20.0/3) || !approx(seg.Args[1], 20.0/3) {
		t.Errorf("c1 = (%v,%v)", seg.Args[0], seg.Args[1])
	}
	if !approx(seg.Args[4], 20) || !approx(seg.Args[5], 0) {
		t.Errorf("end = (%v,%v)", seg.Args[4], seg.Args[5])
	}
}

func TestParseTightNumbers(t *testing.T) {
	// Numbers packed by sign/decimal without separators.
	p, _ := ParsePath("M0 0L1.5.5-2-3")
	if len(p) != 3 {
		t.Fatalf("segments = %d, want 3: %+v", len(p), p)
	}
	if !approx(p[1].Args[0], 1.5) || !approx(p[1].Args[1], 0.5) {
		t.Errorf("seg1 = %+v", p[1].Args)
	}
	if !approx(p[2].Args[0], -2) || !approx(p[2].Args[1], -3) {
		t.Errorf("seg2 = %+v", p[2].Args)
	}
}

func TestShapes(t *testing.T) {
	if r := Rect(0, 0, 10, 20, 0, 0); len(r) != 5 || r[0].Op != MoveTo || r[4].Op != Close {
		t.Errorf("plain rect = %+v", r)
	}
	if rr := Rect(0, 0, 10, 20, 3, 3); len(rr) != 10 {
		t.Errorf("rounded rect segments = %d, want 10", len(rr))
	}
	c := Circle(5, 5, 3)
	if len(c) != 6 || c[0].Op != MoveTo || !approx(c[0].Args[0], 8) {
		t.Errorf("circle = %+v", c)
	}
	if pg := Polygon([]float64{0, 0, 10, 0, 10, 10}); len(pg) != 4 || pg[3].Op != Close {
		t.Errorf("polygon = %+v", pg)
	}
	if ln := Line(0, 0, 5, 5); len(ln) != 2 {
		t.Errorf("line = %+v", ln)
	}
}
