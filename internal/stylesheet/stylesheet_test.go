package stylesheet

import (
	"encoding/json"
	"math"
	"testing"
)

func TestParseValueUnits(t *testing.T) {
	ctx := Context{DPI: 72, RemBase: 18, PageW: 600, PageH: 800}
	cases := []struct {
		in    any
		unit  Unit
		pts   float64 // expected absolute points (basis 0); skip check if relative
		basis float64
		rel   float64 // expected resolved value when relative (basis applied)
	}{
		{in: 10.0, unit: UnitPt, pts: 10},
		{in: json.Number("12"), unit: UnitPt, pts: 12},
		{in: "10pt", unit: UnitPt, pts: 10},
		{in: "1in", unit: UnitIn, pts: 72},
		{in: "25.4mm", unit: UnitMm, pts: 72},
		{in: "2.54cm", unit: UnitCm, pts: 72},
		{in: "96px", unit: UnitPx, pts: 96}, // at 72 dpi, px==pt
		{in: "2rem", unit: UnitRem, pts: 36},
		{in: "-10", unit: UnitPt, pts: -10},
		{in: "50%", unit: UnitPercent, basis: 200, rel: 100},
		{in: "50vw", unit: UnitVw, rel: 300},
		{in: "25vh", unit: UnitVh, rel: 200},
	}
	for _, c := range cases {
		v, err := ParseValue(c.in)
		if err != nil {
			t.Errorf("ParseValue(%v): %v", c.in, err)
			continue
		}
		if v.Unit != c.unit {
			t.Errorf("ParseValue(%v).Unit = %v, want %v", c.in, v.Unit, c.unit)
		}
		if v.IsRelative() {
			got := v.Resolve(ctx, c.basis)
			if math.Abs(got-c.rel) > 1e-6 {
				t.Errorf("Resolve(%v, basis=%v) = %v, want %v", c.in, c.basis, got, c.rel)
			}
			if _, ok := v.ToPoints(ctx); ok {
				t.Errorf("ToPoints(%v) should report ok=false for relative units", c.in)
			}
		} else {
			got, ok := v.ToPoints(ctx)
			if !ok || math.Abs(got-c.pts) > 1e-6 {
				t.Errorf("ToPoints(%v) = %v (ok=%v), want %v", c.in, got, ok, c.pts)
			}
		}
	}
}

func TestParseValueAutoAndErrors(t *testing.T) {
	v, err := ParseValue("auto")
	if err != nil || !v.IsAuto() {
		t.Errorf("ParseValue(auto) = %v, %v; want auto", v, err)
	}
	for _, bad := range []any{"", "abc", "10foo", "10 px 20", true} {
		if _, err := ParseValue(bad); err == nil {
			t.Errorf("ParseValue(%v) should error", bad)
		}
	}
}

func TestPxRespectsDPI(t *testing.T) {
	v, _ := ParseValue("96px")
	if got, _ := v.ToPoints(Context{DPI: 96}); math.Abs(got-72) > 1e-6 {
		t.Errorf("96px at 96dpi = %v pts, want 72", got)
	}
}

func TestParseColor(t *testing.T) {
	cases := []struct {
		in      string
		r, g, b uint8
		a       float64
		wantErr bool
	}{
		{in: "#fff", r: 255, g: 255, b: 255, a: 1},
		{in: "#000000", r: 0, g: 0, b: 0, a: 1},
		{in: "#ff0000", r: 255, g: 0, b: 0, a: 1},
		{in: "#00ff0080", r: 0, g: 255, b: 0, a: 128.0 / 255},
		{in: "#0f08", r: 0, g: 255, b: 0, a: 136.0 / 255},
		{in: "rgb(255, 0, 0)", r: 255, g: 0, b: 0, a: 1},
		{in: "rgba(0, 128, 255, 0.5)", r: 0, g: 128, b: 255, a: 0.5},
		{in: "rgb(100%, 0%, 0%)", r: 255, g: 0, b: 0, a: 1},
		{in: "hsl(0, 100%, 50%)", r: 255, g: 0, b: 0, a: 1},
		{in: "hsl(120, 100%, 50%)", r: 0, g: 255, b: 0, a: 1},
		{in: "hsl(240, 100%, 50%)", r: 0, g: 0, b: 255, a: 1},
		{in: "red", r: 255, g: 0, b: 0, a: 1},
		{in: "rebeccapurple", r: 0x66, g: 0x33, b: 0x99, a: 1},
		{in: "CornflowerBlue", r: 0x64, g: 0x95, b: 0xed, a: 1},
		{in: "transparent", r: 0, g: 0, b: 0, a: 0},
		{in: "notacolor", wantErr: true},
		{in: "#12345", wantErr: true},
	}
	for _, c := range cases {
		got, err := ParseColor(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseColor(%q) should error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseColor(%q): %v", c.in, err)
			continue
		}
		if got.R != c.r || got.G != c.g || got.B != c.b || math.Abs(got.A-c.a) > 1e-3 {
			t.Errorf("ParseColor(%q) = %+v, want {%d %d %d %.3f}", c.in, got, c.r, c.g, c.b, c.a)
		}
	}
}

func TestColorRGBNormalized(t *testing.T) {
	c, _ := ParseColor("#8040c0")
	r, g, b := c.RGB()
	if math.Abs(r-128.0/255) > 1e-6 || math.Abs(g-64.0/255) > 1e-6 || math.Abs(b-192.0/255) > 1e-6 {
		t.Errorf("RGB() = %v %v %v", r, g, b)
	}
}

func TestParseFontWeight(t *testing.T) {
	cases := []struct {
		in   any
		want int
		ok   bool
	}{
		{"normal", 400, true},
		{"bold", 700, true},
		{"Thin", 100, true},
		{"semibold", 600, true},
		{"black", 900, true},
		{json.Number("650"), 650, true},
		{700.0, 700, true},
		{"500", 500, true},
		{"nope", 0, false},
		{nil, 0, false},
	}
	for _, c := range cases {
		got, ok := ParseFontWeight(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("ParseFontWeight(%v) = %d, %v; want %d, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestFlatten(t *testing.T) {
	// Single object.
	m := Flatten(map[string]any{"color": "red", "margin": 10.0})
	if m["color"] != "red" || m["margin"] != 10.0 {
		t.Errorf("flatten object = %v", m)
	}

	// Array: later wins.
	m = Flatten([]any{
		map[string]any{"color": "red", "padding": 5.0},
		map[string]any{"color": "blue"},
	})
	if m["color"] != "blue" || m["padding"] != 5.0 {
		t.Errorf("flatten array later-wins = %v", m)
	}

	// Nested arrays and nil entries are handled.
	m = Flatten([]any{
		map[string]any{"a": 1.0},
		nil,
		[]any{map[string]any{"a": 2.0}, map[string]any{"b": 3.0}},
	})
	if m["a"] != 2.0 || m["b"] != 3.0 {
		t.Errorf("flatten nested = %v", m)
	}

	// nil style yields an empty, non-nil map.
	if m := Flatten(nil); m == nil || len(m) != 0 {
		t.Errorf("flatten(nil) = %v, want empty non-nil map", m)
	}
}
