package render

import (
	"bytes"
	"strings"
	"testing"
)

// An underlined Text strokes a decoration line after the text object (ET).
func TestRenderTextUnderline(t *testing.T) {
	res := layoutFromJSON(t, `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,80]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":14,"textDecoration":"underline"}},"children":[
				{"type":"TEXT_INSTANCE","value":"Underlined"}
			]}
		]}
	]}}`)
	var buf bytes.Buffer
	if err := Render(res, &buf, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, buf.Bytes())
	et := strings.Index(content, "ET\n")
	stroke := strings.LastIndex(content, "S\n")
	if et < 0 || stroke < 0 {
		t.Fatalf("expected text (ET) and a stroked decoration (S):\n%s", content)
	}
	if stroke < et {
		t.Errorf("decoration stroke must come after the text object (ET@%d S@%d)", et, stroke)
	}
	if !strings.Contains(content[et:], "l\n") {
		t.Errorf("underline should draw a line (l) after ET:\n%s", content[et:])
	}
}

// line-through strokes a decoration too.
func TestRenderTextLineThrough(t *testing.T) {
	res := layoutFromJSON(t, `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,80]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":14,"textDecoration":"line-through"}},"children":[
				{"type":"TEXT_INSTANCE","value":"Struck"}
			]}
		]}
	]}}`)
	var buf bytes.Buffer
	if err := Render(res, &buf, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, buf.Bytes())
	if strings.LastIndex(content, "S\n") < strings.Index(content, "ET\n") {
		t.Errorf("line-through should stroke after the text object:\n%s", content)
	}
}

// Plain text (no decoration) strokes nothing extra.
func TestRenderTextNoDecoration(t *testing.T) {
	res := layoutFromJSON(t, `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,80]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":14}},"children":[
				{"type":"TEXT_INSTANCE","value":"Plain"}
			]}
		]}
	]}}`)
	var buf bytes.Buffer
	if err := Render(res, &buf, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, buf.Bytes())
	if strings.Contains(content, "S\n") {
		t.Errorf("plain text should not stroke a decoration line:\n%s", content)
	}
}
