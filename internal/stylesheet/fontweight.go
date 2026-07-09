package stylesheet

import (
	"encoding/json"
	"strconv"
	"strings"
)

// fontWeightKeywords maps CSS font-weight keywords to numeric weights, matching
// react-pdf's mapping.
var fontWeightKeywords = map[string]int{
	"thin":       100,
	"hairline":   100,
	"ultralight": 200,
	"extralight": 200,
	"light":      300,
	"normal":     400,
	"regular":    400,
	"medium":     500,
	"semibold":   600,
	"demibold":   600,
	"bold":       700,
	"ultrabold":  800,
	"extrabold":  800,
	"heavy":      900,
	"black":      900,
}

// ParseFontWeight resolves a font weight from a number or a keyword string.
// A bare or "normal" weight is 400; "bold" is 700.
func ParseFontWeight(v any) (int, bool) {
	switch t := v.(type) {
	case nil:
		return 0, false
	case int:
		return t, true
	case float64:
		return int(t), true
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return int(n), true
		}
		if f, err := t.Float64(); err == nil {
			return int(f), true
		}
		return 0, false
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		if w, ok := fontWeightKeywords[s]; ok {
			return w, true
		}
		if n, err := strconv.Atoi(s); err == nil {
			return n, true
		}
		return 0, false
	default:
		return 0, false
	}
}
