package fontstore

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

func testFace(t *testing.T) *Face {
	t.Helper()
	s := New()
	if err := s.Register("Go", FaceSpec{Data: goregular.TTF, Weight: 400}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	f, ok := s.resolveRegistered("Go", 400, StyleNormal)
	if !ok {
		t.Fatal("resolveRegistered failed")
	}
	return f
}

func TestDescriptor(t *testing.T) {
	d := testFace(t).Descriptor()
	if d.Ascent <= 0 {
		t.Errorf("ascent = %v, want > 0", d.Ascent)
	}
	if d.Descent >= 0 {
		t.Errorf("descent = %v, want < 0", d.Descent)
	}
	if d.CapHeight <= 0 {
		t.Errorf("capHeight = %v, want > 0", d.CapHeight)
	}
	if d.Flags&flagNonsymbolic == 0 {
		t.Errorf("flags = %b, want the nonsymbolic bit set", d.Flags)
	}
	if d.Flags&flagFixedPitch != 0 {
		t.Errorf("Go is proportional; fixed-pitch flag should be clear (flags=%b)", d.Flags)
	}
	if d.StemV <= 0 {
		t.Errorf("stemV = %v, want > 0", d.StemV)
	}
	if d.BBox[2] <= d.BBox[0] || d.BBox[3] <= d.BBox[1] {
		t.Errorf("bbox = %v, want a non-degenerate rectangle", d.BBox)
	}
	// Metrics must be in 1000-unit text space (scaled from upem), not raw units.
	if d.Ascent > 2000 || d.Descent < -1000 {
		t.Errorf("metrics look unscaled: asc=%v desc=%v", d.Ascent, d.Descent)
	}
}

func TestWinAnsiWidths(t *testing.T) {
	w := testFace(t).WinAnsiWidths()
	if len(w) != 256 {
		t.Fatalf("widths len = %d, want 256", len(w))
	}
	if w['M'] <= 0 || w['i'] <= 0 || w[' '] <= 0 {
		t.Errorf("expected positive widths: M=%d i=%d space=%d", w['M'], w['i'], w[' '])
	}
	if w['M'] <= w['i'] {
		t.Errorf("expected M (%d) wider than i (%d) in a proportional font", w['M'], w['i'])
	}
}

func TestFontName(t *testing.T) {
	if n := testFace(t).FontName(); n != "Go" {
		t.Errorf("FontName = %q, want Go", n)
	}
	if n := (&Face{Family: "My Font", Weight: 700}).FontName(); n != "MyFont-Bold" {
		t.Errorf("bold FontName = %q, want MyFont-Bold", n)
	}
	if n := (&Face{Family: "Serif", Style: StyleItalic}).FontName(); n != "Serif-Italic" {
		t.Errorf("italic FontName = %q, want Serif-Italic", n)
	}
}
