package render

import (
	"bytes"
	"strings"
	"testing"
)

// A <Note> element produces a text (sticky-note) annotation carrying its text.
func TestRenderNote(t *testing.T) {
	doc := `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"NOTE","props":{"style":{"position":"absolute","top":20,"left":30}},"children":[
				{"type":"TEXT_INSTANCE","value":"Check this figure"}
			]}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"/Subtype /Text", "(Check this figure)", "/Name /Note"} {
		if !strings.Contains(s, want) {
			t.Errorf("PDF missing %q:\n%s", want, s)
		}
	}
}

// No <Note> => no text annotation.
func TestRenderNoNote(t *testing.T) {
	doc := `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"VIEW","props":{"style":{"height":10}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "/Subtype /Text") {
		t.Error("no Note element should mean no Text annotation")
	}
}
