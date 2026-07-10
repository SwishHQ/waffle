package pdf

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentOutline(t *testing.T) {
	doc := New(Options{})
	c1 := NewContent()
	c1.Save().Rect(0, 0, 10, 10).Fill().Restore()
	doc.AddPage(200, 200, c1)
	c2 := NewContent()
	c2.Save().Rect(0, 0, 10, 10).Fill().Restore()
	doc.AddPage(200, 200, c2)

	doc.SetOutline([]*Outline{
		{Title: "Chapter 1", Page: 0, Top: 190, Children: []*Outline{
			{Title: "Section 1.1", Page: 0, Top: 150},
		}},
		{Title: "Chapter 2", Page: 1, Top: 190},
	})

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	s := buf.String()
	for _, want := range []string{"/Type /Outlines", "/Outlines ", "(Chapter 1)", "(Section 1.1)", "(Chapter 2)", "/Dest", "/XYZ", "/First", "/Last", "/PageMode /UseOutlines"} {
		if !strings.Contains(s, want) {
			t.Errorf("PDF missing %q", want)
		}
	}
	// Root /Count should be the total descendant count (3 items, all open).
	if !strings.Contains(s, "/Count 3") {
		t.Errorf("outline root should report /Count 3:\n%s", s)
	}

	if bin := findPDFCPU(); bin != "" {
		p := filepath.Join(t.TempDir(), "outline.pdf")
		if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput(); err != nil {
			t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
		}
	}
}

// With no bookmarks, no /Outlines dictionary is written.
func TestDocumentNoOutline(t *testing.T) {
	doc := New(Options{})
	c := NewContent()
	c.Save().Rect(0, 0, 10, 10).Fill().Restore()
	doc.AddPage(100, 100, c)
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "/Outlines") {
		t.Error("a document without bookmarks should not declare /Outlines")
	}
}
