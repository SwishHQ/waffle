// Package stylesheet parses react-pdf-shaped style values into typed, resolved
// forms for the layout engine: dimensioned values and their unit conversions,
// colors, font weights, and style flattening. Values arrive from the contract as
// numbers (json.Number) or CSS-ish strings and are interpreted here, keeping the
// engine the single source of truth for style semantics.
package stylesheet

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Unit is a length unit. The zero value is points, react-pdf's default.
type Unit int

const (
	UnitPt Unit = iota // points (default for bare numbers)
	UnitIn
	UnitMm
	UnitCm
	UnitPx
	UnitRem
	UnitPercent
	UnitVw
	UnitVh
	UnitAuto
)

// Value is a dimensioned style value, e.g. 10pt, 50%, 2in, or auto.
type Value struct {
	Amount float64
	Unit   Unit
}

// Auto is the "auto" value.
var Auto = Value{Unit: UnitAuto}

// Context carries the resolution environment for units. Zero fields fall back to
// react-pdf's defaults via Context.withDefaults.
type Context struct {
	DPI     float64 // device pixels per inch for px; default 72
	RemBase float64 // point size of 1rem; default 18
	PageW   float64 // page width in points, for vw
	PageH   float64 // page height in points, for vh
}

func (c Context) withDefaults() Context {
	if c.DPI == 0 {
		c.DPI = 72
	}
	if c.RemBase == 0 {
		c.RemBase = 18
	}
	return c
}

var valueRe = regexp.MustCompile(`^\s*(-?(?:\d+\.?\d*|\.\d+))\s*(pt|in|mm|cm|px|rem|vw|vh|%)?\s*$`)

// ParseValue parses a style value from a number or a CSS-ish string.
func ParseValue(v any) (Value, error) {
	switch t := v.(type) {
	case nil:
		return Value{}, fmt.Errorf("stylesheet: nil value")
	case float64:
		return Value{Amount: t, Unit: UnitPt}, nil
	case int:
		return Value{Amount: float64(t), Unit: UnitPt}, nil
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return Value{}, fmt.Errorf("stylesheet: bad number %q: %w", t.String(), err)
		}
		return Value{Amount: f, Unit: UnitPt}, nil
	case string:
		return parseValueString(t)
	default:
		return Value{}, fmt.Errorf("stylesheet: cannot parse value of type %T", v)
	}
}

func parseValueString(s string) (Value, error) {
	if strings.EqualFold(strings.TrimSpace(s), "auto") {
		return Auto, nil
	}
	m := valueRe.FindStringSubmatch(s)
	if m == nil {
		return Value{}, fmt.Errorf("stylesheet: invalid value %q", s)
	}
	amount, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return Value{}, fmt.Errorf("stylesheet: invalid number in %q: %w", s, err)
	}
	unit := UnitPt
	switch m[2] {
	case "", "pt":
		unit = UnitPt
	case "in":
		unit = UnitIn
	case "mm":
		unit = UnitMm
	case "cm":
		unit = UnitCm
	case "px":
		unit = UnitPx
	case "rem":
		unit = UnitRem
	case "vw":
		unit = UnitVw
	case "vh":
		unit = UnitVh
	case "%":
		unit = UnitPercent
	}
	return Value{Amount: amount, Unit: unit}, nil
}

// IsRelative reports whether resolving the value needs a layout-time basis
// (percentages) or page dimensions (vw/vh).
func (v Value) IsRelative() bool {
	switch v.Unit {
	case UnitPercent, UnitVw, UnitVh:
		return true
	}
	return false
}

// IsAuto reports whether the value is "auto".
func (v Value) IsAuto() bool { return v.Unit == UnitAuto }

// ToPoints converts an absolute value to points. It returns ok=false for auto and
// for relative units (percent/vw/vh), which require Resolve with a basis.
func (v Value) ToPoints(ctx Context) (float64, bool) {
	if v.IsAuto() || v.IsRelative() {
		return 0, false
	}
	return v.Resolve(ctx, 0), true
}

// Resolve converts the value to points. basis is the reference length for
// percentages (e.g. the parent's content width); vw/vh use the page dimensions
// from ctx. Auto resolves to 0.
//
// NOTE: px uses value*72/dpi (no rounding); verify against react-pdf's units.ts
// when wiring the full style pass.
func (v Value) Resolve(ctx Context, basis float64) float64 {
	ctx = ctx.withDefaults()
	switch v.Unit {
	case UnitPt:
		return v.Amount
	case UnitIn:
		return v.Amount * 72
	case UnitMm:
		return v.Amount * (72.0 / 25.4)
	case UnitCm:
		return v.Amount * (72.0 / 2.54)
	case UnitPx:
		return v.Amount * (72.0 / ctx.DPI)
	case UnitRem:
		return v.Amount * ctx.RemBase
	case UnitPercent:
		return basis * v.Amount / 100
	case UnitVw:
		return ctx.PageW * v.Amount / 100
	case UnitVh:
		return ctx.PageH * v.Amount / 100
	case UnitAuto:
		return 0
	}
	return v.Amount
}
