package contract

import (
	"encoding/json"
	"strings"
	"testing"
)

const sampleTree = `{
  "version": "feast-tree/v1",
  "callbacks": ["cb_0", "cb_1"],
  "document": {
    "props": { "title": "Invoice", "pdfVersion": "1.4" },
    "fonts": [
      { "family": "Roboto", "fonts": [
        { "src": "https://example.com/Roboto.ttf", "fontWeight": 700 },
        { "src": { "$inline": "QUJD" }, "fontStyle": "italic" }
      ]}
    ],
    "hyphenationCallback": { "$cb": "cb_0" },
    "children": [
      { "type": "PAGE",
        "props": { "size": "A4", "style": [ {"flexDirection":"row"}, {"padding":"10mm"} ] },
        "children": [
          { "type": "VIEW", "props": { "style": {"flexGrow": 1} }, "children": [
            { "type": "TEXT", "props": {}, "children": [ {"type":"TEXT_INSTANCE","value":"Hello"} ] },
            { "type": "TEXT", "props": { "fixed": true, "render": {"$cb":"cb_1"} } }
          ]}
        ]}
    ]
  }
}`

func TestParseSample(t *testing.T) {
	tree, err := Parse([]byte(sampleTree))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if tree.Version != "feast-tree/v1" {
		t.Errorf("version = %q", tree.Version)
	}
	if got := tree.Document.Props["title"]; got != "Invoice" {
		t.Errorf("title = %v", got)
	}

	// Fonts, including inline asset and preserved numeric weight.
	if len(tree.Document.Fonts) != 1 {
		t.Fatalf("fonts = %d, want 1", len(tree.Document.Fonts))
	}
	f := tree.Document.Fonts[0]
	if f.Family != "Roboto" || len(f.Faces) != 2 {
		t.Fatalf("font = %+v", f)
	}
	if src, ok := f.Faces[0].Src.(string); !ok || src != "https://example.com/Roboto.ttf" {
		t.Errorf("face0 src = %v", f.Faces[0].Src)
	}
	if n, ok := f.Faces[0].FontWeight.(json.Number); !ok || n.String() != "700" {
		t.Errorf("face0 weight = %v (%T), want json.Number 700", f.Faces[0].FontWeight, f.Faces[0].FontWeight)
	}
	if inline, ok := f.Faces[1].Src.(InlineAsset); !ok || inline.Base64 != "QUJD" {
		t.Errorf("face1 src = %v, want InlineAsset", f.Faces[1].Src)
	}
	if f.Faces[1].FontStyle != "italic" {
		t.Errorf("face1 style = %q", f.Faces[1].FontStyle)
	}

	// Document-level callback.
	if tree.Document.HyphenationCallback == nil || tree.Document.HyphenationCallback.ID != "cb_0" {
		t.Errorf("hyphenationCallback = %v", tree.Document.HyphenationCallback)
	}

	// Tree shape: PAGE > VIEW > [TEXT > TEXT_INSTANCE, TEXT(fixed, render cb)].
	page := tree.Document.Children[0]
	if page.Type != TypePage {
		t.Fatalf("child0 type = %q", page.Type)
	}
	if style, ok := page.Props["style"].([]any); !ok || len(style) != 2 {
		t.Errorf("page style = %v, want 2-element array", page.Props["style"])
	}
	view := page.Children[0]
	textA := view.Children[0]
	if textA.Type != TypeText || len(textA.Children) != 1 || textA.Children[0].Type != TypeTextInstance {
		t.Fatalf("textA = %+v", textA)
	}
	if textA.Children[0].Value != "Hello" {
		t.Errorf("text instance value = %q", textA.Children[0].Value)
	}
	textB := view.Children[1]
	if fixed, ok := textB.Props["fixed"].(bool); !ok || !fixed {
		t.Errorf("textB fixed = %v", textB.Props["fixed"])
	}
	if cb, ok := textB.Props["render"].(CallbackRef); !ok || cb.ID != "cb_1" {
		t.Errorf("textB render = %v, want CallbackRef cb_1", textB.Props["render"])
	}

	if !tree.RequiresEvaluator() {
		t.Errorf("RequiresEvaluator = false, want true (render + hyphenation callbacks present)")
	}
	if len(tree.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", tree.Warnings)
	}
}

func TestVersionGating(t *testing.T) {
	cases := []struct {
		name    string
		version string
		wantErr bool
	}{
		{"v1", "feast-tree/v1", false},
		{"v1 minor", "feast-tree/v1.5", false},
		{"v2", "feast-tree/v2", true},
		{"missing", "", true},
		{"garbage", "not-a-version", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := `{"version":` + jsonString(c.version) + `,"document":{"children":[]}}`
			if c.version == "" {
				doc = `{"document":{"children":[]}}`
			}
			_, err := Parse([]byte(doc))
			if (err != nil) != c.wantErr {
				t.Errorf("Parse(version=%q) err = %v, wantErr = %v", c.version, err, c.wantErr)
			}
		})
	}
}

func TestUnknownTypeWarns(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"BLINK","props":{}}
	]}}`
	tree, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(tree.Warnings) == 0 || !strings.Contains(tree.Warnings[0], "unknown node type") {
		t.Errorf("warnings = %v, want an unknown-node-type warning", tree.Warnings)
	}
	// Unknown types still parse into the tree.
	if len(tree.Document.Children) != 1 || tree.Document.Children[0].Type != "BLINK" {
		t.Errorf("unknown node not preserved: %+v", tree.Document.Children)
	}
}

func TestStaticTreeNeedsNoEvaluator(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","children":[{"type":"TEXT","children":[{"type":"TEXT_INSTANCE","value":"hi"}]}]}
	]}}`
	tree, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if tree.RequiresEvaluator() {
		t.Errorf("RequiresEvaluator = true, want false for a callback-free tree")
	}
	if len(tree.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", tree.Warnings)
	}
}

func TestCallbackConsistencyWarnings(t *testing.T) {
	// cb_1 referenced but not declared; cb_9 declared but not referenced.
	doc := `{"version":"feast-tree/v1","callbacks":["cb_9"],"document":{"children":[
		{"type":"TEXT","props":{"render":{"$cb":"cb_1"}}}
	]}}`
	tree, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var referencedNotDeclared, declaredNotReferenced bool
	for _, w := range tree.Warnings {
		if strings.Contains(w, `"cb_1"`) && strings.Contains(w, "not declared") {
			referencedNotDeclared = true
		}
		if strings.Contains(w, `"cb_9"`) && strings.Contains(w, "not referenced") {
			declaredNotReferenced = true
		}
	}
	if !referencedNotDeclared || !declaredNotReferenced {
		t.Errorf("missing consistency warnings, got %v", tree.Warnings)
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
