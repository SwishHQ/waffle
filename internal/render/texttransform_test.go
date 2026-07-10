package render

import (
	"bytes"
	"strings"
	"testing"
)

// textTransform:uppercase rewrites the shown text; the content stream carries the
// transformed literal, and widths follow because it happens before wrapping.
func TestRenderTextTransformUppercase(t *testing.T) {
	res := layoutFromJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[300,80]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":14,"textTransform":"uppercase"}},"children":[
				{"type":"TEXT_INSTANCE","value":"Hello waffle"}
			]}
		]}
	]}}`)
	var buf bytes.Buffer
	if err := Render(res, &buf, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, buf.Bytes())
	if !strings.Contains(content, "(HELLO WAFFLE)") {
		t.Errorf("expected uppercased literal (HELLO WAFFLE):\n%s", content)
	}
	if strings.Contains(content, "(Hello waffle)") {
		t.Errorf("original casing should not be shown:\n%s", content)
	}
}
