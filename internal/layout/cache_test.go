package layout

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/SwishHQ/waffle/internal/contract"
	"github.com/SwishHQ/waffle/internal/pdf"
	"github.com/SwishHQ/waffle/internal/tree"
)

// layoutWith parses, builds, and lays out a document with the given cache —
// each call simulates one render of the same template.
func layoutWith(t *testing.T, doc string, cache *Cache) *Result {
	t.Helper()
	ct, err := contract.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("contract.Parse: %v", err)
	}
	tr, err := tree.Build(ct)
	if err != nil {
		t.Fatalf("tree.Build: %v", err)
	}
	res, err := Layout(tr, Options{Cache: cache})
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	return res
}

func findBox(b *Box, pred func(*Box) bool) *Box {
	if pred(b) {
		return b
	}
	for _, c := range b.Children {
		if got := findBox(c, pred); got != nil {
			return got
		}
	}
	return nil
}

func firstImageSpec(t *testing.T, res *Result) *pdf.ImageSpec {
	t.Helper()
	box := findBox(res.Pages[0].Root, func(b *Box) bool { return b.Image != nil })
	if box == nil {
		t.Fatal("no image box found")
	}
	return box.Image.Spec
}

func firstEmbeddedFont(t *testing.T, res *Result) *pdf.EmbeddedFont {
	t.Helper()
	box := findBox(res.Pages[0].Root, func(b *Box) bool { return b.Text != nil })
	if box == nil {
		t.Fatal("no text box found")
	}
	return box.Text.EmbeddedFont
}

func pngDataURI(t *testing.T) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

// A shared Cache hands every render the same decoded *pdf.ImageSpec, so the
// image is decoded (and its pixels compressed) once per template, not per
// render. Without a cache each render decodes its own copy.
func TestCacheReusesImageSpec(t *testing.T) {
	doc := fmt.Sprintf(`{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"IMAGE","props":{"src":%q,"style":{"width":10}}}
		]}
	]}}`, pngDataURI(t))

	c := NewCache()
	a := firstImageSpec(t, layoutWith(t, doc, c))
	b := firstImageSpec(t, layoutWith(t, doc, c))
	if a != b {
		t.Error("renders sharing a Cache should reuse the same decoded ImageSpec")
	}

	u := firstImageSpec(t, layoutWith(t, doc, nil))
	v := firstImageSpec(t, layoutWith(t, doc, nil))
	if u == v {
		t.Error("renders without a Cache must decode independent ImageSpecs")
	}
}

// A shared Cache reuses the parsed font store, observable as the same
// *pdf.EmbeddedFont resolving for the same face — which in turn lets the PDF
// writer reuse the compressed font program across documents.
func TestCacheReusesFontStore(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString(goregular.TTF)
	doc := fmt.Sprintf(`{"version":"waffle-tree/v1","document":{
		"fonts":[{"family":"Go","fonts":[{"src":{"$inline":%q},"fontWeight":400}]}],
		"children":[
			{"type":"PAGE","props":{"size":[300,100]},"children":[
				{"type":"TEXT","props":{"style":{"fontFamily":"Go","fontSize":12}},"children":[
					{"type":"TEXT_INSTANCE","value":"Hello"}
				]}
			]}
		]}}`, b64)

	c := NewCache()
	a := firstEmbeddedFont(t, layoutWith(t, doc, c))
	b := firstEmbeddedFont(t, layoutWith(t, doc, c))
	if a == nil {
		t.Fatal("registered custom font should resolve to an embedded font")
	}
	if a != b {
		t.Error("renders sharing a Cache should reuse the parsed font (same *EmbeddedFont)")
	}

	if u := firstEmbeddedFont(t, layoutWith(t, doc, nil)); u == a {
		t.Error("a render without the Cache must build a fresh font store")
	}
}
