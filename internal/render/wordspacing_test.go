package render

import (
	"bytes"
	"strings"
	"testing"
)

// wordSpacing emits a Tw (word-spacing) operator with the resolved value.
func TestRenderWordSpacing(t *testing.T) {
	res := layoutFromJSON(t, `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[400,80]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":14,"wordSpacing":6}},"children":[
				{"type":"TEXT_INSTANCE","value":"one two three"}
			]}
		]}
	]}}`)
	var buf bytes.Buffer
	if err := Render(res, &buf, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, buf.Bytes())
	if !strings.Contains(content, "6 Tw\n") {
		t.Errorf("wordSpacing 6 should emit '6 Tw':\n%s", content)
	}
}
