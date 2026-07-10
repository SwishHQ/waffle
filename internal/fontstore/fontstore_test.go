package fontstore

import (
	"math"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/SwishHQ/waffle/internal/pdf/afm"
)

func TestRegisterAndMeasure(t *testing.T) {
	s := New()
	if err := s.Register("Go", FaceSpec{Data: goregular.TTF, Weight: 400}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	f, ok := s.Resolve("Go", 400, StyleNormal)
	if !ok {
		t.Fatal("Resolve failed for registered font")
	}
	w := f.StringWidth("Hello", 12)
	if w <= 0 {
		t.Errorf("StringWidth = %v, want > 0", w)
	}
	// Wider text measures wider.
	if f.StringWidth("Hello World", 12) <= w {
		t.Errorf("longer string should measure wider")
	}
	// Larger size scales linearly.
	if got, want := f.StringWidth("Hello", 24), w*2; math.Abs(got-want) > 1e-6 {
		t.Errorf("size scaling: 24pt=%v, want 2*12pt=%v", got, want)
	}
}

func TestNearestWeight(t *testing.T) {
	s := New()
	_ = s.Register("Go",
		FaceSpec{Data: goregular.TTF, Weight: 400},
		FaceSpec{Data: goregular.TTF, Weight: 700},
	)
	cases := []struct {
		req  int
		want int
	}{
		{400, 400},
		{500, 400}, // tie 100 each → lighter wins
		{600, 700},
		{700, 700},
		{900, 700},
	}
	for _, c := range cases {
		f, ok := s.Resolve("Go", c.req, StyleNormal)
		if !ok {
			t.Fatalf("Resolve(%d) failed", c.req)
		}
		if got := f.(*Face).Weight; got != c.want {
			t.Errorf("Resolve weight %d → face weight %d, want %d", c.req, got, c.want)
		}
	}
}

func TestStyleMatching(t *testing.T) {
	s := New()
	_ = s.Register("Go",
		FaceSpec{Data: goregular.TTF, Weight: 400, Style: StyleNormal},
		FaceSpec{Data: goregular.TTF, Weight: 400, Style: StyleItalic},
	)
	f, _ := s.Resolve("Go", 400, StyleItalic)
	if f.(*Face).Style != StyleItalic {
		t.Errorf("expected italic face")
	}
	f, _ = s.Resolve("Go", 400, StyleNormal)
	if f.(*Face).Style != StyleNormal {
		t.Errorf("expected normal face")
	}
}

func TestStandardFallback(t *testing.T) {
	s := New()
	// Helvetica bold is not registered → falls back to the standard font.
	f, ok := s.Resolve("Helvetica", 700, StyleNormal)
	if !ok {
		t.Fatal("standard-font fallback failed")
	}
	m, _ := afm.Load("Helvetica-Bold")
	if got, want := f.StringWidth("Hi", 12), m.StringWidth("Hi", 12); math.Abs(got-want) > 1e-9 {
		t.Errorf("fallback width %v, want Helvetica-Bold %v", got, want)
	}

	// Style/weight selection maps to the right base font.
	f2, _ := s.Resolve("times", 400, StyleItalic)
	mi, _ := afm.Load("Times-Italic")
	if math.Abs(f2.StringWidth("Hi", 12)-mi.StringWidth("Hi", 12)) > 1e-9 {
		t.Errorf("times italic did not map to Times-Italic")
	}
}

func TestUnknownFamily(t *testing.T) {
	s := New()
	if _, ok := s.Resolve("Nonexistent", 400, StyleNormal); ok {
		t.Errorf("unknown family should not resolve")
	}
}

func TestParseError(t *testing.T) {
	s := New()
	if err := s.Register("Bad", FaceSpec{Data: []byte("not a font")}); err == nil {
		t.Errorf("expected parse error for garbage data")
	}
}
