package layout

import (
	"strings"
	"testing"

	"github.com/SwishHQ/waffle/internal/pdf/afm"
)

func mustHelv(t *testing.T) *afm.Metrics {
	t.Helper()
	m, err := afm.Load("Helvetica")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// maxLines caps wrapped output; textOverflow:ellipsis appends … to the last line.
func TestWrapMaxLinesEllipsis(t *testing.T) {
	tr := &textResolve{
		content:  "alpha beta gamma delta epsilon zeta eta theta",
		size:     12,
		measurer: mustHelv(t),
		maxLines: 2,
		ellipsis: true,
	}
	lines := tr.wrap(70) // narrow enough to force several lines
	if len(lines) != 2 {
		t.Fatalf("maxLines=2 should cap at 2 lines, got %d: %q", len(lines), lines)
	}
	if !strings.HasSuffix(lines[1], "…") {
		t.Errorf("last line should end with an ellipsis: %q", lines[1])
	}
	if w := tr.stringWidth(lines[1]); w > 70 {
		t.Errorf("ellipsized line width %v exceeds maxW 70: %q", w, lines[1])
	}
}

// maxLines without ellipsis just truncates.
func TestWrapMaxLinesNoEllipsis(t *testing.T) {
	tr := &textResolve{
		content:  "alpha beta gamma delta epsilon zeta eta theta",
		size:     12,
		measurer: mustHelv(t),
		maxLines: 2,
	}
	lines := tr.wrap(70)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if strings.Contains(strings.Join(lines, ""), "…") {
		t.Errorf("no ellipsis expected without textOverflow: %q", lines)
	}
}

// When content fits within maxLines, nothing is truncated or ellipsized.
func TestWrapUnderMaxLines(t *testing.T) {
	tr := &textResolve{content: "short", size: 12, measurer: mustHelv(t), maxLines: 3, ellipsis: true}
	lines := tr.wrap(200)
	if len(lines) != 1 || strings.Contains(lines[0], "…") {
		t.Errorf("fitting content should be untouched: %q", lines)
	}
}
