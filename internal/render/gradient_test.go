package render

import (
	"bytes"
	"strings"
	"testing"
)

// An SVG rect filled with a linear gradient must clip to the shape and paint a
// shading (sh), and the page must declare an axial /Shading resource.
func TestRenderSVGLinearGradient(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"SVG","props":{"style":{"width":100,"height":100},"viewBox":"0 0 100 100"},"children":[
				{"type":"DEFS","children":[
					{"type":"LINEAR_GRADIENT","props":{"id":"g","x1":0,"y1":0,"x2":1,"y2":0},"children":[
						{"type":"STOP","props":{"offset":0,"stopColor":"red"}},
						{"type":"STOP","props":{"offset":1,"stopColor":"blue"}}
					]}
				]},
				{"type":"RECT","props":{"x":0,"y":0,"width":100,"height":100,"fill":"url(#g)"}}
			]}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	if !strings.Contains(content, " sh\n") {
		t.Errorf("gradient fill should emit a shading (sh) op:\n%s", content)
	}
	if !strings.Contains(content, "W\nn\n") {
		t.Errorf("gradient fill should clip to the shape (W n):\n%s", content)
	}
	if !strings.Contains(out.String(), "/ShadingType 2") {
		t.Error("page should declare an axial /Shading")
	}
}

// A radial gradient uses a ShadingType 3.
func TestRenderSVGRadialGradient(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"SVG","props":{"style":{"width":80,"height":80},"viewBox":"0 0 80 80"},"children":[
				{"type":"DEFS","children":[
					{"type":"RADIAL_GRADIENT","props":{"id":"r","cx":0.5,"cy":0.5,"r":0.5},"children":[
						{"type":"STOP","props":{"offset":0,"stopColor":"#ffffff"}},
						{"type":"STOP","props":{"offset":1,"stopColor":"#000000"}}
					]}
				]},
				{"type":"CIRCLE","props":{"cx":40,"cy":40,"r":40,"fill":"url(#r)"}}
			]}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "/ShadingType 3") {
		t.Error("radial gradient should declare a ShadingType 3")
	}
}

// A solid-fill SVG is unchanged: no shading, an ordinary rg fill.
func TestRenderSVGSolidFillNoShading(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"SVG","props":{"style":{"width":100,"height":100},"viewBox":"0 0 100 100"},"children":[
				{"type":"RECT","props":{"x":0,"y":0,"width":50,"height":50,"fill":"#00ff00"}}
			]}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "/Shading") {
		t.Error("a solid-fill SVG should not declare a /Shading")
	}
	if !strings.Contains(inflateStreams(t, out.Bytes()), " rg\n") {
		t.Error("solid fill should still emit an rg color")
	}
}
