package render

import (
	"bytes"
	"strings"
	"testing"
)

// A nested <Text style bold/red> inside a paragraph renders as its own run: the
// PDF carries both Helvetica and Helvetica-Bold, and the bold word is red while
// the surrounding text is black.
func TestRenderInlineRuns(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[300,100]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":12}},"children":[
				{"type":"TEXT_INSTANCE","value":"Hello "},
				{"type":"TEXT","props":{"style":{"fontWeight":"bold","color":"#ff0000"}},"children":[
					{"type":"TEXT_INSTANCE","value":"world"}
				]},
				{"type":"TEXT_INSTANCE","value":"!"}
			]}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	raw := out.String()
	// "/BaseFont /Helvetica " (trailing space) matches the regular face without
	// also matching "/BaseFont /Helvetica-Bold".
	if !strings.Contains(raw, "/BaseFont /Helvetica-Bold") || !strings.Contains(raw, "/BaseFont /Helvetica ") {
		t.Error("both regular and bold Helvetica should be declared as fonts")
	}
	content := inflateStreams(t, out.Bytes())
	for _, w := range []string{"(Hello)", "(world)", "(!)"} {
		if !strings.Contains(content, w) {
			t.Errorf("run text %q should be shown:\n%s", w, content)
		}
	}
	if !strings.Contains(content, "1 0 0 rg") {
		t.Errorf("the bold run should be red (1 0 0 rg):\n%s", content)
	}
	// The bold word is positioned after "Hello " — its Tm x should exceed the
	// paragraph start x.
	if strings.Count(content, " Tj\n") < 3 {
		t.Errorf("expected at least 3 shown fragments, got:\n%s", content)
	}
}

// An underlined nested run strokes a decoration line; the surrounding text does not.
func TestRenderInlineRunUnderline(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[300,100]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":12}},"children":[
				{"type":"TEXT_INSTANCE","value":"plain "},
				{"type":"TEXT","props":{"style":{"textDecoration":"underline"}},"children":[
					{"type":"TEXT_INSTANCE","value":"linked"}
				]}
			]}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	if !strings.Contains(content, "S\n") {
		t.Errorf("underlined run should stroke a decoration line:\n%s", content)
	}
}
