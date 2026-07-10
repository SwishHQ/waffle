package pdf

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoteAnnotation(t *testing.T) {
	doc := New(Options{})
	c := NewContent()
	c.Save().Rect(0, 0, 10, 10).Fill().Restore()
	c.AddNote(30, 180, "A reviewer note")
	doc.AddPage(200, 200, c)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	s := buf.String()
	for _, want := range []string{"/Subtype /Text", "(A reviewer note)", "/Name /Note", "/Open false"} {
		if !strings.Contains(s, want) {
			t.Errorf("PDF missing %q", want)
		}
	}
	if bin, _ := exec.LookPath("pdfcpu"); bin != "" {
		p := filepath.Join(t.TempDir(), "note.pdf")
		os.WriteFile(p, buf.Bytes(), 0o644)
		if out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput(); err != nil {
			t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
		}
	}
}
