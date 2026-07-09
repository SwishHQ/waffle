package flexbox

import (
	"math"
	"testing"
)

func leaf(w, h float64) *Node {
	return &Node{Style: Style{Width: Pt(w), Height: Pt(h)}}
}

func eq(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func box(t *testing.T, n *Node, name string, l, top, w, h float64) {
	t.Helper()
	eq(t, name+".Left", n.Layout.Left, l)
	eq(t, name+".Top", n.Layout.Top, top)
	eq(t, name+".Width", n.Layout.Width, w)
	eq(t, name+".Height", n.Layout.Height, h)
}

func TestRowFixedFlexStart(t *testing.T) {
	root := &Node{Style: Style{Direction: Row}, Children: []*Node{leaf(50, 50), leaf(50, 50), leaf(50, 50)}}
	Calculate(root, 300, 50)
	eq(t, "c0.Left", root.Children[0].Layout.Left, 0)
	eq(t, "c1.Left", root.Children[1].Layout.Left, 50)
	eq(t, "c2.Left", root.Children[2].Layout.Left, 100)
	for _, c := range root.Children {
		eq(t, "top", c.Layout.Top, 0)
		eq(t, "height", c.Layout.Height, 50)
	}
}

func TestRowSpaceBetween(t *testing.T) {
	root := &Node{Style: Style{Direction: Row, Justify: JustifySpaceBetween}, Children: []*Node{leaf(50, 50), leaf(50, 50), leaf(50, 50)}}
	Calculate(root, 300, 50)
	// free = 300 - 150 = 150; gap between = 75.
	eq(t, "c0.Left", root.Children[0].Layout.Left, 0)
	eq(t, "c1.Left", root.Children[1].Layout.Left, 125)
	eq(t, "c2.Left", root.Children[2].Layout.Left, 250)
}

func TestColumnGrowFills(t *testing.T) {
	root := &Node{Style: Style{Direction: Column}, Children: []*Node{
		{Style: Style{Grow: 1}}, {Style: Style{Grow: 1}},
	}}
	Calculate(root, 100, 200)
	box(t, root.Children[0], "c0", 0, 0, 100, 100)
	box(t, root.Children[1], "c1", 0, 100, 100, 100)
}

func TestPaddingOffsetsContent(t *testing.T) {
	root := &Node{Style: Style{Direction: Column, PaddingTop: 10, PaddingRight: 10, PaddingBottom: 10, PaddingLeft: 10},
		Children: []*Node{leaf(50, 50)}}
	Calculate(root, 200, 100)
	box(t, root.Children[0], "child", 10, 10, 50, 50)
}

func TestGapBetweenChildren(t *testing.T) {
	root := &Node{Style: Style{Direction: Row, Gap: 20}, Children: []*Node{leaf(50, 50), leaf(50, 50)}}
	Calculate(root, 300, 50)
	eq(t, "c0.Left", root.Children[0].Layout.Left, 0)
	eq(t, "c1.Left", root.Children[1].Layout.Left, 70)
}

func TestAlignItemsCenter(t *testing.T) {
	root := &Node{Style: Style{Direction: Row, AlignItems: AlignCenter}, Children: []*Node{leaf(50, 40)}}
	Calculate(root, 300, 100)
	eq(t, "child.Top", root.Children[0].Layout.Top, 30) // (100-40)/2
	eq(t, "child.Height", root.Children[0].Layout.Height, 40)
}

func TestAlignItemsStretch(t *testing.T) {
	// Child has auto cross dimension (height), so stretch fills the cross axis.
	root := &Node{Style: Style{Direction: Row}, Children: []*Node{{Style: Style{Width: Pt(50)}}}}
	Calculate(root, 300, 80)
	eq(t, "child.Height", root.Children[0].Layout.Height, 80)
	eq(t, "child.Width", root.Children[0].Layout.Width, 50)
}

func TestShrinkOverflow(t *testing.T) {
	root := &Node{Style: Style{Direction: Row}, Children: []*Node{
		{Style: Style{Width: Pt(80), Height: Pt(50), Shrink: 1}},
		{Style: Style{Width: Pt(80), Height: Pt(50), Shrink: 1}},
	}}
	Calculate(root, 100, 50)
	// free = -60, each shrinks by 30 → width 50.
	eq(t, "c0.Width", root.Children[0].Layout.Width, 50)
	eq(t, "c1.Width", root.Children[1].Layout.Width, 50)
	eq(t, "c1.Left", root.Children[1].Layout.Left, 50)
}

func TestNestedContainers(t *testing.T) {
	inner := &Node{Style: Style{Direction: Column, Width: Pt(100), Height: Pt(100)}, Children: []*Node{leaf(30, 20), leaf(30, 20)}}
	root := &Node{Style: Style{Direction: Row}, Children: []*Node{inner}}
	Calculate(root, 200, 100)

	box(t, inner, "inner", 0, 0, 100, 100)
	box(t, inner.Children[0], "g0", 0, 0, 30, 20)
	box(t, inner.Children[1], "g1", 0, 20, 30, 20)
}

func TestIntrinsicColumnSizing(t *testing.T) {
	// A column container with auto size should size to its children plus gap.
	inner := &Node{Style: Style{Direction: Column, Gap: 5}, Children: []*Node{leaf(40, 10), leaf(60, 10)}}
	// width should be max child (60), height should be 10+5+10 = 25.
	got := measure(inner, 1000, 1000)
	eq(t, "intrinsic.W", got.W, 60)
	eq(t, "intrinsic.H", got.H, 25)
}

func TestPercentDimensions(t *testing.T) {
	// Row parent 200x100; child 50% wide, 100% tall.
	root := &Node{Style: Style{Direction: Row}, Children: []*Node{
		{Style: Style{Width: Percent(50), Height: Percent(100)}},
	}}
	Calculate(root, 200, 100)
	box(t, root.Children[0], "child", 0, 0, 100, 100)
}

func TestPercentBasis(t *testing.T) {
	// Row parent content width 300; two children with 25% and 75% flex-basis.
	root := &Node{Style: Style{Direction: Row}, Children: []*Node{
		{Style: Style{Basis: Percent(25), Height: Pt(10)}},
		{Style: Style{Basis: Percent(75), Height: Pt(10)}},
	}}
	Calculate(root, 300, 10)
	eq(t, "c0.Width", root.Children[0].Layout.Width, 75)
	eq(t, "c1.Width", root.Children[1].Layout.Width, 225)
	eq(t, "c1.Left", root.Children[1].Layout.Left, 75)
}

func TestAbsolutePositioning(t *testing.T) {
	root := &Node{Style: Style{Direction: Column, PaddingTop: 10, PaddingLeft: 10}, Children: []*Node{
		{Style: Style{Height: Pt(20)}},
		{Style: Style{Position: PositionAbsolute, Width: Pt(30), Height: Pt(15), Top: Pt(5), Left: Pt(8)}},
		{Style: Style{Position: PositionAbsolute, Width: Pt(30), Height: Pt(15), Bottom: Pt(0), Right: Pt(0)}},
	}}
	Calculate(root, 200, 200)
	// Flow child: content origin (10,10), stretched to content width (190).
	box(t, root.Children[0], "flow", 10, 10, 190, 20)
	// Absolute top-left: origin + (left,top).
	box(t, root.Children[1], "abs-tl", 18, 15, 30, 15)
	// Absolute bottom-right: contentW=190, contentH=190 → x=160,y=175 (+origin).
	box(t, root.Children[2], "abs-br", 170, 185, 30, 15)
}
