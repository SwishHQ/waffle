package render

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestFitImage(t *testing.T) {
	cases := []struct {
		name                           string
		fit                            string
		bw, bh, iw, ih                 float64
		wantDw, wantDh, wantOx, wantOy float64
		wantClip                       bool
	}{
		{"fill", "fill", 200, 100, 100, 100, 200, 100, 0, 0, false},
		{"empty means fill", "", 200, 100, 100, 100, 200, 100, 0, 0, false},
		{"unknown means fill", "weird", 200, 100, 100, 100, 200, 100, 0, 0, false},
		{"contain in wide box", "contain", 200, 100, 100, 100, 100, 100, 50, 0, false},
		{"contain in tall box", "contain", 100, 200, 100, 100, 100, 100, 0, 50, false},
		{"cover in wide box clips", "cover", 200, 100, 100, 100, 200, 200, 0, -50, true},
		{"none fits", "none", 200, 100, 100, 100, 100, 100, 50, 0, false},
		{"none overflows clips", "none", 50, 50, 100, 100, 100, 100, -25, -25, true},
		{"scale-down shrinks like contain", "scale-down", 50, 50, 100, 100, 50, 50, 0, 0, false},
		{"scale-down keeps intrinsic when it fits", "scale-down", 200, 200, 100, 100, 100, 100, 50, 50, false},
	}
	for _, c := range cases {
		dw, dh, ox, oy, clip := fitImage(c.fit, c.bw, c.bh, c.iw, c.ih)
		if !approx(dw, c.wantDw) || !approx(dh, c.wantDh) || !approx(ox, c.wantOx) || !approx(oy, c.wantOy) || clip != c.wantClip {
			t.Errorf("%s: got dw=%g dh=%g ox=%g oy=%g clip=%v; want dw=%g dh=%g ox=%g oy=%g clip=%v",
				c.name, dw, dh, ox, oy, clip, c.wantDw, c.wantDh, c.wantOx, c.wantOy, c.wantClip)
		}
	}
}

// objectFit changes how a square image fills a 200×100 box (both dims explicit,
// so the box keeps its non-square shape) — verified in the content stream.
func imageObjectFitDoc(fit string) string {
	// A tiny 100×100 square image; box 200×100.
	return `{"version":"feast-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[300,300]},"children":[
			{"type":"IMAGE","props":{"src":"__URI__","style":{"width":200,"height":100,"objectFit":"` + fit + `"}}}
		]}
	]}}`
}

func TestRenderImageObjectFitContain(t *testing.T) {
	doc := strings.Replace(imageObjectFitDoc("contain"), "__URI__", pngDataURI(t, 100, 100), 1)
	res := layoutFromJSON(t, doc)
	box := res.Pages[0].Root.Children[0]
	if !approx(box.Frame.W, 200) || !approx(box.Frame.H, 100) {
		t.Fatalf("box frame = %+v, want 200×100 (both dims explicit)", box.Frame)
	}
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	// contain: square scaled to 100×100 (fits the 100-tall box), centered.
	if !strings.Contains(content, "100 0 0 100") {
		t.Errorf("contain: want a 100×100 image matrix, content:\n%s", content)
	}
	if strings.Contains(content, "W\nn\n") {
		t.Errorf("contain: should not clip (image fits), content:\n%s", content)
	}
}

func TestRenderImageObjectFitCoverClips(t *testing.T) {
	doc := strings.Replace(imageObjectFitDoc("cover"), "__URI__", pngDataURI(t, 100, 100), 1)
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	// cover: square scaled to 200×200 to fill the 200-wide box, then clipped.
	if !strings.Contains(content, "200 0 0 200") {
		t.Errorf("cover: want a 200×200 image matrix, content:\n%s", content)
	}
	if !strings.Contains(content, "W\nn\n") {
		t.Errorf("cover: want a clip (W n) around the overflow, content:\n%s", content)
	}
}

func TestRenderImageFillDefault(t *testing.T) {
	doc := strings.Replace(imageObjectFitDoc("fill"), "__URI__", pngDataURI(t, 100, 100), 1)
	res := layoutFromJSON(t, doc)
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	// fill: stretched to the full 200×100 box.
	if !strings.Contains(content, "200 0 0 100") {
		t.Errorf("fill: want a 200×100 image matrix, content:\n%s", content)
	}
}
