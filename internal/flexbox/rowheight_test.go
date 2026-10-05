package flexbox

import (
	"math"
	"testing"
)

// textLeaf measures like a Text of the given single-line width and line height:
// it breaks into ceil(lineW/availW) lines at the width it is offered.
func textLeaf(lineW, lineH float64) *Node {
	return &Node{Measure: func(availW, _ float64) Size {
		if availW <= 0 || lineW <= availW {
			return Size{W: lineW, H: lineH}
		}
		return Size{W: availW, H: math.Ceil(lineW/availW) * lineH}
	}}
}

// A text cell that fits on one line at the row's full width but wraps at the
// width its flex share leaves it. The row (auto height, in a column) must be as
// tall as the wrapped text, or the paint pass draws two lines into a one-line
// box and the second spills over whatever follows.
func TestRowHeightMeasuresChildAtItsFlexedWidth(t *testing.T) {
	cell := &Node{Style: Style{Grow: 6, Shrink: 1, Basis: Pt(0)}, Children: []*Node{textLeaf(150, 10)}}
	other := &Node{Style: Style{Grow: 4, Shrink: 1, Basis: Pt(0)}, Children: []*Node{textLeaf(20, 10)}}
	row := &Node{Style: Style{Direction: Row}, Children: []*Node{cell, other}}
	next := leaf(200, 10)
	root := &Node{Style: Style{Direction: Column}, Children: []*Node{row, next}}
	Calculate(root, 200, 400)

	// The cell is 6/10 of 200 = 120pt wide; 150pt of text wraps to 2 lines.
	box(t, cell, "cell", 0, 0, 120, 20)
	box(t, row, "row", 0, 0, 200, 20)
	box(t, next, "next", 0, 20, 200, 10)
}

// The same at a padded container: children wrap inside its content box.
func TestColumnHeightMeasuresChildrenInContentBox(t *testing.T) {
	inner := &Node{Style: Style{Direction: Column, PaddingLeft: 20, PaddingRight: 20}, Children: []*Node{textLeaf(150, 10)}}
	next := leaf(170, 10)
	root := &Node{Style: Style{Direction: Column}, Children: []*Node{inner, next}}
	Calculate(root, 170, 400)

	// Content width is 170-40 = 130pt; 150pt of text wraps to 2 lines.
	box(t, inner, "inner", 0, 0, 170, 20)
	box(t, next, "next", 0, 20, 170, 10)
}

// A non-stretched row item takes its cross size from measurement, so it must
// also be measured at its flexed width.
func TestRowFlexStartItemCrossAtFlexedWidth(t *testing.T) {
	cell := &Node{Style: Style{Grow: 1, Shrink: 1, Basis: Pt(0), AlignSelf: AlignFlexStart}, Children: []*Node{textLeaf(150, 10)}}
	other := &Node{Style: Style{Grow: 1, Shrink: 1, Basis: Pt(0)}, Children: []*Node{textLeaf(20, 10)}}
	row := &Node{Style: Style{Direction: Row}, Children: []*Node{cell, other}}
	Calculate(row, 200, 100)

	// Each cell is 100pt; 150pt of text wraps to 2 lines.
	box(t, cell, "cell", 0, 0, 100, 20)
}
