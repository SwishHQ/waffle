package pdf

import (
	"bytes"
	"strings"
	"testing"
)

// A page whose content records a link annotation must emit a /Link annotation
// dict wired to a /URI action, referenced from the page's /Annots.
func TestLinkAnnotation(t *testing.T) {
	doc := New(Options{})
	c := NewContent()
	c.SetFont("Helvetica", 12).BeginText().TextPosition(10, 100).ShowText("hi").EndText()
	c.AddLink(10, 90, 110, 110, "https://example.com/path?q=1")
	doc.AddPage(200, 200, c)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	s := buf.String()
	for _, want := range []string{"/Annots", "/Subtype /Link", "/S /URI", "https://example.com/path?q=1", "/Rect [10 90 110 110]"} {
		if !strings.Contains(s, want) {
			t.Errorf("PDF missing %q\n%s", want, s)
		}
	}
}

// No links → no /Annots key (keep pages clean/deterministic).
func TestNoLinkNoAnnots(t *testing.T) {
	doc := New(Options{})
	c := NewContent()
	c.SetFont("Helvetica", 12).BeginText().TextPosition(10, 100).ShowText("hi").EndText()
	doc.AddPage(200, 200, c)
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "/Annots") {
		t.Error("page without links should not have /Annots")
	}
}
