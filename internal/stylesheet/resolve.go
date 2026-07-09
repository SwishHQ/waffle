package stylesheet

// Resolve turns a raw react-pdf style value into a flat map of longhand
// properties: it flattens object/array styles (later wins), merges matching
// @media blocks for the page context, then expands shorthands. Unit conversion
// (which needs a layout-time basis) and inheritance (which needs the parent) are
// applied later, by the caller.
func Resolve(style any, media MediaContext) map[string]any {
	return ExpandShorthands(ApplyMedia(Flatten(style), media))
}
