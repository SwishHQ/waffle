package layout

import (
	"strconv"
	"strings"

	"github.com/swish/waffle/internal/tree"
)

// svgViewBoxSize returns the intrinsic size of an Svg node: its viewBox width and
// height, falling back to the width/height props.
func svgViewBoxSize(node *tree.Node) (float64, float64) {
	if node.Props == nil {
		return 0, 0
	}
	if vb, ok := node.Props["viewBox"].(string); ok {
		f := strings.Fields(strings.ReplaceAll(vb, ",", " "))
		if len(f) == 4 {
			w, _ := strconv.ParseFloat(f[2], 64)
			h, _ := strconv.ParseFloat(f[3], 64)
			return w, h
		}
	}
	w, _ := num(node.Props["width"])
	h, _ := num(node.Props["height"])
	return w, h
}
