package render

import (
	"bytes"
	"strings"
	"testing"
)

// letterSpacing emits a Tc (character-spacing) operator with the resolved value.
func TestRenderLetterSpacing(t *testing.T) {
	res := layoutFromJSON(t, `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[300,80]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":14,"letterSpacing":3}},"children":[
				{"type":"TEXT_INSTANCE","value":"Spaced"}
			]}
		]}
	]}}`)
	var buf bytes.Buffer
	if err := Render(res, &buf, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, buf.Bytes())
	if !strings.Contains(content, "3 Tc\n") {
		t.Errorf("letterSpacing 3 should emit '3 Tc':\n%s", content)
	}
}

// Without letterSpacing, no Tc is emitted (default character spacing is 0).
func TestRenderNoLetterSpacing(t *testing.T) {
	res := layoutFromJSON(t, `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[300,80]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":14}},"children":[
				{"type":"TEXT_INSTANCE","value":"Plain"}
			]}
		]}
	]}}`)
	var buf bytes.Buffer
	if err := Render(res, &buf, Options{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(inflateStreams(t, buf.Bytes()), " Tc\n") {
		t.Error("default text should not emit a Tc operator")
	}
}
