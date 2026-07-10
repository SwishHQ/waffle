package render

import (
	"bytes"
	"strings"
	"testing"
)

// A shape with fill-opacity paints through an ExtGState alpha (gs) < 1.
func TestRenderSVGFillOpacity(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"SVG","props":{"style":{"width":100,"height":100},"viewBox":"0 0 100 100"},"children":[
				{"type":"RECT","props":{"x":0,"y":0,"width":50,"height":50,"fill":"#ff0000","fillOpacity":0.4}}
			]}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inflateStreams(t, out.Bytes()), " gs\n") {
		t.Errorf("fill-opacity should set an ExtGState alpha (gs):\n%s", inflateStreams(t, out.Bytes()))
	}
	if !strings.Contains(out.String(), "/ca 0.4") {
		t.Errorf("expected a 0.4 constant alpha in the ExtGState:\n%s", out.String())
	}
}

// A fully-opaque shape emits no gs.
func TestRenderSVGNoOpacity(t *testing.T) {
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
	if strings.Contains(inflateStreams(t, out.Bytes()), " gs\n") {
		t.Error("opaque shape should not set an ExtGState alpha")
	}
}
