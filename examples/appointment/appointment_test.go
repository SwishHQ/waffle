package main

import (
	"bytes"
	"compress/zlib"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/swish/feast"
)

// renderAppointment renders the embedded appointment.jsx with sample props and
// returns the PDF bytes. It changes into the package directory first so the
// JSX's relative asset paths (assets/…) resolve.
func renderAppointment(t *testing.T) ([]byte, *feast.RenderInfo) {
	t.Helper()
	if err := os.Chdir(packageDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	var buf bytes.Buffer
	info, err := feast.RenderReact(context.Background(), appointmentJSX, sampleProps(), &buf)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.Bytes(), info
}

// TestGenerateAppointmentPDF renders the letter, writes it to
// examples/appointment/appointment.pdf for inspection, and checks the output is
// a well-formed multi-page PDF with the fonts, images, and text it should have.
func TestGenerateAppointmentPDF(t *testing.T) {
	pdf, info := renderAppointment(t)

	outPath := filepath.Join(packageDir(), "appointment.pdf")
	if err := os.WriteFile(outPath, pdf, 0o644); err != nil {
		t.Fatalf("write pdf: %v", err)
	}
	t.Logf("wrote %s — %d page(s), %d KB", outPath, info.PageCount, len(pdf)/1024)

	// The letter runs to several pages.
	if info.PageCount < 2 {
		t.Errorf("PageCount = %d, want >= 2", info.PageCount)
	}
	// Well-formed PDF envelope.
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Error("output is missing the PDF header")
	}
	if !bytes.Contains(pdf, []byte("%%EOF")) {
		t.Error("output is missing the trailing EOF marker")
	}

	// The custom fonts are embedded (FontFile2). The memoization fix means each
	// face is embedded once for the whole document, not once per text box — guard
	// against that regression with a tight upper bound.
	ff := bytes.Count(pdf, []byte("/FontFile2"))
	if ff == 0 {
		t.Error("expected embedded custom fonts (FontFile2)")
	}
	if ff > 8 {
		t.Errorf("FontFile2 count = %d — fonts are not being deduped (expected ~4)", ff)
	}
	// Sanity: a 7-page letter with 4 embedded faces should be well under a MB.
	if len(pdf) > 2<<20 {
		t.Errorf("PDF is %d bytes (> 2 MiB) — fonts/images likely duplicated", len(pdf))
	}

	// The letterhead and signature images are present.
	if n := bytes.Count(pdf, []byte("/Subtype /Image")); n < 2 {
		t.Errorf("expected the letterhead + signature images, found %d image XObjects", n)
	}

	// Inflate the content streams and confirm real text made it in. Bold spans
	// are inline runs, which feast emits one word per Tj, so multi-word emphasised
	// phrases are matched by a single word (e.g. "Operations" from the designation).
	content := inflateStreams(t, pdf)
	for _, want := range []string{
		"(APPOINTMENT LETTER)", // title (plain Text)
		"Ravi Kumar",           // filled-in candidate (plain Text)
		"Bengaluru",            // filled-in address (plain Text)
		"Operations",           // filled-in designation (word of a bold run)
		"Compensation Details", // Annexure A heading
	} {
		if !strings.Contains(content, want) {
			t.Errorf("rendered text is missing %q", want)
		}
	}
}

// TestAppointmentValidatesWithPDFCPU strict-validates the generated PDF with
// pdfcpu, if it is installed. Skipped otherwise.
func TestAppointmentValidatesWithPDFCPU(t *testing.T) {
	bin := findPDFCPU()
	if bin == "" {
		t.Skip("pdfcpu not on PATH or in GOPATH/bin; skipping validation")
	}
	pdf, _ := renderAppointment(t)
	p := filepath.Join(t.TempDir(), "appointment.pdf")
	if err := os.WriteFile(p, pdf, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput(); err != nil {
		t.Fatalf("pdfcpu strict validation failed: %v\n%s", err, out)
	}
}

// inflateStreams returns the concatenated inflated content of every FlateDecode
// stream in the PDF (page content, so text-showing operators are searchable).
func inflateStreams(t *testing.T, pdf []byte) string {
	t.Helper()
	var out strings.Builder
	marker := []byte("stream")
	rest := pdf
	for {
		i := bytes.Index(rest, marker)
		if i < 0 {
			break
		}
		start := i + len(marker)
		// Skip the EOL after "stream" (\r\n or \n).
		if start < len(rest) && rest[start] == '\r' {
			start++
		}
		if start < len(rest) && rest[start] == '\n' {
			start++
		}
		end := bytes.Index(rest[start:], []byte("endstream"))
		if end < 0 {
			break
		}
		data := rest[start : start+end]
		if zr, err := zlib.NewReader(bytes.NewReader(data)); err == nil {
			if b, err := io.ReadAll(zr); err == nil {
				out.Write(b)
			}
			zr.Close()
		}
		rest = rest[start+end+len("endstream"):]
	}
	return out.String()
}

// findPDFCPU locates the pdfcpu binary on PATH or in GOPATH/bin.
func findPDFCPU() string {
	if p, err := exec.LookPath("pdfcpu"); err == nil {
		return p
	}
	if gp := os.Getenv("GOPATH"); gp != "" {
		cand := filepath.Join(gp, "bin", "pdfcpu")
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		cand := filepath.Join(home, "go", "bin", "pdfcpu")
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	return ""
}
