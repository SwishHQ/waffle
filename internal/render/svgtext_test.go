package render

import (
	"bytes"
	"strings"
	"testing"
)

func renderSVGText(t *testing.T, textNode string) string {
	t.Helper()
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"SVG","props":{"style":{"width":100,"height":100},"viewBox":"0 0 100 100"},"children":[
				` + textNode + `
			]}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	return inflateStreams(t, out.Bytes())
}

// An SVG <Text> paints a text run with a local y-flip (so it's upright under the
// flipped SVG CTM), the correct fill, and the string content.
func TestRenderSVGText(t *testing.T) {
	content := renderSVGText(t, `{"type":"TEXT","props":{"x":10,"y":50,"fontSize":12,"fill":"#ff0000"},"children":[{"type":"TEXT_INSTANCE","value":"Hi"}]}`)
	if !strings.Contains(content, "1 0 0 -1 10 50 Tm") {
		t.Errorf("SVG text should set a y-flipped text matrix at (10,50):\n%s", content)
	}
	if !strings.Contains(content, "(Hi) Tj") {
		t.Errorf("SVG text should show the string:\n%s", content)
	}
	if !strings.Contains(content, "1 0 0 rg") {
		t.Errorf("SVG text fill should be red:\n%s", content)
	}
}

// text-anchor:middle shifts the origin left relative to the default start anchor.
func TestRenderSVGTextAnchorMiddle(t *testing.T) {
	start := renderSVGText(t, `{"type":"TEXT","props":{"x":50,"y":50,"fontSize":12,"textAnchor":"start"},"children":[{"type":"TEXT_INSTANCE","value":"Centered"}]}`)
	middle := renderSVGText(t, `{"type":"TEXT","props":{"x":50,"y":50,"fontSize":12,"textAnchor":"middle"},"children":[{"type":"TEXT_INSTANCE","value":"Centered"}]}`)
	if !strings.Contains(start, "1 0 0 -1 50 50 Tm") {
		t.Errorf("start anchor should place origin at x=50:\n%s", start)
	}
	if strings.Contains(middle, "1 0 0 -1 50 50 Tm") {
		t.Errorf("middle anchor should shift x off 50:\n%s", middle)
	}
}

// fill:none makes the text invisible (no text object emitted).
func TestRenderSVGTextFillNone(t *testing.T) {
	content := renderSVGText(t, `{"type":"TEXT","props":{"x":10,"y":50,"fill":"none"},"children":[{"type":"TEXT_INSTANCE","value":"Hidden"}]}`)
	if strings.Contains(content, "(Hidden)") {
		t.Errorf("fill:none SVG text should not be drawn:\n%s", content)
	}
}
