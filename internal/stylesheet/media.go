package stylesheet

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const mediaPrefix = "@media"

// MediaContext is the environment a media query is evaluated against: the page
// dimensions (in points) and orientation.
type MediaContext struct {
	Width       float64
	Height      float64
	Orientation string // "portrait" or "landscape"
}

// IsMediaKey reports whether a style key is an @media block.
func IsMediaKey(k string) bool {
	return strings.HasPrefix(strings.TrimSpace(strings.ToLower(k)), mediaPrefix)
}

var mediaAndRe = regexp.MustCompile(`(?i)\s+and\s+`)

// MatchMedia evaluates a media query key such as
// "@media max-width: 400 and orientation: portrait" against ctx. A bare "@media"
// matches everything. Supported features: min-width, max-width, min-height,
// max-height, orientation.
func MatchMedia(key string, ctx MediaContext) (bool, error) {
	q := strings.TrimSpace(key)
	if !strings.HasPrefix(strings.ToLower(q), mediaPrefix) {
		return false, fmt.Errorf("stylesheet: not a media query: %q", key)
	}
	q = strings.TrimSpace(q[len(mediaPrefix):])
	if q == "" {
		return true, nil
	}
	for _, cond := range mediaAndRe.Split(q, -1) {
		ok, err := matchMediaCondition(cond, ctx)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func matchMediaCondition(cond string, ctx MediaContext) (bool, error) {
	cond = strings.TrimSpace(strings.Trim(strings.TrimSpace(cond), "()"))
	feature, value, found := strings.Cut(cond, ":")
	if !found {
		return false, fmt.Errorf("stylesheet: malformed media condition %q", cond)
	}
	feature = strings.ToLower(strings.TrimSpace(feature))
	value = strings.TrimSpace(value)

	switch feature {
	case "orientation":
		return strings.EqualFold(value, ctx.Orientation), nil
	case "min-width", "max-width", "min-height", "max-height":
		n, err := mediaNumber(value)
		if err != nil {
			return false, err
		}
		switch feature {
		case "min-width":
			return ctx.Width >= n, nil
		case "max-width":
			return ctx.Width <= n, nil
		case "min-height":
			return ctx.Height >= n, nil
		default: // max-height
			return ctx.Height <= n, nil
		}
	default:
		return false, fmt.Errorf("stylesheet: unknown media feature %q", feature)
	}
}

func mediaNumber(s string) (float64, error) {
	v, err := ParseValue(s)
	if err != nil {
		return 0, err
	}
	if v.IsRelative() || v.IsAuto() {
		return 0, fmt.Errorf("stylesheet: media value must be absolute: %q", s)
	}
	return v.Resolve(Context{}, 0), nil
}

// ApplyMedia merges matching @media blocks into the base style and drops the
// @media keys. Base props apply first; matching blocks are then applied in sorted
// key order (deterministic) so a later block overrides an earlier one. The input
// map is not modified.
func ApplyMedia(style map[string]any, ctx MediaContext) map[string]any {
	out := make(map[string]any, len(style))
	var mediaKeys []string
	for k, v := range style {
		if IsMediaKey(k) {
			mediaKeys = append(mediaKeys, k)
			continue
		}
		out[k] = v
	}
	sort.Strings(mediaKeys)
	for _, k := range mediaKeys {
		match, err := MatchMedia(k, ctx)
		if err != nil || !match {
			continue
		}
		if block, ok := style[k].(map[string]any); ok {
			for bk, bv := range block {
				out[bk] = bv
			}
		}
	}
	return out
}
