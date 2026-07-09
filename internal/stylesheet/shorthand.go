package stylesheet

import "strings"

// ExpandShorthands rewrites CSS-style shorthands in a flattened style map into
// their longhand properties, returning a new map. Non-shorthand properties pass
// through unchanged. More specific inputs win over less specific ones (an
// explicit marginTop overrides margin; a per-side border overrides the border
// shorthand).
//
// Handled: margin/padding (+Horizontal/Vertical, 1–4 value box), border and its
// per-side forms (width/style/color, incl. "1pt solid red"), borderRadius
// (single value) and per-corner radii, and gap (row/column). flex and transform
// string shorthands are handled in their own passes and are left untouched here.
func ExpandShorthands(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		if !expandedKeys[k] {
			out[k] = v
		}
	}
	expandBox("margin", in, out)
	expandBox("padding", in, out)
	expandBorder(in, out)
	expandBorderRadius(in, out)
	expandGap(in, out)
	return out
}

var borderSides = []string{"Top", "Right", "Bottom", "Left"}
var radiusCorners = []string{"TopLeft", "TopRight", "BottomRight", "BottomLeft"}

var expandedKeys = func() map[string]bool {
	keys := []string{
		"margin", "marginTop", "marginRight", "marginBottom", "marginLeft", "marginHorizontal", "marginVertical",
		"padding", "paddingTop", "paddingRight", "paddingBottom", "paddingLeft", "paddingHorizontal", "paddingVertical",
		"border", "borderWidth", "borderColor", "borderStyle",
		"borderTop", "borderRight", "borderBottom", "borderLeft",
		"borderRadius",
		"gap", "rowGap", "columnGap",
	}
	for _, side := range borderSides {
		keys = append(keys, "border"+side+"Width", "border"+side+"Color", "border"+side+"Style")
	}
	for _, corner := range radiusCorners {
		keys = append(keys, "border"+corner+"Radius")
	}
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}()

// expandBox expands margin/padding: the box shorthand, the Horizontal/Vertical
// pair, and explicit per-side longhands, applied least- to most-specific.
func expandBox(prefix string, in, out map[string]any) {
	if v, ok := in[prefix]; ok {
		s := boxSides(v)
		out[prefix+"Top"], out[prefix+"Right"] = s[0], s[1]
		out[prefix+"Bottom"], out[prefix+"Left"] = s[2], s[3]
	}
	if v, ok := in[prefix+"Vertical"]; ok {
		out[prefix+"Top"], out[prefix+"Bottom"] = v, v
	}
	if v, ok := in[prefix+"Horizontal"]; ok {
		out[prefix+"Left"], out[prefix+"Right"] = v, v
	}
	for _, side := range borderSides {
		if v, ok := in[prefix+side]; ok {
			out[prefix+side] = v
		}
	}
}

func expandBorder(in, out map[string]any) {
	setSides := func(suffix string, val any) {
		if val == nil {
			return
		}
		for _, side := range borderSides {
			out["border"+side+suffix] = val
		}
	}

	// Generic border shorthand: "1pt solid red".
	if v, ok := in["border"].(string); ok {
		w, st, c := parseBorderShorthand(v)
		setSides("Width", w)
		setSides("Style", st)
		setSides("Color", c)
	}
	// Generic per-property values.
	if v, ok := in["borderWidth"]; ok {
		setSides("Width", v)
	}
	if v, ok := in["borderColor"]; ok {
		setSides("Color", v)
	}
	if v, ok := in["borderStyle"]; ok {
		setSides("Style", v)
	}
	// Per-side shorthand: borderTop: "1pt solid red".
	for _, side := range borderSides {
		if v, ok := in["border"+side].(string); ok {
			w, st, c := parseBorderShorthand(v)
			if w != nil {
				out["border"+side+"Width"] = w
			}
			if st != nil {
				out["border"+side+"Style"] = st
			}
			if c != nil {
				out["border"+side+"Color"] = c
			}
		}
	}
	// Explicit per-side longhands win.
	for _, side := range borderSides {
		for _, suffix := range []string{"Width", "Color", "Style"} {
			if v, ok := in["border"+side+suffix]; ok {
				out["border"+side+suffix] = v
			}
		}
	}
}

// expandBorderRadius supports a single-value borderRadius (all corners) plus
// explicit per-corner radii. Multi-value radius shorthands are not expanded yet.
func expandBorderRadius(in, out map[string]any) {
	if v, ok := in["borderRadius"]; ok {
		toks := valueTokens(v)
		if len(toks) > 0 {
			for _, corner := range radiusCorners {
				out["border"+corner+"Radius"] = toks[0]
			}
		}
	}
	for _, corner := range radiusCorners {
		if v, ok := in["border"+corner+"Radius"]; ok {
			out["border"+corner+"Radius"] = v
		}
	}
}

func expandGap(in, out map[string]any) {
	if v, ok := in["gap"]; ok {
		toks := valueTokens(v)
		switch {
		case len(toks) >= 2:
			out["rowGap"], out["columnGap"] = toks[0], toks[1]
		case len(toks) == 1:
			out["rowGap"], out["columnGap"] = toks[0], toks[0]
		}
	}
	if v, ok := in["rowGap"]; ok {
		out["rowGap"] = v
	}
	if v, ok := in["columnGap"]; ok {
		out["columnGap"] = v
	}
}

// boxSides expands a box value into [top, right, bottom, left] following CSS
// 1–4 value rules.
func boxSides(v any) [4]any {
	t := valueTokens(v)
	switch len(t) {
	case 1:
		return [4]any{t[0], t[0], t[0], t[0]}
	case 2:
		return [4]any{t[0], t[1], t[0], t[1]}
	case 3:
		return [4]any{t[0], t[1], t[2], t[1]}
	case 0:
		return [4]any{v, v, v, v}
	default:
		return [4]any{t[0], t[1], t[2], t[3]}
	}
}

// valueTokens splits a multi-value string ("10 20") into individual tokens; a
// non-string value is returned as a single token.
func valueTokens(v any) []any {
	if s, ok := v.(string); ok {
		fields := strings.Fields(s)
		out := make([]any, len(fields))
		for i, f := range fields {
			out[i] = f
		}
		return out
	}
	return []any{v}
}

// parseBorderShorthand parses "width style color" (the common order). Any part
// may be absent; a returned nil means "not specified". The color is the trailing
// remainder, preserving spaces (so functional colors like "rgb(1, 2, 3)" survive).
func parseBorderShorthand(s string) (width, style, color any) {
	fields := strings.Fields(strings.TrimSpace(s))
	i := 0
	if i < len(fields) && looksLikeValue(fields[i]) {
		width = fields[i]
		i++
	}
	if i < len(fields) && isBorderStyle(fields[i]) {
		style = fields[i]
		i++
	}
	if i < len(fields) {
		color = strings.Join(fields[i:], " ")
	}
	return width, style, color
}

func looksLikeValue(tok string) bool {
	return valueRe.MatchString(tok)
}

func isBorderStyle(tok string) bool {
	switch strings.ToLower(tok) {
	case "solid", "dashed", "dotted":
		return true
	}
	return false
}
