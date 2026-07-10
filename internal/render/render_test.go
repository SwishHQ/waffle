package render

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/SwishHQ/waffle/internal/contract"
	"github.com/SwishHQ/waffle/internal/layout"
	"github.com/SwishHQ/waffle/internal/tree"
)

func layoutFromJSON(t *testing.T, doc string) *layout.Result {
	t.Helper()
	ct, err := contract.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("contract.Parse: %v", err)
	}
	tr, err := tree.Build(ct)
	if err != nil {
		t.Fatalf("tree.Build: %v", err)
	}
	res, err := layout.Layout(tr, layout.Options{})
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	return res
}

// inflateStreams returns the concatenated inflated content of every
// FlateDecode stream in a PDF.
func inflateStreams(t *testing.T, data []byte) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)
	var out strings.Builder
	for _, m := range re.FindAllSubmatch(data, -1) {
		zr, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue
		}
		dec, err := io.ReadAll(zr)
		zr.Close()
		if err == nil {
			out.Write(dec)
		}
	}
	return out.String()
}

func TestRenderYFlip(t *testing.T) {
	// A 50x30 red box at the page's top-left corner. Page 200x200.
	res := layoutFromJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"VIEW","props":{"style":{"width":50,"height":30,"backgroundColor":"#ff0000"}}}
		]}
	]}}`)

	var buf bytes.Buffer
	if err := Render(res, &buf, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, buf.Bytes())

	// Top-left in layout (0,0,50,30) → PDF lower-left corner at y = 200-0-30 = 170.
	if !strings.Contains(content, "0 170 50 30 re") {
		t.Errorf("expected Y-flipped rect '0 170 50 30 re' in content:\n%s", content)
	}
	// Red fill color.
	if !strings.Contains(content, "1 0 0 rg") {
		t.Errorf("expected red fill '1 0 0 rg' in content:\n%s", content)
	}
}

func TestRenderBackgroundAndBorders(t *testing.T) {
	res := layoutFromJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[
			{"type":"VIEW","props":{"style":{"width":40,"height":40,"backgroundColor":"blue","border":"2pt solid black"}}}
		]}
	]}}`)
	var buf bytes.Buffer
	if err := Render(res, &buf, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, buf.Bytes())
	// Blue background.
	if !strings.Contains(content, "0 0 1 rg") {
		t.Errorf("missing blue background in:\n%s", content)
	}
	// Four black border strips (fill color 0 0 0). Count 're' ops: 1 bg + 4 borders = 5.
	if n := strings.Count(content, " re\n"); n < 5 {
		t.Errorf("expected >=5 rectangles (bg + 4 borders), got %d:\n%s", n, content)
	}
}

func TestRenderEmptyPage(t *testing.T) {
	res := layoutFromJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[100,100]},"children":[]}
	]}}`)
	var buf bytes.Buffer
	if err := Render(res, &buf, Options{}); err != nil {
		t.Fatalf("empty page should render: %v", err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Errorf("not a PDF")
	}
}

func TestRenderText(t *testing.T) {
	res := layoutFromJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[300,100]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":24,"color":"#0000ff"}},"children":[
				{"type":"TEXT_INSTANCE","value":"Hello waffle"}
			]}
		]}
	]}}`)
	var buf bytes.Buffer
	if err := Render(res, &buf, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, buf.Bytes())
	if !strings.Contains(content, "(Hello waffle) Tj") {
		t.Errorf("missing text show op in:\n%s", content)
	}
	if !strings.Contains(content, "Tf") {
		t.Errorf("missing font selection (Tf)")
	}
	if !strings.Contains(content, "0 0 1 rg") {
		t.Errorf("missing blue text color")
	}
}

func TestRenderTextAlignCenter(t *testing.T) {
	// Short centered text in a wide box: the line's x origin must be offset right.
	res := layoutFromJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[400,60]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":12,"textAlign":"center","width":"100%"}},"children":[
				{"type":"TEXT_INSTANCE","value":"Centered"}
			]}
		]}
	]}}`)
	var buf bytes.Buffer
	Render(res, &buf, Options{})
	content := inflateStreams(t, buf.Bytes())
	// Find the text matrix "1 0 0 1 X Y Tm"; X must be well right of 0.
	m := regexp.MustCompile(`1 0 0 1 ([0-9.]+) [0-9.]+ Tm`).FindStringSubmatch(content)
	if m == nil {
		t.Fatalf("no text matrix found:\n%s", content)
	}
	if m[1] == "0" {
		t.Errorf("centered text should be offset right, got x=%s", m[1])
	}
}

func TestRenderJustify(t *testing.T) {
	res := layoutFromJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[140,300]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":12,"textAlign":"justify"}},"children":[
				{"type":"TEXT_INSTANCE","value":"The quick brown fox jumps over the lazy dog several times over"}
			]}
		]}
	]}}`)
	var buf bytes.Buffer
	Render(res, &buf, Options{})
	content := inflateStreams(t, buf.Bytes())
	// At least one line must have a positive word spacing (Tw).
	pos := false
	for _, m := range regexp.MustCompile(`([0-9.]+) Tw`).FindAllStringSubmatch(content, -1) {
		if m[1] != "0" {
			pos = true
		}
	}
	if !pos {
		t.Errorf("justify should produce a positive Tw word spacing:\n%s", content)
	}
}

func TestRenderSVG(t *testing.T) {
	res := layoutFromJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"SVG","props":{"style":{"width":100,"height":100},"viewBox":"0 0 100 100"},"children":[
				{"type":"RECT","props":{"x":0,"y":0,"width":100,"height":100,"fill":"#457b9d"}},
				{"type":"CIRCLE","props":{"cx":50,"cy":50,"r":30,"fill":"#e63946","stroke":"#1d3557","strokeWidth":4}}
			]}
		]}
	]}}`)
	svgBox := res.Pages[0].Root.Children[0]
	if svgBox.SVG == nil {
		t.Fatal("svg subtree not attached to box")
	}
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	if !strings.Contains(content, " cm\n") {
		t.Errorf("expected a viewBox CTM (cm) op")
	}
	if !strings.Contains(content, " c\n") {
		t.Errorf("expected cubic curves from the circle")
	}
	if !strings.Contains(content, "\nB\n") {
		t.Errorf("expected fill+stroke (B) for the circle")
	}
}

func TestRenderCanvas(t *testing.T) {
	res := layoutFromJSON(t, `{"version":"waffle-tree/v1","document":{"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"CANVAS","props":{"style":{"width":100,"height":100},"paint":[
				{"op":"fillColor","args":["#e63946"]},
				{"op":"rect","args":[10,10,40,40]},
				{"op":"fill","args":[]},
				{"op":"strokeColor","args":["#1d3557"]},
				{"op":"lineWidth","args":[2]},
				{"op":"moveTo","args":[0,0]},
				{"op":"lineTo","args":[100,100]},
				{"op":"stroke","args":[]}
			]}}
		]}
	]}}`)
	cbox := res.Pages[0].Root.Children[0]
	if len(cbox.Canvas) == 0 {
		t.Fatal("canvas ops not attached")
	}
	var out bytes.Buffer
	if err := Render(res, &out, Options{}); err != nil {
		t.Fatal(err)
	}
	content := inflateStreams(t, out.Bytes())
	for _, want := range []string{"re\n", " rg\n", "\nS\n"} {
		if !strings.Contains(content, want) {
			t.Errorf("canvas content missing %q:\n%s", want, content)
		}
	}
}
