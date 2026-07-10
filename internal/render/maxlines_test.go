package render

import (
	"bytes"
	"strings"
	"testing"
)

// A maxLines + ellipsis Text truncates to N lines and renders a trailing ellipsis.
// The ellipsis glyph (WinAnsi 0x85) is escaped as octal \205 in the literal string.
func TestRenderMaxLinesEllipsis(t *testing.T) {
	res := layoutFromJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[120,120]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":12,"width":70,"maxLines":2,"textOverflow":"ellipsis"}},"children":[
				{"type":"TEXT_INSTANCE","value":"alpha beta gamma delta epsilon zeta eta theta"}
			]}
		]}
	]}}`)
	var buf bytes.Buffer
	if err := Render(res, &buf, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, buf.Bytes())
	if !strings.Contains(content, `\205`) {
		t.Errorf("truncated text should render an ellipsis (octal \\205):\n%q", content)
	}
	// Only two lines are shown: three Td/TJ shows would mean no truncation.
	if n := strings.Count(content, " Tj\n"); n != 2 {
		t.Errorf("maxLines=2 should show exactly 2 lines, got %d Tj ops", n)
	}
}
