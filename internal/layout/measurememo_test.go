package layout

import (
	"slices"
	"testing"
)

// A memoised measurement must behave exactly as measuring again: a hit at an
// earlier width restores that width's lines, not the most recently computed ones,
// because the renderer paints whatever lines the last measure call left.
func TestTextMeasureMemoRestoresLinesOfThatWidth(t *testing.T) {
	tr := &textResolve{content: "alpha beta gamma delta epsilon", size: 12, measurer: mustHelv(t), lineHeight: 14}
	wide := tr.measure(400, 0)
	wideLines := slices.Clone(tr.lines)
	narrow := tr.measure(60, 0)
	narrowLines := slices.Clone(tr.lines)
	if len(narrowLines) <= len(wideLines) {
		t.Fatalf("fixture should wrap more at 60 than at 400: %q vs %q", narrowLines, wideLines)
	}

	if got := tr.measure(400, 0); got != wide || !slices.Equal(tr.lines, wideLines) {
		t.Errorf("hit at 400: size %v lines %q, want %v %q", got, tr.lines, wide, wideLines)
	}
	if got := tr.measure(60, 0); got != narrow || !slices.Equal(tr.lines, narrowLines) {
		t.Errorf("hit at 60: size %v lines %q, want %v %q", got, tr.lines, narrow, narrowLines)
	}
}
