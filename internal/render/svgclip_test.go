package render

import (
	"bytes"
	"strings"
	"testing"
)

// A shape referencing clip-path="url(#id)" clips to the ClipPath's shapes: the
// content stream emits the clip path (W n) before painting the shape.
func TestRenderSVGClipPath(t *testing.T) {
	doc := `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"SVG","props":{"style":{"width":100,"height":100},"viewBox":"0 0 100 100"},"children":[
				{"type":"DEFS","children":[
					{"type":"CLIP_PATH","props":{"id":"clip"},"children":[
						{"type":"CIRCLE","props":{"cx":50,"cy":50,"r":40}}
					]}
				]},
				{"type":"RECT","props":{"x":0,"y":0,"width":100,"height":100,"fill":"#ff0000","clipPath":"url(#clip)"}}
			]}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	if !strings.Contains(content, "W\nn\n") {
		t.Errorf("clip-path should intersect a clip region (W n):\n%s", content)
	}
	// The clip circle uses beziers; a curve op should appear before the rect's fill.
	clip := strings.Index(content, " c\n")
	fill := strings.Index(content, "1 0 0 rg")
	if clip < 0 || fill < 0 || clip > fill {
		t.Errorf("clip shape should be emitted before the clipped rect fill:\n%s", content)
	}
}

// A shape without clip-path is unaffected (no clip emitted around it).
func TestRenderSVGNoClipPath(t *testing.T) {
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
	if strings.Contains(inflateStreams(t, out.Bytes()), "W\nn\n") {
		t.Error("a shape without clip-path should not emit a clip")
	}
}
