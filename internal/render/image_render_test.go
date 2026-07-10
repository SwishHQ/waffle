package render

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"testing"
)

func pngDataURI(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestRenderImage(t *testing.T) {
	uri := pngDataURI(t, 200, 100) // 2:1 aspect
	doc := `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[300,300]},"children":[
			{"type":"IMAGE","props":{"src":"` + uri + `","style":{"width":100}}}
		]}
	]}}`
	res := layoutFromJSON(t, doc)

	imgBox := res.Pages[0].Root.Children[0]
	if imgBox.Image == nil || imgBox.Image.Spec == nil {
		t.Fatalf("image not resolved: %+v", imgBox.Image)
	}
	// width 100 on a 2:1 image → height 50 (aspect preserved).
	if math.Abs(imgBox.Frame.W-100) > 1e-6 || math.Abs(imgBox.Frame.H-50) > 1e-6 {
		t.Errorf("image frame = %+v, want W=100 H=50", imgBox.Frame)
	}

	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	if !strings.Contains(content, " Do\n") {
		t.Errorf("expected an image draw (Do) op in content:\n%s", content)
	}
	if !bytes.Contains(out.Bytes(), []byte("/Subtype /Image")) {
		t.Errorf("expected an image XObject in the PDF")
	}
}
