package tree

import (
	"strings"
	"testing"

	"github.com/swish/waffle/internal/contract"
)

// mk builds a node of the given type with children, wiring Parent pointers.
func mk(typ string, children ...*Node) *Node {
	n := &Node{Type: typ}
	for _, c := range children {
		c.Parent = n
		n.Children = append(n.Children, c)
	}
	return n
}

func inst(v string) *Node { return &Node{Type: contract.TypeTextInstance, Value: v} }

func TestBuildFromContract(t *testing.T) {
	tr, err := contract.Parse([]byte(sampleTree))
	if err != nil {
		t.Fatalf("contract.Parse: %v", err)
	}
	built, err := Build(tr)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if built.Root.Type != contract.TypeDocument {
		t.Fatalf("root type = %s", built.Root.Type)
	}
	if len(built.Fonts) != 1 || built.Fonts[0].Family != "Roboto" {
		t.Errorf("fonts not carried: %+v", built.Fonts)
	}
	if built.HyphenationCallback == nil {
		t.Errorf("hyphenation callback not carried")
	}

	// Navigate to the fixed Text with the render callback.
	page := built.Root.Children[0]
	view := page.Children[0]
	fixedText := view.Children[1]
	if !fixedText.Fixed {
		t.Errorf("expected Fixed=true on second Text")
	}
	if fixedText.Render == nil || fixedText.Render.ID != "cb_1" {
		t.Errorf("render callback = %v, want cb_1", fixedText.Render)
	}
	// Wrap is unset on this node.
	if fixedText.Wrap != nil {
		t.Errorf("Wrap = %v, want nil (unset)", *fixedText.Wrap)
	}
	// Parent pointers wired.
	if fixedText.Parent != view || view.Parent != page || page.Parent != built.Root {
		t.Errorf("parent pointers not wired correctly")
	}
}

// sampleTree mirrors the contract package's fixture (kept local to avoid
// cross-package test coupling).
const sampleTree = `{
  "version": "waffle-tree/v1",
  "callbacks": ["cb_0", "cb_1"],
  "document": {
    "props": { "title": "Invoice" },
    "fonts": [ { "family": "Roboto", "fonts": [ { "src": "https://x/Roboto.ttf", "fontWeight": 700 } ] } ],
    "hyphenationCallback": { "$cb": "cb_0" },
    "children": [
      { "type": "PAGE", "props": { "size": "A4" }, "children": [
        { "type": "VIEW", "props": {}, "children": [
          { "type": "TEXT", "props": {}, "children": [ {"type":"TEXT_INSTANCE","value":"Hello"} ] },
          { "type": "TEXT", "props": { "fixed": true, "render": {"$cb":"cb_1"} } }
        ]}
      ]}
    ]
  }
}`

func TestValidateAcceptsWellFormed(t *testing.T) {
	root := mk(contract.TypeDocument,
		mk(contract.TypePage,
			mk(contract.TypeView,
				mk(contract.TypeText, inst("a"), mk(contract.TypeText, inst("b")), mk(contract.TypeLink, inst("c"))),
				mk(contract.TypeSvg, mk(contract.TypeG, mk(contract.TypePath))),
			),
		),
	)
	if err := Validate(root); err != nil {
		t.Fatalf("Validate rejected a valid tree: %v", err)
	}
}

func TestValidateRules(t *testing.T) {
	cases := []struct {
		name  string
		root  *Node
		wants string
	}{
		{
			name:  "page not under document",
			root:  mk(contract.TypeDocument, mk(contract.TypePage, mk(contract.TypeView, mk(contract.TypePage)))),
			wants: "Page must be a direct child of Document",
		},
		{
			name:  "view inside text",
			root:  mk(contract.TypeDocument, mk(contract.TypePage, mk(contract.TypeText, mk(contract.TypeView)))),
			wants: "not allowed inside Text",
		},
		{
			name:  "note with non-text child",
			root:  mk(contract.TypeDocument, mk(contract.TypePage, mk(contract.TypeNote, mk(contract.TypeView)))),
			wants: "Note may only contain text",
		},
		{
			name:  "svg primitive outside svg",
			root:  mk(contract.TypeDocument, mk(contract.TypePage, mk(contract.TypeView, mk(contract.TypeRect)))),
			wants: "only valid inside an Svg",
		},
		{
			name:  "text string outside text",
			root:  mk(contract.TypeDocument, mk(contract.TypePage, mk(contract.TypeView, inst("loose")))),
			wants: "must be inside Text",
		},
		{
			name:  "non-page under document",
			root:  mk(contract.TypeDocument, mk(contract.TypeView)),
			wants: "not allowed inside Document",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Validate(c.root)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.wants)
			}
			if !strings.Contains(err.Error(), c.wants) {
				t.Errorf("error = %q, want substring %q", err.Error(), c.wants)
			}
		})
	}
}

func TestValidateRootMustBeDocument(t *testing.T) {
	if err := Validate(mk(contract.TypePage)); err == nil {
		t.Errorf("expected error for non-Document root")
	}
}

func TestCloneIsDeep(t *testing.T) {
	wrap := true
	orig := mk(contract.TypeDocument,
		mk(contract.TypePage,
			mk(contract.TypeView, inst("x")),
		),
	)
	orig.Children[0].Children[0].Wrap = &wrap
	orig.Children[0].Children[0].Props = map[string]any{"k": "v"}

	clone := orig.Clone()

	// Mutating the original must not affect the clone.
	origView := orig.Children[0].Children[0]
	origView.Value = "MUTATED"
	origView.Children[0].Value = "MUTATED"
	*origView.Wrap = false
	origView.Props["k"] = "MUTATED"

	cloneView := clone.Children[0].Children[0]
	if cloneView.Children[0].Value != "x" {
		t.Errorf("clone child value = %q, want x (deep copy failed)", cloneView.Children[0].Value)
	}
	if cloneView.Wrap == nil || *cloneView.Wrap != true {
		t.Errorf("clone Wrap pointer was shared with original")
	}
	if cloneView.Props["k"] != "v" {
		t.Errorf("clone Props map was shared with original")
	}
	// Parent pointers in the clone point within the clone, not the original.
	if cloneView.Parent == origView.Parent {
		t.Errorf("clone parent pointer still references the original tree")
	}
	if clone.Children[0].Parent != clone {
		t.Errorf("clone parent pointers not rewired to the clone root")
	}
}

func TestPath(t *testing.T) {
	root := mk(contract.TypeDocument,
		mk(contract.TypePage,
			mk(contract.TypeView),
			mk(contract.TypeView, mk(contract.TypeText)),
		),
	)
	text := root.Children[0].Children[1].Children[0]
	if got, want := Path(text), "Document > Page[0] > View[1] > Text[0]"; got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}
