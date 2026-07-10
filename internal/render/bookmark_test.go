package render

import (
	"bytes"
	"strings"
	"testing"
)

// bookmark props (string form and object form) become an /Outlines tree, with a
// bookmarked child nesting under its bookmarked ancestor.
func TestRenderBookmarks(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,400]},"children":[
			{"type":"VIEW","props":{"bookmark":"Intro","style":{"height":50}}},
			{"type":"VIEW","props":{"bookmark":{"title":"Body"},"style":{"height":300}},"children":[
				{"type":"VIEW","props":{"bookmark":"Body detail","style":{"height":40}}}
			]}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var buf bytes.Buffer
	if err := Render(res, &buf, Options{}); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	for _, want := range []string{"/Type /Outlines", "(Intro)", "(Body)", "(Body detail)"} {
		if !strings.Contains(s, want) {
			t.Errorf("PDF missing %q:\n%s", want, s)
		}
	}
	// "Body detail" nests under "Body": the Body item must carry a /First pointer.
	// Root /Count counts Intro + Body + Body detail = 3.
	if !strings.Contains(s, "/Count 3") {
		t.Errorf("expected root outline /Count 3 (nested child counted):\n%s", s)
	}
}

// No bookmark props => no outline.
func TestRenderNoBookmarks(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"VIEW","props":{"style":{"height":50}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var buf bytes.Buffer
	if err := Render(res, &buf, Options{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "/Outlines") {
		t.Error("no bookmarks should mean no /Outlines")
	}
}
