package render

import (
	"bytes"
	"strings"
	"testing"
)

// A <TextInput name value> renders an AcroForm text-field widget.
func TestRenderTextInput(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[300,200]},"children":[
			{"type":"TEXT_INPUT","props":{"name":"fullName","value":"Ada Lovelace","style":{"width":200,"height":24,"fontSize":12}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"/AcroForm", "/FT /Tx", "(fullName)", "(Ada Lovelace)", "/Subtype /Widget"} {
		if !strings.Contains(s, want) {
			t.Errorf("PDF missing %q:\n%s", want, s)
		}
	}
}

// A TextInput without a name is skipped (no field, no AcroForm).
func TestRenderTextInputNoName(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[300,200]},"children":[
			{"type":"TEXT_INPUT","props":{"value":"x","style":{"width":100,"height":20}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "/AcroForm") {
		t.Error("a nameless TextInput should not create an AcroForm")
	}
}

// A <Checkbox name checked> renders an AcroForm button widget with appearances.
func TestRenderCheckbox(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"CHECKBOX","props":{"name":"subscribe","checked":true,"style":{"width":16,"height":16}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"/FT /Btn", "(subscribe)", "/AS /Yes", "/AP"} {
		if !strings.Contains(s, want) {
			t.Errorf("PDF missing %q:\n%s", want, s)
		}
	}
}

// A <Select options> renders a combo (dropdown) choice field; <List> a list box.
func TestRenderSelectAndList(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[300,300]},"children":[
			{"type":"SELECT","props":{"name":"lang","value":"go","options":["go","rust",{"value":"ts","label":"TypeScript"}],"style":{"width":150,"height":20}}},
			{"type":"LIST","props":{"name":"tags","options":["a","b"],"style":{"width":150,"height":40}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"/FT /Ch", "(lang)", "(TypeScript)", "(ts)", "(tags)", "/Ff 131072"} {
		if !strings.Contains(s, want) {
			t.Errorf("PDF missing %q:\n%s", want, s)
		}
	}
	// The list box (tags) must NOT have the combo flag; only the Select does.
	if strings.Count(s, "/Ff 131072") != 1 {
		t.Errorf("exactly one combo flag expected (Select, not List):\n%s", s)
	}
}
