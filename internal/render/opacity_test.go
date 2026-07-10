package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderOpacity(t *testing.T) {
	doc := `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"VIEW","props":{"style":{"width":50,"height":50,"backgroundColor":"#ff0000","opacity":0.5}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inflateStreams(t, out.Bytes()), "gs\n") {
		t.Error("opacity should emit an ExtGState (gs) op")
	}
	raw := out.String()
	if !strings.Contains(raw, "/ExtGState") {
		t.Error("page should declare an /ExtGState resource")
	}
	if !strings.Contains(raw, "/ca 0.5") || !strings.Contains(raw, "/CA 0.5") {
		t.Errorf("ExtGState should set /ca and /CA to 0.5:\n%s", raw)
	}
}

// Nested opacity multiplies (CSS semantics): child 0.4 inside parent 0.5 → 0.2.
func TestRenderNestedOpacityMultiplies(t *testing.T) {
	doc := `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"VIEW","props":{"style":{"opacity":0.5}},"children":[
				{"type":"VIEW","props":{"style":{"width":30,"height":30,"backgroundColor":"#0000ff","opacity":0.4}}}
			]}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	raw := out.String()
	if !strings.Contains(raw, "/ca 0.5") {
		t.Errorf("outer alpha 0.5 missing:\n%s", raw)
	}
	if !strings.Contains(raw, "/ca 0.2") {
		t.Errorf("nested alpha should be 0.5*0.4=0.2:\n%s", raw)
	}
	// Two distinct ExtGState resources and two gs ops.
	content := inflateStreams(t, out.Bytes())
	if n := strings.Count(content, "gs\n"); n != 2 {
		t.Errorf("expected 2 gs ops (nested), got %d", n)
	}
}

// A fully opaque box (opacity 1 or absent) must not emit any ExtGState.
func TestRenderNoOpacityNoExtGState(t *testing.T) {
	doc := `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"VIEW","props":{"style":{"width":50,"height":50,"backgroundColor":"#ff0000"}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "/ExtGState") {
		t.Error("opaque page should not declare /ExtGState")
	}
}
