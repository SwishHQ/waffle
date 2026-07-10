package render

import (
	"bytes"
	"strings"
	"testing"
)

func transformDoc(tf string) string {
	return `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"VIEW","props":{"style":{"width":40,"height":40,"backgroundColor":"#ff0000","transform":"` + tf + `"}}}
		]}
	]}}`
}

// A CSS translate maps to a known CTM after the y-flip conjugation: for a box at
// layout (0,0) on a 200-tall page, translate(10,20) → cm "1 0 0 1 10 -20"
// (right 10, down 20 in PDF space).
func TestRenderTransformTranslate(t *testing.T) {
	res := layoutFromJSON(t, transformDoc("translate(10, 20)"))
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	if !strings.Contains(content, "1 0 0 1 10 -20 cm") {
		t.Errorf("translate transform produced wrong CTM:\n%s", content)
	}
}

// A rotate must emit a non-identity CTM and stay valid.
func TestRenderTransformRotate(t *testing.T) {
	res := layoutFromJSON(t, transformDoc("rotate(90deg)"))
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	if !strings.Contains(content, " cm\n") {
		t.Errorf("rotate transform should emit a cm op:\n%s", content)
	}
	// 90° rotation has ~zero on the diagonal and ±1 off-diagonal; assert a "1" and
	// a "-1" appear in a cm (loose but catches an identity/no-op).
	if !strings.Contains(content, "-1 ") && !strings.Contains(content, " -1 ") {
		t.Errorf("rotate(90) CTM should contain a -1 term:\n%s", content)
	}
}

// A box with no transform must not emit a CTM (a plain background fill is just re/f).
func TestRenderNoTransformNoCM(t *testing.T) {
	doc := `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"VIEW","props":{"style":{"width":40,"height":40,"backgroundColor":"#ff0000"}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(inflateStreams(t, out.Bytes()), " cm\n") {
		t.Error("a box without a transform should not emit a cm op")
	}
}
