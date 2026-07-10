package render

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func TestParseSVGTransform(t *testing.T) {
	m, ok := parseSVGTransform("translate(10, 20)")
	if !ok || m.E != 10 || m.F != 20 {
		t.Errorf("translate: %+v ok=%v", m, ok)
	}
	m, _ = parseSVGTransform("scale(2)")
	if m.A != 2 || m.D != 2 {
		t.Errorf("scale(2) should be uniform: %+v", m)
	}
	m, _ = parseSVGTransform("matrix(1 2 3 4 5 6)")
	if m.A != 1 || m.B != 2 || m.C != 3 || m.D != 4 || m.E != 5 || m.F != 6 {
		t.Errorf("matrix passthrough failed: %+v", m)
	}
	// Composition: translate then scale multiplies in order.
	m, _ = parseSVGTransform("translate(5,0) scale(2)")
	x, y := m.Apply(1, 1)
	if math.Abs(x-7) > 1e-9 || math.Abs(y-2) > 1e-9 {
		t.Errorf("composed transform Apply(1,1) = (%v,%v), want (7,2)", x, y)
	}
	if _, ok := parseSVGTransform("garbage"); ok {
		t.Error("non-transform string should not parse")
	}
}

// A <G transform> concatenates a CTM around its children.
func TestRenderSVGGroupTransform(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"SVG","props":{"style":{"width":100,"height":100},"viewBox":"0 0 100 100"},"children":[
				{"type":"G","props":{"transform":"translate(10, 20)"},"children":[
					{"type":"RECT","props":{"x":0,"y":0,"width":10,"height":10,"fill":"#000000"}}
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
	if !strings.Contains(content, "1 0 0 1 10 20 cm") {
		t.Errorf("group transform should emit its CTM:\n%s", content)
	}
}
