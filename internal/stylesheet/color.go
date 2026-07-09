package stylesheet

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Color is an RGBA color. R, G, B are 0–255; A is 0–1 (1 = opaque).
type Color struct {
	R, G, B uint8
	A       float64
}

// RGB returns the color's components normalized to 0–1, ignoring alpha.
func (c Color) RGB() (r, g, b float64) {
	return float64(c.R) / 255, float64(c.G) / 255, float64(c.B) / 255
}

// ParseColor parses a CSS color string: #hex (3/4/6/8 digits), rgb()/rgba(),
// hsl()/hsla(), a named color, or "transparent".
func ParseColor(s string) (Color, error) {
	t := strings.TrimSpace(strings.ToLower(s))
	if t == "" {
		return Color{}, fmt.Errorf("stylesheet: empty color")
	}
	if t == "transparent" {
		return Color{A: 0}, nil
	}
	if strings.HasPrefix(t, "#") {
		return parseHexColor(t)
	}
	if strings.HasPrefix(t, "rgb") {
		return parseRGBColor(t)
	}
	if strings.HasPrefix(t, "hsl") {
		return parseHSLColor(t)
	}
	if hex, ok := namedColors[t]; ok {
		return parseHexColor(hex)
	}
	return Color{}, fmt.Errorf("stylesheet: unrecognized color %q", s)
}

func parseHexColor(s string) (Color, error) {
	h := strings.TrimPrefix(s, "#")
	expand := func(b byte) uint8 {
		v := hexNibble(b)
		return v<<4 | v
	}
	switch len(h) {
	case 3: // #rgb
		return Color{expand(h[0]), expand(h[1]), expand(h[2]), 1}, nil
	case 4: // #rgba
		return Color{expand(h[0]), expand(h[1]), expand(h[2]), float64(expand(h[3])) / 255}, nil
	case 6: // #rrggbb
		return Color{hexByte(h[0:2]), hexByte(h[2:4]), hexByte(h[4:6]), 1}, nil
	case 8: // #rrggbbaa
		return Color{hexByte(h[0:2]), hexByte(h[2:4]), hexByte(h[4:6]), float64(hexByte(h[6:8])) / 255}, nil
	}
	return Color{}, fmt.Errorf("stylesheet: invalid hex color %q", s)
}

func parseRGBColor(s string) (Color, error) {
	args, err := colorArgs(s)
	if err != nil {
		return Color{}, err
	}
	if len(args) != 3 && len(args) != 4 {
		return Color{}, fmt.Errorf("stylesheet: rgb() needs 3 or 4 components: %q", s)
	}
	comp := func(a string) uint8 {
		a = strings.TrimSpace(a)
		if strings.HasSuffix(a, "%") {
			f, _ := strconv.ParseFloat(strings.TrimSuffix(a, "%"), 64)
			return clamp8(f * 255 / 100)
		}
		f, _ := strconv.ParseFloat(a, 64)
		return clamp8(f)
	}
	c := Color{R: comp(args[0]), G: comp(args[1]), B: comp(args[2]), A: 1}
	if len(args) == 4 {
		c.A = parseAlpha(args[3])
	}
	return c, nil
}

func parseHSLColor(s string) (Color, error) {
	args, err := colorArgs(s)
	if err != nil {
		return Color{}, err
	}
	if len(args) != 3 && len(args) != 4 {
		return Color{}, fmt.Errorf("stylesheet: hsl() needs 3 or 4 components: %q", s)
	}
	h, _ := strconv.ParseFloat(strings.TrimSpace(args[0]), 64)
	sPct, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(args[1]), "%"), 64)
	lPct, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(args[2]), "%"), 64)
	r, g, b := hslToRGB(h, sPct/100, lPct/100)
	c := Color{R: r, G: g, B: b, A: 1}
	if len(args) == 4 {
		c.A = parseAlpha(args[3])
	}
	return c, nil
}

// colorArgs extracts the comma-separated arguments inside the parentheses of a
// functional color notation.
func colorArgs(s string) ([]string, error) {
	open := strings.IndexByte(s, '(')
	close := strings.LastIndexByte(s, ')')
	if open < 0 || close < 0 || close < open {
		return nil, fmt.Errorf("stylesheet: malformed color %q", s)
	}
	inner := s[open+1 : close]
	return strings.Split(inner, ","), nil
}

func parseAlpha(a string) float64 {
	a = strings.TrimSpace(a)
	if strings.HasSuffix(a, "%") {
		f, _ := strconv.ParseFloat(strings.TrimSuffix(a, "%"), 64)
		return clampUnit(f / 100)
	}
	f, _ := strconv.ParseFloat(a, 64)
	return clampUnit(f)
}

// hslToRGB converts HSL (h in degrees, s and l in 0–1) to 8-bit RGB.
func hslToRGB(h, s, l float64) (uint8, uint8, uint8) {
	h = math.Mod(math.Mod(h, 360)+360, 360) / 360
	if s == 0 {
		v := clamp8(l * 255)
		return v, v, v
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	r := hueToRGB(p, q, h+1.0/3.0)
	g := hueToRGB(p, q, h)
	b := hueToRGB(p, q, h-1.0/3.0)
	return clamp8(r * 255), clamp8(g * 255), clamp8(b * 255)
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 1.0/2.0:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	}
	return p
}

func hexNibble(b byte) uint8 {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10
	}
	return 0
}

func hexByte(s string) uint8 {
	return hexNibble(s[0])<<4 | hexNibble(s[1])
}

func clamp8(f float64) uint8 {
	if f < 0 {
		return 0
	}
	if f > 255 {
		return 255
	}
	return uint8(math.Round(f))
}

func clampUnit(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}
