package transform

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Parse parses a CSS transform string such as
//
//	"rotate(45deg) translate(10, 5px) scale(1.5)"
//
// into a single composed matrix. Functions are read left to right and composed
// so the composed matrix is M = f1 ∘ f2 ∘ ... : the FIRST (leftmost) function is
// the outermost and is applied to points LAST, matching CSS. Equivalently, the
// last function listed transforms the point first.
//
// Supported functions: translate/translateX/translateY, scale/scaleX/scaleY,
// rotate, skew/skewX/skewY, and matrix(a,b,c,d,e,f). Function names are matched
// case-insensitively. Lengths accept px, pt, or a bare number, all treated as
// PDF points (at waffle's default 72 dpi, 1px == 1pt); percentages are NOT
// supported and produce an error. Angles accept deg (the default for a bare
// number), rad, grad, and turn.
//
// Parse returns Identity for the empty string. Unknown or unsupported functions
// (e.g. translate3d, perspective) are ignored. Malformed input — a bad number,
// a wrong argument count for a known function, a percentage length, or broken
// syntax such as an unclosed parenthesis — returns (Identity, error). Parse
// never panics.
func Parse(s string) (Matrix, error) {
	m := Identity()
	i := 0
	for i < len(s) {
		for i < len(s) && (isSpace(s[i]) || s[i] == ',') {
			i++
		}
		if i >= len(s) {
			break
		}
		start := i
		for i < len(s) && isNameByte(s[i]) {
			i++
		}
		name := s[start:i]
		if name == "" {
			return Identity(), fmt.Errorf("transform: unexpected %q in %q", string(s[i]), s)
		}
		if i >= len(s) || s[i] != '(' {
			return Identity(), fmt.Errorf("transform: expected '(' after %q in %q", name, s)
		}
		i++ // consume '('
		argStart := i
		for i < len(s) && s[i] != ')' {
			i++
		}
		if i >= len(s) {
			return Identity(), fmt.Errorf("transform: unclosed '(' for %q in %q", name, s)
		}
		args := splitArgs(s[argStart:i])
		i++ // consume ')'

		f, err := parseFunc(name, args)
		if err != nil {
			return Identity(), err
		}
		m = m.Mul(f)
	}
	return m, nil
}

// parseFunc builds the matrix for a single transform function. Unknown names
// yield Identity (ignored). Known names validate their argument count.
func parseFunc(name string, args []string) (Matrix, error) {
	switch strings.ToLower(name) {
	case "translate":
		if len(args) != 1 && len(args) != 2 {
			return Identity(), argCountErr(name, "1 or 2", args)
		}
		tx, err := parseLength(args[0])
		if err != nil {
			return Identity(), err
		}
		ty := 0.0
		if len(args) == 2 {
			if ty, err = parseLength(args[1]); err != nil {
				return Identity(), err
			}
		}
		return Translate(tx, ty), nil

	case "translatex":
		tx, err := parseSingleLength(name, args)
		if err != nil {
			return Identity(), err
		}
		return Translate(tx, 0), nil

	case "translatey":
		ty, err := parseSingleLength(name, args)
		if err != nil {
			return Identity(), err
		}
		return Translate(0, ty), nil

	case "scale":
		if len(args) != 1 && len(args) != 2 {
			return Identity(), argCountErr(name, "1 or 2", args)
		}
		sx, err := parseNumber(args[0])
		if err != nil {
			return Identity(), err
		}
		sy := sx
		if len(args) == 2 {
			if sy, err = parseNumber(args[1]); err != nil {
				return Identity(), err
			}
		}
		return Scale(sx, sy), nil

	case "scalex":
		sx, err := parseSingleNumber(name, args)
		if err != nil {
			return Identity(), err
		}
		return Scale(sx, 1), nil

	case "scaley":
		sy, err := parseSingleNumber(name, args)
		if err != nil {
			return Identity(), err
		}
		return Scale(1, sy), nil

	case "rotate":
		if len(args) != 1 {
			return Identity(), argCountErr(name, "1", args)
		}
		rad, err := ParseAngle(args[0])
		if err != nil {
			return Identity(), err
		}
		return rotateRad(rad), nil

	case "skew":
		if len(args) != 1 && len(args) != 2 {
			return Identity(), argCountErr(name, "1 or 2", args)
		}
		ax, err := ParseAngle(args[0])
		if err != nil {
			return Identity(), err
		}
		ay := 0.0
		if len(args) == 2 {
			if ay, err = ParseAngle(args[1]); err != nil {
				return Identity(), err
			}
		}
		return skewRad(ax, ay), nil

	case "skewx":
		if len(args) != 1 {
			return Identity(), argCountErr(name, "1", args)
		}
		ax, err := ParseAngle(args[0])
		if err != nil {
			return Identity(), err
		}
		return skewRad(ax, 0), nil

	case "skewy":
		if len(args) != 1 {
			return Identity(), argCountErr(name, "1", args)
		}
		ay, err := ParseAngle(args[0])
		if err != nil {
			return Identity(), err
		}
		return skewRad(0, ay), nil

	case "matrix":
		if len(args) != 6 {
			return Identity(), argCountErr(name, "6", args)
		}
		var v [6]float64
		for k, a := range args {
			n, err := parseNumber(a)
			if err != nil {
				return Identity(), err
			}
			v[k] = n
		}
		return Matrix{A: v[0], B: v[1], C: v[2], D: v[3], E: v[4], F: v[5]}, nil

	default:
		// Unknown / unsupported function: ignore (identity).
		return Identity(), nil
	}
}

// ParseAngle parses a CSS angle token and returns the angle in radians. It
// accepts the units deg, grad, rad, and turn; a bare number is interpreted as
// degrees. It is the shared angle helper used by rotate and skew parsing.
func ParseAngle(s string) (float64, error) {
	t := strings.TrimSpace(s)
	switch {
	case hasSuffixFold(t, "grad"):
		v, err := parseNumber(t[:len(t)-4])
		if err != nil {
			return 0, err
		}
		return v * math.Pi / 200, nil
	case hasSuffixFold(t, "turn"):
		v, err := parseNumber(t[:len(t)-4])
		if err != nil {
			return 0, err
		}
		return v * 2 * math.Pi, nil
	case hasSuffixFold(t, "deg"):
		v, err := parseNumber(t[:len(t)-3])
		if err != nil {
			return 0, err
		}
		return v * math.Pi / 180, nil
	case hasSuffixFold(t, "rad"):
		v, err := parseNumber(t[:len(t)-3])
		if err != nil {
			return 0, err
		}
		return v, nil
	default:
		// Bare number: treat as degrees.
		v, err := parseNumber(t)
		if err != nil {
			return 0, fmt.Errorf("transform: invalid angle %q", s)
		}
		return v * math.Pi / 180, nil
	}
}

// parseLength parses a length token (px, pt, or a bare number) into PDF points.
// Percentages are not supported and produce an error.
func parseLength(s string) (float64, error) {
	t := strings.TrimSpace(s)
	if strings.HasSuffix(t, "%") {
		return 0, fmt.Errorf("transform: percentage length %q not supported", s)
	}
	if hasSuffixFold(t, "px") || hasSuffixFold(t, "pt") {
		t = t[:len(t)-2]
	}
	return parseNumber(t)
}

// parseNumber parses a plain float token.
func parseNumber(s string) (float64, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("transform: invalid number %q", s)
	}
	return v, nil
}

func parseSingleLength(name string, args []string) (float64, error) {
	if len(args) != 1 {
		return 0, argCountErr(name, "1", args)
	}
	return parseLength(args[0])
}

func parseSingleNumber(name string, args []string) (float64, error) {
	if len(args) != 1 {
		return 0, argCountErr(name, "1", args)
	}
	return parseNumber(args[0])
}

func argCountErr(name, want string, args []string) error {
	return fmt.Errorf("transform: %s expects %s argument(s), got %d", name, want, len(args))
}

// splitArgs splits an argument list on commas and whitespace, dropping empties.
func splitArgs(s string) []string {
	return strings.Fields(strings.ReplaceAll(s, ",", " "))
}

func hasSuffixFold(s, suffix string) bool {
	return len(s) >= len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix)
}

func isSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f' }

// isNameByte reports whether b can appear in a transform function name. CSS
// transform functions are letters plus a trailing digit for the 3d variants
// (translate3d, matrix3d, ...), which we read as one token and then ignore.
func isNameByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}
