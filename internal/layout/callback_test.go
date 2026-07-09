package layout

import (
	"fmt"
	"testing"

	"github.com/swish/feast/internal/contract"
	"github.com/swish/feast/internal/tree"
)

// stubEval returns a deterministic "P{page}/{total}" string, standing in for the
// JS VM so the layout wiring can be tested without goja.
type stubEval struct{ calls int }

func (s *stubEval) EvalText(id string, pc PageContext) (string, error) {
	s.calls++
	return fmt.Sprintf("%s:P%d/%d", id, pc.PageNumber, pc.TotalPages), nil
}

func (s *stubEval) EvalPaint(id string, w, h float64) ([]any, error) {
	// A single rect spanning the canvas, so tests can assert the op flowed through.
	return []any{map[string]any{"op": "rect", "args": []any{0.0, 0.0, w, h}}, map[string]any{"op": "fill", "args": []any{"#ff0000"}}}, nil
}

// A fixed footer with a function render-prop ({$cb}) must be evaluated per page
// with that page's number — the dynamic analogue of the string-template footer.
func TestCallbackRenderPropPerPage(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[80,100]},"children":[
			{"type":"VIEW","props":{"style":{"height":40}}},
			{"type":"VIEW","props":{"style":{"height":40}}},
			{"type":"VIEW","props":{"style":{"height":40}}},
			{"type":"VIEW","props":{"style":{"height":40}}},
			{"type":"TEXT","props":{"fixed":true,"render":{"$cb":"cb_0"},"style":{"position":"absolute","bottom":5,"left":5}}}
		]}
	],"callbacks":["cb_0"]}}`

	ct, err := contract.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("contract.Parse: %v", err)
	}
	tr, err := tree.Build(ct)
	if err != nil {
		t.Fatalf("tree.Build: %v", err)
	}
	eval := &stubEval{}
	res, err := Layout(tr, Options{Eval: eval})
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	if len(res.Pages) != 2 {
		t.Fatalf("pages = %d, want 2 (four 40pt blocks in a 100pt page)", len(res.Pages))
	}

	want := []string{"cb_0:P1/2", "cb_0:P2/2"}
	for i, pg := range res.Pages {
		ft := findCallbackText(pg.Root)
		if ft == nil {
			t.Fatalf("page %d: no callback footer text box found", i+1)
		}
		if ft.Content != want[i] {
			t.Errorf("page %d footer = %q, want %q", i+1, ft.Content, want[i])
		}
		if len(ft.Lines) != 1 || ft.Lines[0] != want[i] {
			t.Errorf("page %d footer lines = %v, want [%q]", i+1, ft.Lines, want[i])
		}
	}
	// One seed eval ({1,1}) at build + one per page after pagination.
	if eval.calls < 3 {
		t.Errorf("evaluator calls = %d, want >= 3 (seed + 2 pages)", eval.calls)
	}
}

// A callback render-prop with no evaluator (static path) must not crash; it
// simply produces no text box.
func TestCallbackRenderPropNoEvaluator(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[80,100]},"children":[
			{"type":"TEXT","props":{"fixed":true,"render":{"$cb":"cb_0"},"style":{"position":"absolute","bottom":5}}}
		]}
	],"callbacks":["cb_0"]}}`
	ct, err := contract.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("contract.Parse: %v", err)
	}
	tr, err := tree.Build(ct)
	if err != nil {
		t.Fatalf("tree.Build: %v", err)
	}
	res, err := Layout(tr, Options{}) // no Eval
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	if findCallbackText(res.Pages[0].Root) != nil {
		t.Error("callback footer produced a text box with no evaluator; want none")
	}
}

// findCallbackText returns the first Text box carrying a callback id.
func findCallbackText(b *Box) *TextInfo {
	if b.Text != nil && b.Text.CallbackID != "" {
		return b.Text
	}
	for _, c := range b.Children {
		if ft := findCallbackText(c); ft != nil {
			return ft
		}
	}
	return nil
}

// A Canvas with a function paint prop must be evaluated after layout (once its
// frame is known) and its ops attached to the box.
func TestCanvasPaintCallback(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"CANVAS","props":{"paint":{"$cb":"cb_0"},"style":{"width":80,"height":40}}}
		]}
	],"callbacks":["cb_0"]}}`
	ct, err := contract.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("contract.Parse: %v", err)
	}
	tr, err := tree.Build(ct)
	if err != nil {
		t.Fatalf("tree.Build: %v", err)
	}
	res, err := Layout(tr, Options{Eval: &stubEval{}})
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	cv := findCanvas(res.Pages[0].Root)
	if cv == nil {
		t.Fatal("no canvas box with ops")
	}
	if len(cv) != 2 {
		t.Fatalf("canvas ops = %d, want 2 (rect, fill)", len(cv))
	}
	op0, _ := cv[0].(map[string]any)
	if op0["op"] != "rect" {
		t.Errorf("op[0] = %v, want rect", op0["op"])
	}
	// The stub's rect spans the resolved frame (80×40) — proves w,h were threaded.
	args, _ := op0["args"].([]any)
	if len(args) != 4 || args[2].(float64) != 80 || args[3].(float64) != 40 {
		t.Errorf("rect args = %v, want [0 0 80 40]", args)
	}
}

func findCanvas(b *Box) []any {
	if len(b.Canvas) > 0 {
		return b.Canvas
	}
	for _, c := range b.Children {
		if ops := findCanvas(c); ops != nil {
			return ops
		}
	}
	return nil
}
