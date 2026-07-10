package jsruntime

import (
	"encoding/json"
	"strings"
	"testing"
)

// A function render prop must survive as a {$cb} ref and be evaluable per page
// through the live VM — proves the callback bridge round-trips under goja.
func TestGojaRenderCallbackBridge(t *testing.T) {
	const doc = `
import { Document, Page, Text } from '@waffle/react';
export default function App() {
  return (
    <Document>
      <Page>
        <Text fixed render={({ pageNumber, totalPages }) => 'page ' + pageNumber + ' of ' + totalPages} />
      </Page>
    </Document>
  );
}
`
	prog, err := Compile([]byte(doc), Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	inst, err := prog.Instantiate(nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	// The tree must declare the callback and reference it via {$cb}.
	var tree struct {
		Callbacks []string `json:"callbacks"`
		Document  struct {
			Children []struct {
				Children []struct {
					Props map[string]any `json:"props"`
				} `json:"children"`
			} `json:"children"`
		} `json:"document"`
	}
	if err := json.Unmarshal(inst.Tree(), &tree); err != nil {
		t.Fatalf("tree JSON: %v", err)
	}
	if len(tree.Callbacks) != 1 {
		t.Fatalf("expected 1 declared callback, got %v (tree=%s)", tree.Callbacks, inst.Tree())
	}
	id := tree.Callbacks[0]
	render, _ := tree.Document.Children[0].Children[0].Props["render"].(map[string]any)
	if render["$cb"] != id {
		t.Fatalf("render prop should be {$cb:%q}, got %v", id, render)
	}

	// Evaluate it with a page context — the closure runs in the VM.
	nodes, err := inst.EvalCallback(id, []byte(`{"pageNumber":2,"totalPages":7}`))
	if err != nil {
		t.Fatalf("EvalCallback: %v", err)
	}
	if !strings.Contains(string(nodes), "page 2 of 7") {
		t.Errorf("callback result = %s, want text 'page 2 of 7'", nodes)
	}

	// A second call with different context re-evaluates (closure is reusable).
	nodes2, err := inst.EvalCallback(id, []byte(`{"pageNumber":5,"totalPages":7}`))
	if err != nil {
		t.Fatalf("EvalCallback 2: %v", err)
	}
	if !strings.Contains(string(nodes2), "page 5 of 7") {
		t.Errorf("second callback result = %s, want text 'page 5 of 7'", nodes2)
	}
}

// A callback returning an element (not a string) serializes to a node subtree.
func TestGojaRenderCallbackElement(t *testing.T) {
	const doc = `
import { Document, Page, View, Text } from '@waffle/react';
export default function App() {
  return (
    <Document>
      <Page>
        <View render={({ pageNumber }) => <Text style={{ fontSize: 9 }}>{'n' + pageNumber}</Text>} />
      </Page>
    </Document>
  );
}
`
	prog, err := Compile([]byte(doc), Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	inst, err := prog.Instantiate(nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	nodes, err := inst.EvalCallback("cb_0", []byte(`{"pageNumber":4}`))
	if err != nil {
		t.Fatalf("EvalCallback: %v", err)
	}
	s := string(nodes)
	if !strings.Contains(s, `"type":"TEXT"`) || !strings.Contains(s, "n4") {
		t.Errorf("callback element result = %s", s)
	}
}

// A Canvas paint callback must become a {$cb} ref and, when evaluated with a
// size, return the recorded painter ops in the Go replayer's {op,args} format.
func TestGojaCanvasPaintBridge(t *testing.T) {
	const doc = `
import { Document, Page, Canvas } from '@waffle/react';
export default function App() {
  return (
    <Document>
      <Page>
        <Canvas style={{ width: 120, height: 60 }}
          paint={(p, w, h) => p.rect(0, 0, w, h).fillColor('#ff0000').fill()} />
      </Page>
    </Document>
  );
}
`
	prog, err := Compile([]byte(doc), Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	inst, err := prog.Instantiate(nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if !strings.Contains(string(inst.Tree()), `"$cb"`) {
		t.Fatalf("paint not serialized as $cb: %s", inst.Tree())
	}

	ops, err := inst.EvalPaint("cb_0", 120, 60)
	if err != nil {
		t.Fatalf("EvalPaint: %v", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(ops, &decoded); err != nil {
		t.Fatalf("ops JSON: %v\n%s", err, ops)
	}
	if len(decoded) != 3 {
		t.Fatalf("ops = %d, want 3 (rect, fillColor, fill): %s", len(decoded), ops)
	}
	if decoded[0]["op"] != "rect" {
		t.Errorf("op[0] = %v, want rect", decoded[0]["op"])
	}
	args, _ := decoded[0]["args"].([]any)
	if len(args) != 4 || args[2].(float64) != 120 || args[3].(float64) != 60 {
		t.Errorf("rect args = %v, want [0 0 120 60] (w,h threaded in)", args)
	}
	if decoded[1]["op"] != "fillColor" || decoded[2]["op"] != "fill" {
		t.Errorf("ops = %v, want fillColor then fill", decoded)
	}
}
