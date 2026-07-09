package pdf

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

func uniformWidths(w int) []int {
	out := make([]int, 256)
	for i := range out {
		out[i] = w
	}
	return out
}

func TestEmbedTrueTypeFont(t *testing.T) {
	ef := &EmbeddedFont{
		Name:    "GoTestFont",
		Program: goregular.TTF,
		Descriptor: FontDescriptor{
			Ascent: 900, Descent: -200, CapHeight: 700,
			BBox: [4]float64{0, -200, 1000, 900}, Flags: 32, StemV: 80,
		},
		Widths: uniformWidths(500),
	}

	doc := New(Options{})
	c := NewContent()
	c.BeginText().SetEmbeddedFont(ef, 24).TextPosition(50, 120)
	c.ShowText("Hello, custom font!").EndText()
	doc.AddPage(400, 200, c)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	s := buf.String()
	for _, want := range []string{
		"/Subtype /TrueType",
		"/FontDescriptor",
		"/FontFile2",
		"/Length1",
		"/Encoding /WinAnsiEncoding",
		"/FirstChar 32",
		"/LastChar 255",
		"GoTestFont",
		"/TT1", // resource name used in the content stream + resource dict
	} {
		if !strings.Contains(s, want) {
			t.Errorf("PDF missing %q", want)
		}
	}

	// The embedded program must be present (compressed), and Length1 must be the
	// uncompressed program length.
	if !strings.Contains(s, "/Length1 "+itoa(len(goregular.TTF))) {
		t.Errorf("Length1 should equal the raw TTF length %d", len(goregular.TTF))
	}

	if bin := findPDFCPU(); bin != "" {
		p := filepath.Join(t.TempDir(), "embed.pdf")
		if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput(); err != nil {
			t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
		}
	} else {
		t.Log("pdfcpu not found; skipping strict validation")
	}
}

// A second page reusing the same *EmbeddedFont must share one font object.
func TestEmbeddedFontDedup(t *testing.T) {
	ef := &EmbeddedFont{Name: "GoDedup", Program: goregular.TTF,
		Descriptor: FontDescriptor{Ascent: 900, Descent: -200, CapHeight: 700, BBox: [4]float64{0, -200, 1000, 900}, Flags: 32, StemV: 80},
		Widths:     uniformWidths(500)}
	doc := New(Options{})
	for i := 0; i < 2; i++ {
		c := NewContent()
		c.BeginText().SetEmbeddedFont(ef, 12).TextPosition(10, 10).ShowText("x").EndText()
		doc.AddPage(100, 100, c)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(buf.String(), "/FontFile2"); n != 1 {
		t.Errorf("FontFile2 appears %d times; the shared font should embed once", n)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}
