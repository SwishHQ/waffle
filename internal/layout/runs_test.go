package layout

import (
	"testing"

	"github.com/swish/waffle/internal/contract"
	"github.com/swish/waffle/internal/stylesheet"
	"github.com/swish/waffle/internal/tree"
)

// buildRuns flattens nested Text into styled runs; a bold child yields a run that
// resolves to Helvetica-Bold while the surrounding runs stay Helvetica.
func TestBuildRunsStyles(t *testing.T) {
	node := &tree.Node{Type: contract.TypeText, Children: []*tree.Node{
		{Type: contract.TypeTextInstance, Value: "Hello "},
		{Type: contract.TypeText, Style: map[string]any{"fontWeight": "bold"}, Children: []*tree.Node{
			{Type: contract.TypeTextInstance, Value: "world"},
		}},
		{Type: contract.TypeTextInstance, Value: "!"},
	}}
	parent := stylesheet.Resolve(map[string]any{"fontSize": 12}, stylesheet.MediaContext{})
	runs := buildRuns(node, parent, stylesheet.MediaContext{}, stylesheet.Context{}, nil)
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(runs))
	}
	if runs[0].style.base != "Helvetica" {
		t.Errorf("first run should be Helvetica, got %q", runs[0].style.base)
	}
	if runs[1].style.base != "Helvetica-Bold" {
		t.Errorf("bold run should be Helvetica-Bold, got %q", runs[1].style.base)
	}
	if runs[1].text != "world" || runs[2].text != "!" {
		t.Errorf("run texts wrong: %q %q", runs[1].text, runs[2].text)
	}
}

// hasInlineRuns detects nested styled elements but not plain text-only Text.
func TestHasInlineRuns(t *testing.T) {
	plain := &tree.Node{Type: contract.TypeText, Children: []*tree.Node{
		{Type: contract.TypeTextInstance, Value: "just text"},
	}}
	if hasInlineRuns(plain) {
		t.Error("plain text should not be treated as inline runs")
	}
	nested := &tree.Node{Type: contract.TypeText, Children: []*tree.Node{
		{Type: contract.TypeText, Children: []*tree.Node{{Type: contract.TypeTextInstance, Value: "x"}}},
	}}
	if !hasInlineRuns(nested) {
		t.Error("nested Text should be treated as inline runs")
	}
}
