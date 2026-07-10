package fontstore

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

// EmbeddedFont returns the same pointer on repeated calls so the PDF writer can
// dedupe the font program to a single FontFile2 stream. Previously each call
// allocated a fresh value, re-embedding the whole TTF per text box.
func TestEmbeddedFontMemoized(t *testing.T) {
	s := New()
	if err := s.Register("Go", FaceSpec{Data: goregular.TTF, Weight: 400}); err != nil {
		t.Fatal(err)
	}
	face, ok := s.ResolveFace("Go", 400, StyleNormal)
	if !ok {
		t.Fatal("face should resolve")
	}
	a := face.EmbeddedFont()
	b := face.EmbeddedFont()
	if a != b {
		t.Error("EmbeddedFont should return the same cached pointer across calls")
	}
}
