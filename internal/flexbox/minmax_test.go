package flexbox

import (
	"math"
	"testing"
)

func approxEq(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// A grow item is capped at MaxWidth instead of filling the row.
func TestMaxWidthCapsGrow(t *testing.T) {
	child := &Node{Style: Style{Grow: 1, MaxWidth: Pt(60)}}
	root := &Node{Style: Style{Direction: Row, Width: Pt(200), Height: Pt(50)}, Children: []*Node{child}}
	Calculate(root, 200, 50)
	if !approxEq(child.Layout.Width, 60) {
		t.Errorf("grow child width = %v, want 60 (capped by maxWidth)", child.Layout.Width)
	}
}

// An item narrower than MinWidth is raised to MinWidth.
func TestMinWidthFloor(t *testing.T) {
	child := &Node{Style: Style{Width: Pt(20), MinWidth: Pt(50)}}
	root := &Node{Style: Style{Direction: Column, Width: Pt(200), Height: Pt(50)}, Children: []*Node{child}}
	Calculate(root, 200, 50)
	if !approxEq(child.Layout.Width, 50) {
		t.Errorf("child width = %v, want 50 (raised by minWidth)", child.Layout.Width)
	}
}

// MaxHeight caps an explicit height.
func TestMaxHeightCaps(t *testing.T) {
	child := &Node{Style: Style{Width: Pt(40), Height: Pt(400), MaxHeight: Pt(100)}}
	root := &Node{Style: Style{Direction: Column, Width: Pt(200), Height: Pt(500)}, Children: []*Node{child}}
	Calculate(root, 200, 500)
	if !approxEq(child.Layout.Height, 100) {
		t.Errorf("child height = %v, want 100 (capped by maxHeight)", child.Layout.Height)
	}
}

// A percentage max resolves against the parent's content size.
func TestPercentMaxWidth(t *testing.T) {
	child := &Node{Style: Style{Grow: 1, MaxWidth: Percent(25)}} // 25% of 200 = 50
	root := &Node{Style: Style{Direction: Row, Width: Pt(200), Height: Pt(50)}, Children: []*Node{child}}
	Calculate(root, 200, 50)
	if !approxEq(child.Layout.Width, 50) {
		t.Errorf("child width = %v, want 50 (25%% of 200)", child.Layout.Width)
	}
}

// With no min/max, a grow child still fills the row (no regression).
func TestNoConstraintUnchanged(t *testing.T) {
	child := &Node{Style: Style{Grow: 1}}
	root := &Node{Style: Style{Direction: Row, Width: Pt(200), Height: Pt(50)}, Children: []*Node{child}}
	Calculate(root, 200, 50)
	if !approxEq(child.Layout.Width, 200) {
		t.Errorf("unconstrained grow child width = %v, want 200", child.Layout.Width)
	}
}
