package pdf

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func validateShadingPDF(t *testing.T, b []byte) {
	t.Helper()
	bin := findPDFCPU()
	if bin == "" {
		t.Log("pdfcpu not found; skipping strict validation")
		return
	}
	p := filepath.Join(t.TempDir(), "shade.pdf")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput(); err != nil {
		t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
	}
}

// A two-stop linear gradient clipped to a rect: axial shading (Type 2) with a
// single Type 2 exponential color function.
func TestShadingLinear(t *testing.T) {
	doc := New(Options{})
	c := NewContent()
	sh := &Shading{Coords: []float64{0, 0, 100, 0}, Stops: []ShadingStop{{0, 1, 0, 0}, {1, 0, 0, 1}}}
	c.Save().Rect(0, 0, 100, 100).Clip().EndPath().Shade(sh).Restore()
	doc.AddPage(100, 100, c)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	s := buf.String()
	for _, want := range []string{"/Shading", "/ShadingType 2", "/ColorSpace /DeviceRGB", "/Coords", "/FunctionType 2", "/Extend"} {
		if !strings.Contains(s, want) {
			t.Errorf("PDF missing %q", want)
		}
	}
	if strings.Contains(s, "/FunctionType 3") {
		t.Error("two stops should use a single Type 2 function, not a Type 3 stitching function")
	}
	validateShadingPDF(t, buf.Bytes())
}

// A three-stop radial gradient: radial shading (Type 3) with a Type 3 stitching
// function over two Type 2 segments.
func TestShadingRadialMultiStop(t *testing.T) {
	doc := New(Options{})
	c := NewContent()
	sh := &Shading{
		Radial: true,
		Coords: []float64{50, 50, 0, 50, 50, 50},
		Stops:  []ShadingStop{{0, 1, 1, 1}, {0.5, 1, 0, 0}, {1, 0, 0, 0}},
	}
	c.Save().Rect(0, 0, 100, 100).Clip().EndPath().Shade(sh).Restore()
	doc.AddPage(100, 100, c)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	s := buf.String()
	for _, want := range []string{"/ShadingType 3", "/FunctionType 3", "/Bounds", "/Encode"} {
		if !strings.Contains(s, want) {
			t.Errorf("PDF missing %q", want)
		}
	}
	validateShadingPDF(t, buf.Bytes())
}

// normalizeStops guarantees a usable, sorted, end-pinned stop list.
func TestNormalizeStops(t *testing.T) {
	if got := normalizeStops(nil); len(got) != 2 {
		t.Errorf("empty stops -> %d, want 2 (transparent black span)", len(got))
	}
	one := normalizeStops([]ShadingStop{{0.5, 1, 0, 0}})
	if len(one) != 2 || one[0].R != 1 || one[1].R != 1 {
		t.Errorf("single stop should expand to a solid two-stop span: %+v", one)
	}
	// Out-of-order + unclamped ends get sorted and pinned to [0,1].
	s := normalizeStops([]ShadingStop{{0.9, 0, 0, 1}, {0.1, 1, 0, 0}})
	if s[0].Offset != 0 || s[len(s)-1].Offset != 1 {
		t.Errorf("ends not pinned to 0/1: %+v", s)
	}
	if s[0].R != 1 {
		t.Errorf("stops not sorted by offset: %+v", s)
	}
}
