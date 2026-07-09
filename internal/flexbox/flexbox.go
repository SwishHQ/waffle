// Package flexbox is feast's own flexbox layout engine — the replacement for
// react-pdf's Yoga binding. It computes box positions and sizes for a tree of
// nodes following CSS flexbox semantics (the subset react-pdf documents).
//
// Owning the engine (rather than binding an external Yoga port) keeps the
// dependency graph clean and gives the pagination step the re-layout control it
// needs. The core implementation covers flex-direction row/column, grow/shrink,
// justify-content, align-items/self (including stretch), the full box model
// (margin/padding/border), gap, percentages, aspect-ratio, absolute
// positioning, and flex-wrap (wrap/wrap-reverse) with align-content. Min/max
// constraints and reversed directions are layered on in later passes.
//
// All coordinates in Layout are border-box and relative to the parent's
// content-box origin, matching react-pdf's node model.
package flexbox

// Direction is the flex main-axis direction. The zero value is Column, matching
// react-pdf's default.
type Direction int

const (
	Column Direction = iota
	Row
)

// Justify is the main-axis distribution (justify-content).
type Justify int

const (
	JustifyFlexStart Justify = iota
	JustifyCenter
	JustifyFlexEnd
	JustifySpaceBetween
	JustifySpaceAround
	JustifySpaceEvenly
)

// Wrap controls whether flow children are laid out on a single main-axis line
// or may break onto multiple lines (flex-wrap). The zero value is NoWrap,
// matching react-pdf's default; WrapReverse wraps and reverses the order of
// the lines along the cross axis.
type Wrap int

const (
	WrapNoWrap Wrap = iota
	WrapWrap
	WrapReverse
)

// Position is a node's positioning scheme. Absolute takes the node out of flow
// and positions it via Top/Right/Bottom/Left against the parent's content box.
// Relative is currently treated as Static (in-flow) for layout.
type Position int

const (
	PositionStatic Position = iota
	PositionRelative
	PositionAbsolute
)

// Align is a cross-axis alignment (align-items / align-self / align-content).
// AlignAuto on a child defers to the container's align-items; AlignAuto on a
// container means stretch for align-items and flex-start for align-content.
// AlignSpaceBetween and AlignSpaceAround are meaningful only for align-content;
// elsewhere they fall back to flex-start.
type Align int

const (
	AlignAuto Align = iota
	AlignFlexStart
	AlignCenter
	AlignFlexEnd
	AlignStretch
	AlignSpaceBetween
	AlignSpaceAround
)

// DimKind distinguishes the kinds of a dimension value.
type DimKind uint8

const (
	DimAuto    DimKind = iota // the zero value: unset / "auto"
	DimPoint                  // a definite point value
	DimPercent                // a percentage of the parent's content size
)

// Dim is a dimension that may be a definite point value, a percentage, or auto.
// Its zero value is auto, so an unset Width/Height/Basis behaves like CSS "auto"
// rather than a definite 0.
type Dim struct {
	Value float64
	Kind  DimKind
}

// Pt returns a definite point dimension; Percent returns a percentage dimension;
// Auto returns the auto dimension (the zero value).
func Pt(v float64) Dim      { return Dim{Value: v, Kind: DimPoint} }
func Percent(v float64) Dim { return Dim{Value: v, Kind: DimPercent} }
func Auto() Dim             { return Dim{} }

// IsAuto reports whether the dimension is auto (undefined).
func (d Dim) IsAuto() bool { return d.Kind == DimAuto }

// resolve returns the dimension in points given the available reference size,
// and whether it was defined (false for auto).
func (d Dim) resolve(avail float64) (float64, bool) {
	switch d.Kind {
	case DimPoint:
		return d.Value, true
	case DimPercent:
		return d.Value / 100 * avail, true
	default:
		return 0, false
	}
}

// Style holds the resolved layout inputs for a node. Lengths are in points;
// percentages are resolved by the caller before layout.
type Style struct {
	Direction    Direction
	Wrap         Wrap // flex-wrap; the zero value (NoWrap) keeps all children on one line
	Justify      Justify
	AlignItems   Align // container cross alignment; AlignAuto ⇒ stretch
	AlignSelf    Align // per-child override; AlignAuto ⇒ use parent's AlignItems
	AlignContent Align // cross-axis distribution of wrapped lines; AlignAuto ⇒ flex-start

	Grow   float64
	Shrink float64
	Basis  Dim // flex-basis; Auto ⇒ use the main-dimension size

	Width, Height Dim
	AspectRatio   float64 // width/height; when one dimension is auto, derive the other

	Position                 Position
	Top, Right, Bottom, Left Dim // insets for absolute positioning

	MarginTop, MarginRight, MarginBottom, MarginLeft     float64
	PaddingTop, PaddingRight, PaddingBottom, PaddingLeft float64
	BorderTop, BorderRight, BorderBottom, BorderLeft     float64

	Gap float64 // space between children along the main axis and between wrapped lines along the cross axis
}

func (s *Style) isAbsolute() bool { return s.Position == PositionAbsolute }

// Size is a measured width/height.
type Size struct{ W, H float64 }

// MeasureFunc measures a leaf node (e.g. text or an image) given the available
// space. Returned sizes are content sizes (the engine adds nothing to them).
type MeasureFunc func(availW, availH float64) Size

// Node is a layout node. Set Style, Children, and (for leaves) Measure; after
// Calculate, read Layout.
type Node struct {
	Style    Style
	Children []*Node
	Measure  MeasureFunc

	Layout Layout
}

// Layout is a node's computed border-box, relative to its parent's content-box
// origin.
type Layout struct {
	Left, Top, Width, Height float64
}

func (s *Style) isRow() bool { return s.Direction == Row }

func (s *Style) horizEdges() float64 {
	return s.PaddingLeft + s.PaddingRight + s.BorderLeft + s.BorderRight
}
func (s *Style) vertEdges() float64 {
	return s.PaddingTop + s.PaddingBottom + s.BorderTop + s.BorderBottom
}
