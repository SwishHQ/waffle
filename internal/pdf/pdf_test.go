package pdf

import (
	"bytes"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// buildHello builds the canonical single-page "Hello, World!" document with
// deterministic metadata.
func buildHello(t *testing.T) []byte {
	t.Helper()
	const a4W, a4H = 595.28, 841.89

	c := NewContent()
	c.BeginText().
		SetFont("Helvetica", 24).
		TextPosition(72, a4H-72).
		ShowText("Hello, World!").
		EndText()

	doc := New(Options{
		Title:        "Hello",
		Creator:      "feast",
		Producer:     "feast",
		PDFVersion:   "1.4",
		CreationDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	doc.AddPage(a4W, a4H, c)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	return buf.Bytes()
}

func TestHelloDeterministic(t *testing.T) {
	a := buildHello(t)
	b := buildHello(t)
	if !bytes.Equal(a, b) {
		t.Fatalf("output not deterministic: %d vs %d bytes", len(a), len(b))
	}
}

func TestHelloStructure(t *testing.T) {
	data := buildHello(t)
	if !bytes.HasPrefix(data, []byte("%PDF-1.4\n")) {
		t.Errorf("missing/incorrect PDF header")
	}
	for _, want := range []string{"/Type /Catalog", "/Type /Pages", "/Type /Page", "/BaseFont /Helvetica", "startxref", "%%EOF"} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("output missing %q", want)
		}
	}
	// startxref offset must point at the xref keyword.
	off := parseStartxref(t, data)
	if off < 0 || off >= len(data) || !bytes.HasPrefix(data[off:], []byte("xref")) {
		t.Fatalf("startxref offset %d does not point at 'xref'", off)
	}
}

func parseStartxref(t *testing.T, data []byte) int {
	t.Helper()
	idx := bytes.LastIndex(data, []byte("startxref"))
	if idx < 0 {
		t.Fatal("no startxref")
	}
	rest := data[idx+len("startxref"):]
	off := 0
	seen := false
	for _, ch := range rest {
		if ch == '\r' || ch == '\n' || ch == ' ' {
			if seen {
				break
			}
			continue
		}
		if ch < '0' || ch > '9' {
			break
		}
		off = off*10 + int(ch-'0')
		seen = true
	}
	if !seen {
		t.Fatal("could not parse startxref offset")
	}
	return off
}

// TestHelloGolden compares against a committed byte-exact golden. The content
// stream is Flate-compressed, so regenerate with FEAST_UPDATE=1 after a Go
// toolchain change if this fails for that reason.
func TestHelloGolden(t *testing.T) {
	golden := filepath.Join("testdata", "hello.golden.pdf")
	data := buildHello(t)

	if os.Getenv("FEAST_UPDATE") == "1" {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, data, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote golden %s (%d bytes)", golden, len(data))
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with FEAST_UPDATE=1 to create): %v", err)
	}
	if !bytes.Equal(want, data) {
		bad := filepath.Join(t.TempDir(), "hello.actual.pdf")
		_ = os.WriteFile(bad, data, 0o644)
		t.Fatalf("golden mismatch: got %d bytes, want %d bytes (actual written to %s)", len(data), len(want), bad)
	}
}

func TestHelloValidatesWithPDFCPU(t *testing.T) {
	bin := findPDFCPU()
	if bin == "" {
		t.Skip("pdfcpu not found on PATH or in GOPATH/bin; skipping validation")
	}
	data := buildHello(t)
	p := filepath.Join(t.TempDir(), "hello.pdf")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput()
	if err != nil {
		t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
	}
}

func findPDFCPU() string {
	if p, err := exec.LookPath("pdfcpu"); err == nil {
		return p
	}
	var candidates []string
	if gp := build.Default.GOPATH; gp != "" {
		candidates = append(candidates, filepath.Join(gp, "bin", "pdfcpu"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, "go", "bin", "pdfcpu"))
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}
