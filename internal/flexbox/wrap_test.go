package flexbox

import "testing"

// wrapRow builds a row container with the given wrap mode and three 40×20
// children — the canonical fixture: two fit per 100pt line, the third wraps.
func wrapRow(w Wrap) *Node {
	return &Node{
		Style:    Style{Direction: Row, Wrap: w},
		Children: []*Node{leaf(40, 20), leaf(40, 20), leaf(40, 20)},
	}
}

func TestWrapRowBreaksLines(t *testing.T) {
	root := wrapRow(WrapWrap)
	Calculate(root, 100, 40)
	box(t, root.Children[0], "c0", 0, 0, 40, 20)
	box(t, root.Children[1], "c1", 40, 0, 40, 20)
	box(t, root.Children[2], "c2", 0, 20, 40, 20)
}

func TestWrapReverseFlipsLineOrder(t *testing.T) {
	root := wrapRow(WrapReverse)
	Calculate(root, 100, 40)
	// Line order flips along the cross axis: the wrapped line (child 2) on top.
	box(t, root.Children[2], "c2", 0, 0, 40, 20)
	box(t, root.Children[0], "c0", 0, 20, 40, 20)
	box(t, root.Children[1], "c1", 40, 20, 40, 20)
}

func TestWrapNoWrapSingleLine(t *testing.T) {
	// Explicit NoWrap keeps all three on one (overflowing) line, exactly as
	// before wrap existed.
	root := wrapRow(WrapNoWrap)
	Calculate(root, 100, 40)
	box(t, root.Children[0], "c0", 0, 0, 40, 20)
	box(t, root.Children[1], "c1", 40, 0, 40, 20)
	box(t, root.Children[2], "c2", 80, 0, 40, 20)
}

func TestWrapAlignContentStretch(t *testing.T) {
	// Auto-height children: the two 0-high lines split the container's 60pt of
	// cross space equally (30 each) and stretch-aligned items fill their line.
	root := &Node{
		Style: Style{Direction: Row, Wrap: WrapWrap, AlignContent: AlignStretch},
		Children: []*Node{
			{Style: Style{Width: Pt(40)}},
			{Style: Style{Width: Pt(40)}},
			{Style: Style{Width: Pt(40)}},
		},
	}
	Calculate(root, 100, 60)
	box(t, root.Children[0], "c0", 0, 0, 40, 30)
	box(t, root.Children[1], "c1", 40, 0, 40, 30)
	box(t, root.Children[2], "c2", 0, 30, 40, 30)
}

func TestWrapAlignContentCenter(t *testing.T) {
	root := &Node{
		Style:    Style{Direction: Row, Wrap: WrapWrap, AlignContent: AlignCenter},
		Children: []*Node{leaf(40, 20), leaf(40, 20), leaf(40, 20)},
	}
	Calculate(root, 100, 60)
	// Two 20pt lines in 60pt: free cross space 20, so the block starts at 10.
	eq(t, "c0.Top", root.Children[0].Layout.Top, 10)
	eq(t, "c1.Top", root.Children[1].Layout.Top, 10)
	eq(t, "c2.Top", root.Children[2].Layout.Top, 30)
}

func TestWrapAutoCrossGrowsToLines(t *testing.T) {
	// A wrapping container with a definite width and auto height sizes its
	// height to the sum of its line cross sizes.
	inner := &Node{
		Style:    Style{Direction: Row, Wrap: WrapWrap, Width: Pt(100)},
		Children: []*Node{leaf(40, 20), leaf(40, 20), leaf(40, 20)},
	}
	root := &Node{Style: Style{Direction: Column}, Children: []*Node{inner}}
	Calculate(root, 200, 200)
	box(t, inner, "inner", 0, 0, 100, 40)
	box(t, inner.Children[2], "c2", 0, 20, 40, 20)
}

func TestWrapGapAppliesToBothAxes(t *testing.T) {
	// Gap separates items within a line (main axis) and lines from each other
	// (cross axis): 40+10+40 fits in 100, the third child wraps below.
	root := &Node{
		Style:    Style{Direction: Row, Wrap: WrapWrap, Gap: 10},
		Children: []*Node{leaf(40, 20), leaf(40, 20), leaf(40, 20)},
	}
	Calculate(root, 100, 50)
	box(t, root.Children[0], "c0", 0, 0, 40, 20)
	box(t, root.Children[1], "c1", 50, 0, 40, 20)
	box(t, root.Children[2], "c2", 0, 30, 40, 20)
}

func TestWrapColumn(t *testing.T) {
	// Column wrap: lines are columns that stack left to right.
	root := &Node{
		Style:    Style{Direction: Column, Wrap: WrapWrap},
		Children: []*Node{leaf(20, 40), leaf(20, 40), leaf(20, 40)},
	}
	Calculate(root, 60, 100)
	box(t, root.Children[0], "c0", 0, 0, 20, 40)
	box(t, root.Children[1], "c1", 0, 40, 20, 40)
	box(t, root.Children[2], "c2", 20, 0, 20, 40)
}

func TestWrapJustifyPerLine(t *testing.T) {
	// Each line centers independently: line 1 has 20pt free, line 2 has 60pt.
	root := &Node{
		Style:    Style{Direction: Row, Wrap: WrapWrap, Justify: JustifyCenter},
		Children: []*Node{leaf(40, 20), leaf(40, 20), leaf(40, 20)},
	}
	Calculate(root, 100, 40)
	eq(t, "c0.Left", root.Children[0].Layout.Left, 10)
	eq(t, "c1.Left", root.Children[1].Layout.Left, 50)
	eq(t, "c2.Left", root.Children[2].Layout.Left, 30)
}

func TestWrapGrowPerLine(t *testing.T) {
	grow := func() *Node { return &Node{Style: Style{Width: Pt(40), Height: Pt(20), Grow: 1}} }
	root := &Node{
		Style:    Style{Direction: Row, Wrap: WrapWrap},
		Children: []*Node{grow(), grow(), grow()},
	}
	Calculate(root, 100, 40)
	// Line 1: both items grow 40→50; line 2: the lone item fills the line.
	box(t, root.Children[0], "c0", 0, 0, 50, 20)
	box(t, root.Children[1], "c1", 50, 0, 50, 20)
	box(t, root.Children[2], "c2", 0, 20, 100, 20)
}
