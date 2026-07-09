package layout

import (
	"encoding/json"
	"strings"

	"github.com/swish/feast/internal/stylesheet"
	"github.com/swish/feast/internal/tree"
)

// pageSizes lists named page sizes in points (portrait orientation). A subset of
// react-pdf's 47 named sizes; the B/C/RA/SRA series can be added as needed.
var pageSizes = map[string][2]float64{
	"A0": {2383.94, 3370.39},
	"A1": {1683.78, 2383.94},
	"A2": {1190.55, 1683.78},
	"A3": {841.89, 1190.55},
	"A4": {595.28, 841.89},
	"A5": {419.53, 595.28},
	"A6": {297.64, 419.53},
	"A7": {209.76, 297.64},
	"A8": {147.40, 209.76},

	"LETTER":    {612, 792},
	"LEGAL":     {612, 1008},
	"TABLOID":   {792, 1224},
	"EXECUTIVE": {521.86, 756},
	"FOLIO":     {612, 936},
}

const (
	defaultPageW = 595.28 // A4
	defaultPageH = 841.89
)

// resolvePageSize resolves a Page node's size and orientation to point
// dimensions, defaulting to A4 portrait.
func resolvePageSize(node *tree.Node, ctx stylesheet.Context) (w, h float64, orientation string) {
	w, h = defaultPageW, defaultPageH
	if node.Props != nil {
		if sz, ok := node.Props["size"]; ok {
			if pw, ph, ok := parseSize(sz, ctx); ok {
				w, h = pw, ph
			}
		}
	}

	orientation = "portrait"
	if node.Props != nil {
		if o, ok := node.Props["orientation"].(string); ok && strings.HasPrefix(strings.ToLower(o), "land") {
			orientation = "landscape"
		}
	}
	if orientation == "landscape" && w < h {
		w, h = h, w
	}
	return w, h, orientation
}

func parseSize(sz any, ctx stylesheet.Context) (float64, float64, bool) {
	switch v := sz.(type) {
	case string:
		if d, ok := pageSizes[strings.ToUpper(strings.TrimSpace(v))]; ok {
			return d[0], d[1], true
		}
		// A unit string like "210mm" is not a named size; treat as square.
		if f := numPt(v, ctx); f > 0 {
			return f, f, true
		}
		return 0, 0, false
	case json.Number, float64, int:
		f := numPt(v, ctx)
		return f, f, true
	case []any:
		switch len(v) {
		case 1:
			f := numPt(v[0], ctx)
			return f, f, true
		case 0:
			return 0, 0, false
		default:
			return numPt(v[0], ctx), numPt(v[1], ctx), true
		}
	case map[string]any:
		w := numPt(v["width"], ctx)
		h := numPt(v["height"], ctx)
		if w > 0 && h > 0 {
			return w, h, true
		}
		// Height-omitted (auto-grow) pages are not yet supported; fall back.
		return 0, 0, false
	}
	return 0, 0, false
}

// numPt resolves any style scalar (number, json.Number, or unit string) to points.
func numPt(v any, ctx stylesheet.Context) float64 {
	if v == nil {
		return 0
	}
	val, err := stylesheet.ParseValue(v)
	if err != nil {
		return 0
	}
	return val.Resolve(ctx, 0)
}
