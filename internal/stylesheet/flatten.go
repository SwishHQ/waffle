package stylesheet

// Flatten merges a react-pdf style value into a single property map. A style may
// be a single object or an array of objects (and nested arrays); later entries
// win, matching react-pdf's `style={[a, b]}` semantics. nil and non-map leaves
// are ignored. The returned map is always non-nil and freshly allocated.
func Flatten(style any) map[string]any {
	out := make(map[string]any)
	mergeInto(out, style)
	return out
}

func mergeInto(dst map[string]any, style any) {
	switch t := style.(type) {
	case nil:
		// ignore
	case map[string]any:
		for k, v := range t {
			dst[k] = v
		}
	case []any:
		for _, item := range t {
			mergeInto(dst, item)
		}
	}
}
