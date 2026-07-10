package render

import (
	"bytes"
	"strings"
	"testing"
)

// A later sibling with a lower zIndex paints BEFORE an earlier sibling with a
// higher zIndex — verified by fill-color order in the content stream.
func TestRenderZIndexOrder(t *testing.T) {
	// Child A (document-first) has zIndex 5 and is red; child B has zIndex 1 and
	// is blue. Paint order should be B (blue) then A (red).
	doc := `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"VIEW","props":{"style":{"position":"absolute","top":0,"left":0,"width":40,"height":40,"backgroundColor":"#ff0000","zIndex":5}}},
			{"type":"VIEW","props":{"style":{"position":"absolute","top":0,"left":0,"width":40,"height":40,"backgroundColor":"#0000ff","zIndex":1}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	blue := strings.Index(content, "0 0 1 rg")
	red := strings.Index(content, "1 0 0 rg")
	if blue < 0 || red < 0 {
		t.Fatalf("both fills should be present:\n%s", content)
	}
	if blue > red {
		t.Errorf("zIndex 1 (blue) should paint before zIndex 5 (red); got blue@%d red@%d", blue, red)
	}
}

// Without zIndex, document order is preserved (first child paints first).
func TestRenderNoZIndexDocOrder(t *testing.T) {
	doc := `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"VIEW","props":{"style":{"position":"absolute","top":0,"left":0,"width":40,"height":40,"backgroundColor":"#ff0000"}}},
			{"type":"VIEW","props":{"style":{"position":"absolute","top":0,"left":0,"width":40,"height":40,"backgroundColor":"#0000ff"}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	if strings.Index(content, "1 0 0 rg") > strings.Index(content, "0 0 1 rg") {
		t.Error("without zIndex, red (first) should paint before blue (second)")
	}
}
