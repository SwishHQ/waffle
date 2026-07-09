package stylesheet

import (
	"reflect"
	"testing"
)

func TestMatchMedia(t *testing.T) {
	ctx := MediaContext{Width: 400, Height: 800, Orientation: "portrait"}
	cases := []struct {
		key   string
		match bool
		err   bool
	}{
		{"@media min-width: 300", true, false},
		{"@media min-width: 500", false, false},
		{"@media max-width: 400", true, false},
		{"@media max-width: 399", false, false},
		{"@media orientation: portrait", true, false},
		{"@media orientation: landscape", false, false},
		{"@media min-width: 300 and max-width: 500", true, false},
		{"@media min-width: 300 and max-width: 350", false, false},
		{"@media (min-width: 300) and (orientation: portrait)", true, false},
		{"@media", true, false},
		{"@media min-width: 400px", true, false}, // trailing unit tolerated
		{"@media bogus-feature: 3", false, true},
		{"not-media", false, true},
	}
	for _, c := range cases {
		got, err := MatchMedia(c.key, ctx)
		if (err != nil) != c.err {
			t.Errorf("MatchMedia(%q) err = %v, wantErr %v", c.key, err, c.err)
			continue
		}
		if err == nil && got != c.match {
			t.Errorf("MatchMedia(%q) = %v, want %v", c.key, got, c.match)
		}
	}
}

func TestApplyMedia(t *testing.T) {
	style := map[string]any{
		"fontSize":                      12.0,
		"color":                         "black",
		"@media max-width: 400":         map[string]any{"fontSize": 10.0, "color": "red"},
		"@media orientation: landscape": map[string]any{"color": "blue"},
	}
	// Narrow portrait page: only the max-width block matches.
	out := ApplyMedia(style, MediaContext{Width: 400, Height: 800, Orientation: "portrait"})
	if out["fontSize"] != 10.0 || out["color"] != "red" {
		t.Errorf("narrow portrait = %v", out)
	}
	for k := range out {
		if IsMediaKey(k) {
			t.Errorf("@media key %q leaked into output", k)
		}
	}
	// Wide landscape page: only the orientation block matches.
	out = ApplyMedia(style, MediaContext{Width: 900, Height: 500, Orientation: "landscape"})
	if out["fontSize"] != 12.0 || out["color"] != "blue" {
		t.Errorf("wide landscape = %v", out)
	}
}

func TestExpandMarginPadding(t *testing.T) {
	// Box shorthand with explicit-longhand override.
	out := ExpandShorthands(map[string]any{
		"margin":     "10 20 30",
		"marginLeft": 5.0,
		"padding":    8.0,
	})
	if out["marginTop"] != "10" || out["marginRight"] != "20" || out["marginBottom"] != "30" {
		t.Errorf("margin box = %v", out)
	}
	if out["marginLeft"] != 5.0 {
		t.Errorf("explicit marginLeft should win, got %v", out["marginLeft"])
	}
	if out["paddingTop"] != 8.0 || out["paddingLeft"] != 8.0 {
		t.Errorf("padding single = %v", out)
	}
	// Shorthand keys are consumed.
	if _, ok := out["margin"]; ok {
		t.Errorf("margin shorthand leaked")
	}

	// Horizontal/Vertical.
	out = ExpandShorthands(map[string]any{"marginHorizontal": 4.0, "marginVertical": 6.0})
	if out["marginLeft"] != 4.0 || out["marginRight"] != 4.0 || out["marginTop"] != 6.0 || out["marginBottom"] != 6.0 {
		t.Errorf("margin H/V = %v", out)
	}
}

func TestExpandBorder(t *testing.T) {
	out := ExpandShorthands(map[string]any{"border": "1pt solid red"})
	for _, side := range []string{"Top", "Right", "Bottom", "Left"} {
		if out["border"+side+"Width"] != "1pt" || out["border"+side+"Style"] != "solid" || out["border"+side+"Color"] != "red" {
			t.Errorf("border %s = w:%v s:%v c:%v", side, out["border"+side+"Width"], out["border"+side+"Style"], out["border"+side+"Color"])
		}
	}

	// Functional color survives, and a per-side override wins.
	out = ExpandShorthands(map[string]any{
		"border":            "2 dashed rgb(1, 2, 3)",
		"borderTopColor":    "black",
		"borderBottomWidth": 4.0,
	})
	if out["borderRightColor"] != "rgb(1, 2, 3)" {
		t.Errorf("functional color = %v", out["borderRightColor"])
	}
	if out["borderTopColor"] != "black" {
		t.Errorf("per-side color override = %v", out["borderTopColor"])
	}
	if out["borderBottomWidth"] != 4.0 {
		t.Errorf("per-side width override = %v", out["borderBottomWidth"])
	}

	// Generic borderWidth applies to all sides.
	out = ExpandShorthands(map[string]any{"borderWidth": 3.0})
	if out["borderTopWidth"] != 3.0 || out["borderLeftWidth"] != 3.0 {
		t.Errorf("borderWidth = %v", out)
	}
}

func TestExpandRadiusAndGap(t *testing.T) {
	out := ExpandShorthands(map[string]any{"borderRadius": 6.0, "borderTopLeftRadius": 2.0})
	if out["borderTopRightRadius"] != 6.0 || out["borderBottomLeftRadius"] != 6.0 {
		t.Errorf("radius all-corners = %v", out)
	}
	if out["borderTopLeftRadius"] != 2.0 {
		t.Errorf("per-corner radius override = %v", out["borderTopLeftRadius"])
	}

	out = ExpandShorthands(map[string]any{"gap": "4 8"})
	if out["rowGap"] != "4" || out["columnGap"] != "8" {
		t.Errorf("gap two-value = %v", out)
	}
	out = ExpandShorthands(map[string]any{"gap": 5.0})
	if out["rowGap"] != 5.0 || out["columnGap"] != 5.0 {
		t.Errorf("gap single = %v", out)
	}
}

func TestExpandPassThrough(t *testing.T) {
	in := map[string]any{"flexDirection": "row", "color": "blue", "margin": 10.0}
	out := ExpandShorthands(in)
	if out["flexDirection"] != "row" || out["color"] != "blue" {
		t.Errorf("non-shorthand props should pass through: %v", out)
	}
	// Input map is not mutated.
	if _, ok := in["marginTop"]; ok {
		t.Errorf("ExpandShorthands mutated its input")
	}
}

func TestBoxSidesRules(t *testing.T) {
	cases := []struct {
		in   any
		want [4]any
	}{
		{"5", [4]any{"5", "5", "5", "5"}},
		{"5 10", [4]any{"5", "10", "5", "10"}},
		{"5 10 15", [4]any{"5", "10", "15", "10"}},
		{"5 10 15 20", [4]any{"5", "10", "15", "20"}},
		{7.0, [4]any{7.0, 7.0, 7.0, 7.0}},
	}
	for _, c := range cases {
		if got := boxSides(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("boxSides(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
