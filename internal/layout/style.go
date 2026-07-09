package layout

import (
	"encoding/json"

	"github.com/swish/feast/internal/flexbox"
	"github.com/swish/feast/internal/stylesheet"
)

// toFlexStyle maps a resolved+inherited style map to flexbox layout inputs,
// resolving absolute units (and vw/vh) to points. Percentages are not yet
// resolved by the flexbox engine and are treated as auto (a known limitation).
func toFlexStyle(r map[string]any, ctx stylesheet.Context) flexbox.Style {
	var s flexbox.Style

	switch str(r["flexDirection"]) {
	case "row", "row-reverse":
		s.Direction = flexbox.Row
	default: // column, column-reverse, unset
		s.Direction = flexbox.Column
	}

	switch str(r["justifyContent"]) {
	case "center":
		s.Justify = flexbox.JustifyCenter
	case "flex-end":
		s.Justify = flexbox.JustifyFlexEnd
	case "space-between":
		s.Justify = flexbox.JustifySpaceBetween
	case "space-around":
		s.Justify = flexbox.JustifySpaceAround
	case "space-evenly":
		s.Justify = flexbox.JustifySpaceEvenly
	default:
		s.Justify = flexbox.JustifyFlexStart
	}

	s.AlignItems = toAlign(str(r["alignItems"]))
	s.AlignSelf = toAlign(str(r["alignSelf"]))
	s.AlignContent = toAlign(str(r["alignContent"]))

	switch str(r["flexWrap"]) {
	case "wrap":
		s.Wrap = flexbox.WrapWrap
	case "wrap-reverse":
		s.Wrap = flexbox.WrapReverse
	default:
		s.Wrap = flexbox.WrapNoWrap
	}

	if f, ok := num(r["flexGrow"]); ok {
		s.Grow = f
	}
	if f, ok := num(r["flexShrink"]); ok {
		s.Shrink = f
	}
	if f, ok := num(r["flex"]); ok { // flex: N shorthand → grow N
		s.Grow = f
	}
	s.Basis = dim(r, "flexBasis", ctx)

	s.Width = dim(r, "width", ctx)
	s.Height = dim(r, "height", ctx)
	s.MinWidth = dim(r, "minWidth", ctx)
	s.MaxWidth = dim(r, "maxWidth", ctx)
	s.MinHeight = dim(r, "minHeight", ctx)
	s.MaxHeight = dim(r, "maxHeight", ctx)
	if ar, ok := num(r["aspectRatio"]); ok && ar > 0 {
		s.AspectRatio = ar
	}

	switch str(r["position"]) {
	case "absolute":
		s.Position = flexbox.PositionAbsolute
	case "relative":
		s.Position = flexbox.PositionRelative
	default:
		s.Position = flexbox.PositionStatic
	}
	s.Top = dim(r, "top", ctx)
	s.Right = dim(r, "right", ctx)
	s.Bottom = dim(r, "bottom", ctx)
	s.Left = dim(r, "left", ctx)

	s.MarginTop = lp(r, "marginTop", ctx)
	s.MarginRight = lp(r, "marginRight", ctx)
	s.MarginBottom = lp(r, "marginBottom", ctx)
	s.MarginLeft = lp(r, "marginLeft", ctx)

	s.PaddingTop = lp(r, "paddingTop", ctx)
	s.PaddingRight = lp(r, "paddingRight", ctx)
	s.PaddingBottom = lp(r, "paddingBottom", ctx)
	s.PaddingLeft = lp(r, "paddingLeft", ctx)

	s.BorderTop = lp(r, "borderTopWidth", ctx)
	s.BorderRight = lp(r, "borderRightWidth", ctx)
	s.BorderBottom = lp(r, "borderBottomWidth", ctx)
	s.BorderLeft = lp(r, "borderLeftWidth", ctx)

	rowGap := lp(r, "rowGap", ctx)
	colGap := lp(r, "columnGap", ctx)
	if s.Direction == flexbox.Row {
		s.Gap = colGap
	} else {
		s.Gap = rowGap
	}

	return s
}

func toAlign(s string) flexbox.Align {
	switch s {
	case "flex-start":
		return flexbox.AlignFlexStart
	case "center":
		return flexbox.AlignCenter
	case "flex-end":
		return flexbox.AlignFlexEnd
	case "stretch":
		return flexbox.AlignStretch
	case "space-between":
		return flexbox.AlignSpaceBetween
	case "space-around":
		return flexbox.AlignSpaceAround
	case "baseline":
		return flexbox.AlignFlexStart // baseline alignment not yet modeled
	default:
		return flexbox.AlignAuto
	}
}

// dim resolves a dimension property to a flexbox.Dim (auto when absent, relative,
// or unparseable).
func dim(r map[string]any, key string, ctx stylesheet.Context) flexbox.Dim {
	v, ok := r[key]
	if !ok {
		return flexbox.Auto()
	}
	val, err := stylesheet.ParseValue(v)
	if err != nil || val.IsAuto() {
		return flexbox.Auto()
	}
	if val.Unit == stylesheet.UnitPercent {
		return flexbox.Percent(val.Amount)
	}
	return flexbox.Pt(val.Resolve(ctx, 0))
}

// lp resolves a length property to points (0 when absent, relative, or auto).
func lp(r map[string]any, key string, ctx stylesheet.Context) float64 {
	v, ok := r[key]
	if !ok {
		return 0
	}
	val, err := stylesheet.ParseValue(v)
	if err != nil || val.IsAuto() || val.Unit == stylesheet.UnitPercent {
		return 0
	}
	return val.Resolve(ctx, 0)
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func num(v any) (float64, bool) {
	switch t := v.(type) {
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case float64:
		return t, true
	case int:
		return float64(t), true
	case string:
		val, err := stylesheet.ParseValue(t)
		if err != nil {
			return 0, false
		}
		return val.Amount, true
	}
	return 0, false
}
