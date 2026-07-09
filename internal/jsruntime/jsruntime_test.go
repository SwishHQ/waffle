package jsruntime

import (
	"encoding/json"
	"strings"
	"testing"
)

// A JSX document exercising the react-pdf authoring surface: primitive imports,
// a function component, props, and .map — no React import (automatic runtime),
// no hooks. It default-exports a function of props, run entirely in goja.
const sampleDoc = `
import { Document, Page, View, Text } from '@feast/react';

function Badge({ label }) {
  return <Text style={{ fontSize: 11 }}>{label}</Text>;
}

export default function App(props) {
  return (
    <Document title={props.title} author="feast">
      <Page size="A6" style={{ padding: 24 }}>
        <Text style={{ fontSize: 22 }}>Hello {props.name}</Text>
        <View style={{ flexDirection: 'row', gap: 8 }}>
          {props.features.map((f, i) => <Badge key={i} label={f} />)}
        </View>
      </Page>
    </Document>
  );
}
`

func TestCompileAndRender(t *testing.T) {
	prog, err := Compile([]byte(sampleDoc), Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	props, _ := json.Marshal(map[string]any{
		"title":    "goja doc",
		"name":     "goja",
		"features": []string{"flexbox", "text", "svg"},
	})
	treeJSON, err := prog.Render(props)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	var tree struct {
		Version  string `json:"version"`
		Document struct {
			Props    map[string]any   `json:"props"`
			Children []map[string]any `json:"children"`
		} `json:"document"`
	}
	if err := json.Unmarshal(treeJSON, &tree); err != nil {
		t.Fatalf("tree is not valid JSON: %v\n%s", err, treeJSON)
	}

	if tree.Version != "feast-tree/v1" {
		t.Errorf("version = %q, want feast-tree/v1", tree.Version)
	}
	if got := tree.Document.Props["title"]; got != "goja doc" {
		t.Errorf("document title = %v, want %q (props threaded through goja)", got, "goja doc")
	}
	if got := tree.Document.Props["author"]; got != "feast" {
		t.Errorf("document author = %v, want feast", got)
	}
	if len(tree.Document.Children) != 1 || tree.Document.Children[0]["type"] != "PAGE" {
		t.Fatalf("expected one PAGE child, got %+v", tree.Document.Children)
	}

	// The three features must have become three Badge->Text nodes.
	s := string(treeJSON)
	for _, want := range []string{"Hello", "goja", "flexbox", "text", "svg", `"type":"VIEW"`, `"type":"TEXT"`} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered tree missing %q\n%s", want, s)
		}
	}
	if n := strings.Count(s, `"fontSize":11`); n != 3 {
		t.Errorf("expected 3 badges (fontSize 11), found %d", n)
	}
}

// A bare element default export (not a function) must also work.
func TestCompileElementDefault(t *testing.T) {
	const doc = `
import { Document, Page, Text } from '@feast/react';
export default (
  <Document title="static">
    <Page><Text>hi</Text></Page>
  </Document>
);
`
	prog, err := Compile([]byte(doc), Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	treeJSON, err := prog.Render(nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(treeJSON), `"title":"static"`) {
		t.Errorf("missing title in %s", treeJSON)
	}
}

// A non-Document root must produce a clear error from the serializer.
func TestRenderRejectsNonDocumentRoot(t *testing.T) {
	const doc = `
import { View } from '@feast/react';
export default <View />;
`
	prog, err := Compile([]byte(doc), Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if _, err := prog.Render(nil); err == nil {
		t.Fatal("expected error rendering a non-Document root, got nil")
	}
}
