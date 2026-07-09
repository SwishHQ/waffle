package stylesheet

import "testing"

func TestInheritBasics(t *testing.T) {
	parent := map[string]any{
		"color":    "red",
		"fontSize": 12.0,
		"width":    100.0, // not inheritable
		"margin":   5.0,   // not inheritable
	}
	child := map[string]any{"fontSize": 14.0}

	out := Inherit(parent, child, false)

	if out["color"] != "red" {
		t.Errorf("color should be inherited, got %v", out["color"])
	}
	if out["fontSize"] != 14.0 {
		t.Errorf("child's own fontSize should win, got %v", out["fontSize"])
	}
	if _, ok := out["width"]; ok {
		t.Errorf("width must not be inherited")
	}
	if _, ok := out["margin"]; ok {
		t.Errorf("margin must not be inherited")
	}
}

func TestInheritTextOnly(t *testing.T) {
	parent := map[string]any{"backgroundColor": "yellow"}

	if out := Inherit(parent, map[string]any{}, false); out["backgroundColor"] != nil {
		t.Errorf("backgroundColor should not inherit on non-Text nodes, got %v", out["backgroundColor"])
	}
	if out := Inherit(parent, map[string]any{}, true); out["backgroundColor"] != "yellow" {
		t.Errorf("backgroundColor should inherit on Text nodes, got %v", out["backgroundColor"])
	}
}

func TestInheritTextDecorationMerge(t *testing.T) {
	cases := []struct {
		name          string
		parent, child any
		want          any // nil means the key should be absent
	}{
		{"child inherits parent", "underline", nil, "underline"},
		{"merge combines", "line-through", "underline", "underline line-through"},
		{"child none suppresses", "underline", "none", "none"},
		{"child only", nil, "underline", "underline"},
		{"neither", nil, nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			parent := map[string]any{}
			if c.parent != nil {
				parent["textDecoration"] = c.parent
			}
			child := map[string]any{}
			if c.child != nil {
				child["textDecoration"] = c.child
			}
			out := Inherit(parent, child, false)
			got, present := out["textDecoration"]
			if c.want == nil {
				if present {
					t.Errorf("textDecoration should be absent, got %v", got)
				}
				return
			}
			// Order within the merged set is parent-then-child; check as a set.
			if !sameDecoration(got, c.want) {
				t.Errorf("textDecoration = %v, want %v", got, c.want)
			}
		})
	}
}

func sameDecoration(a, b any) bool {
	as, bs := decorationTokens(a), decorationTokens(b)
	if len(as) != len(bs) {
		return false
	}
	seen := map[string]bool{}
	for _, t := range as {
		seen[t] = true
	}
	for _, t := range bs {
		if !seen[t] {
			return false
		}
	}
	return true
}

func TestInheritNilParent(t *testing.T) {
	child := map[string]any{"color": "blue"}
	out := Inherit(nil, child, false)
	if out["color"] != "blue" {
		t.Errorf("nil parent should return child unchanged, got %v", out)
	}
}

func TestResolvePipeline(t *testing.T) {
	// Array (later wins) + a matching media block + shorthand expansion.
	style := []any{
		map[string]any{"margin": 10.0, "fontSize": 12.0},
		map[string]any{
			"@media max-width: 400": map[string]any{"fontSize": 9.0},
			"border":                "1pt solid black",
		},
	}
	out := Resolve(style, MediaContext{Width: 400, Height: 800, Orientation: "portrait"})

	if out["fontSize"] != 9.0 {
		t.Errorf("media override fontSize = %v, want 9", out["fontSize"])
	}
	if out["marginTop"] != 10.0 || out["marginLeft"] != 10.0 {
		t.Errorf("margin should be expanded to sides, got top=%v left=%v", out["marginTop"], out["marginLeft"])
	}
	if out["borderTopWidth"] != "1pt" || out["borderTopColor"] != "black" {
		t.Errorf("border should be expanded, got %v", out)
	}
	// Shorthands and @media keys are consumed.
	for _, k := range []string{"margin", "border", "@media max-width: 400"} {
		if _, ok := out[k]; ok {
			t.Errorf("key %q should have been consumed", k)
		}
	}
}
