package render

import (
	"bytes"
	"strings"
	"testing"
)

// A stroked SVG line with stroke-dasharray/linecap/linejoin emits the dash
// pattern, round cap, and round join operators.
func TestRenderSVGStrokeStyle(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"SVG","props":{"style":{"width":100,"height":100},"viewBox":"0 0 100 100"},"children":[
				{"type":"POLYLINE","props":{"points":"10,10 50,80 90,20","fill":"none","stroke":"#000000","strokeWidth":3,"strokeDasharray":"6,4","strokeLinecap":"round","strokeLinejoin":"round"}}
			]}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	for _, want := range []string{"[6 4] 0 d\n", "1 J\n", "1 j\n", "S\n"} {
		if !strings.Contains(content, want) {
			t.Errorf("stroke styling should emit %q:\n%s", want, content)
		}
	}
}

// A plain stroked shape emits no dash pattern and the scoping does not leak.
func TestRenderSVGStrokeNoDash(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"SVG","props":{"style":{"width":100,"height":100},"viewBox":"0 0 100 100"},"children":[
				{"type":"LINE","props":{"x1":0,"y1":0,"x2":50,"y2":50,"stroke":"#ff0000","strokeWidth":2}}
			]}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(inflateStreams(t, out.Bytes()), "] 0 d\n") {
		t.Error("a solid stroke should not emit a dash pattern")
	}
}
