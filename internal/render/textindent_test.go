package render

import (
	"bytes"
	"strings"
	"testing"
)

// textIndent pushes only the first wrapped line to the right; later lines start
// at the paragraph edge.
func TestRenderTextIndent(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,120]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":12,"width":150,"textIndent":20}},"children":[
				{"type":"TEXT_INSTANCE","value":"one two three four five six seven eight nine ten eleven twelve"}
			]}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	// First line starts at x=20 (the indent); at least one later line starts at x=0.
	if !strings.Contains(content, "1 0 0 1 20 ") {
		t.Errorf("first line should start at the 20pt indent:\n%s", content)
	}
	if !strings.Contains(content, "1 0 0 1 0 ") {
		t.Errorf("a later line should start at x=0 (no indent):\n%s", content)
	}
}

// Without textIndent, every line starts at x=0.
func TestRenderNoTextIndent(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,120]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":12,"width":150}},"children":[
				{"type":"TEXT_INSTANCE","value":"one two three four five six seven eight nine ten eleven"}
			]}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	if strings.Contains(content, "1 0 0 1 20 ") {
		t.Errorf("no textIndent should mean no 20pt first-line offset:\n%s", content)
	}
}
