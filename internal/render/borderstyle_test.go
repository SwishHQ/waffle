package render

import (
	"bytes"
	"strings"
	"testing"
)

// A dashed border strokes a dashed line (d operator) rather than filling a strip.
func TestRenderDashedBorder(t *testing.T) {
	doc := `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"VIEW","props":{"style":{"width":60,"height":60,"borderWidth":3,"borderStyle":"dashed","borderColor":"#000000"}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	if !strings.Contains(content, "] 0 d\n") {
		t.Errorf("dashed border should emit a dash pattern (d):\n%s", content)
	}
	if !strings.Contains(content, "[9 6]") {
		t.Errorf("dashed pattern for width 3 should be [9 6]:\n%s", content)
	}
	if !strings.Contains(content, "S\n") {
		t.Errorf("dashed border should stroke (S):\n%s", content)
	}
}

// A dotted border uses round caps (1 J) and a zero-length dash so each on-segment
// renders as a dot.
func TestRenderDottedBorder(t *testing.T) {
	doc := `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"VIEW","props":{"style":{"width":60,"height":60,"borderWidth":2,"borderStyle":"dotted","borderColor":"#0000ff"}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	if !strings.Contains(content, "1 J\n") {
		t.Errorf("dotted border should set round line caps (1 J):\n%s", content)
	}
	if !strings.Contains(content, "[0 4]") {
		t.Errorf("dotted pattern for width 2 should be [0 4]:\n%s", content)
	}
}

// A solid border (default) still fills a strip and emits no dash.
func TestRenderSolidBorderNoDash(t *testing.T) {
	doc := `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"VIEW","props":{"style":{"width":60,"height":60,"borderWidth":3,"borderColor":"#000000"}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	if strings.Contains(content, "] 0 d\n") {
		t.Errorf("solid border must not emit a dash pattern:\n%s", content)
	}
	if !strings.Contains(content, "f\n") {
		t.Errorf("solid border should fill (f):\n%s", content)
	}
}

// A rounded, dashed, uniform border strokes a rounded path with a dash pattern.
func TestRenderRoundedDashedBorder(t *testing.T) {
	doc := `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"VIEW","props":{"style":{"width":60,"height":60,"borderWidth":2,"borderStyle":"dashed","borderColor":"#000000","borderRadius":10}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	if !strings.Contains(content, "] 0 d\n") {
		t.Errorf("rounded dashed border should emit a dash pattern:\n%s", content)
	}
	if !strings.Contains(content, " c\n") {
		t.Errorf("rounded border should use bezier curves (c):\n%s", content)
	}
}
