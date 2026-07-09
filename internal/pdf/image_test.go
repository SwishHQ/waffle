package pdf

import (
	"bytes"
	"compress/zlib"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func flate(data []byte) []byte {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	_, _ = zw.Write(data)
	_ = zw.Close()
	return buf.Bytes()
}

func TestImageXObjectValidates(t *testing.T) {
	// 2x2 RGB image with a 2x2 alpha SMask.
	rgb := []byte{255, 0, 0, 0, 255, 0, 0, 0, 255, 255, 255, 0}
	alpha := []byte{255, 128, 64, 0}
	spec := &ImageSpec{
		Width: 2, Height: 2, ColorSpace: "DeviceRGB", BitsPerComponent: 8,
		Filter: "FlateDecode", Data: flate(rgb), SMask: flate(alpha),
	}

	c := NewContent()
	c.DrawImage(spec, 10, 10, 100, 100)
	doc := New(Options{Producer: "feast", CreationDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
	doc.AddPage(200, 200, c)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	for _, want := range []string{"/Subtype /Image", "/Width 2", "/Height 2", "/SMask", "/XObject"} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("output missing %q", want)
		}
	}

	if bin := findPDFCPU(); bin != "" {
		p := filepath.Join(t.TempDir(), "img.pdf")
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput(); err != nil {
			t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
		}
	}
}
