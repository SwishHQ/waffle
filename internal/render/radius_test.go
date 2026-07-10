package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestClampRadii(t *testing.T) {
	// Overflowing radii scale down uniformly: 100/(60+60) = 0.833 → 50.
	tl, tr, br, bl := clampRadii(100, 100, 60, 60, 60, 60)
	for _, r := range []float64{tl, tr, br, bl} {
		if !approx(r, 50) {
			t.Errorf("clamp square: got %g, want 50", r)
		}
	}
	// A short box clamps by height: 40/(30+30) = 0.667 → 20.
	tl, _, _, bl = clampRadii(100, 40, 30, 30, 30, 30)
	if !approx(tl, 20) || !approx(bl, 20) {
		t.Errorf("clamp short: tl=%g bl=%g, want 20", tl, bl)
	}
	// Within bounds: unchanged.
	tl, tr, br, bl = clampRadii(100, 100, 10, 10, 10, 10)
	if !approx(tl, 10) || !approx(tr, 10) || !approx(br, 10) || !approx(bl, 10) {
		t.Errorf("clamp within bounds changed radii: %g %g %g %g", tl, tr, br, bl)
	}
	// Negatives floored to 0.
	if tl, _, _, _ = clampRadii(100, 100, -5, 0, 0, 0); tl != 0 {
		t.Errorf("negative radius not floored: %g", tl)
	}
}

func radiusDoc(radius string) string {
	return `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"VIEW","props":{"style":{"width":100,"height":60,"backgroundColor":"#3366cc","borderRadius":` + radius + `}}}
		]}
	]}}`
}

func TestRenderRoundedBackground(t *testing.T) {
	res := layoutFromJSON(t, radiusDoc("12"))
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	if !strings.Contains(content, " c\n") {
		t.Errorf("rounded background should emit bezier curves (c):\n%s", content)
	}
}

func TestRenderSharpBackgroundNoCurves(t *testing.T) {
	res := layoutFromJSON(t, radiusDoc("0"))
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	if strings.Contains(content, " c\n") {
		t.Errorf("radius 0 should not emit curves:\n%s", content)
	}
	if !strings.Contains(content, "re\n") {
		t.Errorf("radius 0 background should use a rect (re):\n%s", content)
	}
}

// A percent radius resolves against the box's min dimension (so 50% on a square
// yields a circle) — verified by the presence of curves.
func TestRenderPercentRadius(t *testing.T) {
	doc := `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"VIEW","props":{"style":{"width":80,"height":80,"backgroundColor":"#e63946","borderRadius":"50%"}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inflateStreams(t, out.Bytes()), " c\n") {
		t.Error("percent radius should round the corners (emit curves)")
	}
}

// A rounded image clips its draw to the rounded path.
func TestRenderRoundedImageClips(t *testing.T) {
	uri := pngDataURI(t, 40, 40)
	doc := `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"IMAGE","props":{"src":"` + uri + `","style":{"width":40,"height":40,"borderRadius":20}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	if !strings.Contains(content, " c\n") {
		t.Error("rounded image should clip with a rounded (curved) path")
	}
	if !strings.Contains(content, "W\nn\n") {
		t.Error("rounded image should establish a clip (W n)")
	}
	if !strings.Contains(content, " Do\n") {
		t.Error("image should still be drawn (Do)")
	}
}

// A rounded + uniform border strokes a single rounded path.
func TestRenderRoundedUniformBorder(t *testing.T) {
	doc := `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"VIEW","props":{"style":{"width":100,"height":60,"borderRadius":10,"border":"2pt solid #1d3557"}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	if !strings.Contains(content, " c\n") || !strings.Contains(content, "\nS\n") {
		t.Errorf("rounded border should stroke a curved path (c + S):\n%s", content)
	}
}
