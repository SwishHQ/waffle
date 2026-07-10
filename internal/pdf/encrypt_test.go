package pdf

import (
	"bytes"
	"compress/zlib"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// helloOpts returns the deterministic metadata used by buildHello (pdf_test.go).
func helloOpts() Options {
	return Options{
		Title:        "Hello",
		Creator:      "waffle",
		Producer:     "waffle",
		PDFVersion:   "1.4",
		CreationDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

// buildHelloWith builds the canonical single-page "Hello, World!" document with
// the given options and returns the finalized Document alongside the bytes.
func buildHelloWith(t *testing.T, opts Options) (*Document, []byte) {
	t.Helper()
	const a4W, a4H = 595.28, 841.89

	c := NewContent()
	c.BeginText().
		SetFont("Helvetica", 24).
		TextPosition(72, a4H-72).
		ShowText("Hello, World!").
		EndText()

	doc := New(opts)
	doc.AddPage(a4W, a4H, c)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	return doc, buf.Bytes()
}

// firstStream returns the object number and raw data bytes of the first stream
// in a serialized PDF (the only stream in the hello document: page content).
func firstStream(t *testing.T, data []byte) (num int, stream []byte) {
	t.Helper()
	i := bytes.Index(data, []byte("\nstream\n"))
	if i < 0 {
		t.Fatal("no stream found")
	}
	start := i + len("\nstream\n")
	end := bytes.Index(data[start:], []byte("\nendstream"))
	if end < 0 {
		t.Fatal("no endstream found")
	}
	hdr := bytes.LastIndex(data[:i], []byte(" 0 obj"))
	if hdr < 0 {
		t.Fatal("no object header before stream")
	}
	mul := 1
	for j := hdr; j > 0; j-- {
		c := data[j-1]
		if c < '0' || c > '9' {
			break
		}
		num += int(c-'0') * mul
		mul *= 10
	}
	if num == 0 {
		t.Fatal("could not parse stream object number")
	}
	return num, data[start : start+end]
}

func TestEncryptStructure(t *testing.T) {
	optsEnc := helloOpts()
	optsEnc.UserPassword = "secret"
	_, enc := buildHelloWith(t, optsEnc)
	_, plain := buildHelloWith(t, helloOpts())

	for _, want := range []string{"/Encrypt", "/Filter /Standard", "/V 2", "/R 3", "/Length 128", "/P -4"} {
		if !bytes.Contains(enc, []byte(want)) {
			t.Errorf("encrypted output missing %q", want)
		}
	}

	plainNum, plainStream := firstStream(t, plain)
	encNum, encStream := firstStream(t, enc)
	if plainNum != encNum {
		t.Fatalf("content stream object number changed: plain %d, encrypted %d", plainNum, encNum)
	}
	if bytes.Contains(enc, plainStream) {
		t.Error("encrypted file still contains the plaintext content-stream bytes")
	}
	if len(encStream) != len(plainStream) {
		t.Errorf("RC4 must preserve length: encrypted %d bytes, plain %d bytes", len(encStream), len(plainStream))
	}
	if bytes.Equal(encStream, plainStream) {
		t.Error("content-stream bytes unchanged by encryption")
	}
}

// TestEncryptRoundTrip decrypts the content stream with the derived per-object
// key and checks that inflating it recovers the original page content.
func TestEncryptRoundTrip(t *testing.T) {
	opts := helloOpts()
	opts.UserPassword = "secret"
	opts.OwnerPassword = "hunter2"
	doc, enc := buildHelloWith(t, opts)
	if doc.w.encrypt == nil {
		t.Fatal("writer has no encryptor after WriteTo")
	}

	num, cipher := firstStream(t, enc)
	stream := rc4Apply(doc.w.encrypt.objectKey(num, 0), cipher)
	zr, err := zlib.NewReader(bytes.NewReader(stream))
	if err != nil {
		t.Fatalf("decrypted stream is not valid zlib: %v", err)
	}
	content, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("inflate decrypted stream: %v", err)
	}
	if !bytes.Contains(content, []byte("Hello, World!")) {
		t.Errorf("decrypted content stream missing text, got: %q", content)
	}
}

func TestEncryptDeterministic(t *testing.T) {
	opts := helloOpts()
	opts.UserPassword = "secret"
	_, a := buildHelloWith(t, opts)
	_, b := buildHelloWith(t, opts)
	if !bytes.Equal(a, b) {
		t.Fatalf("encrypted output not deterministic: %d vs %d bytes", len(a), len(b))
	}
}

func TestEncryptValidatesWithPDFCPU(t *testing.T) {
	bin := findPDFCPU()
	if bin == "" {
		t.Log("pdfcpu not found on PATH or in GOPATH/bin")
		t.Skip("skipping pdfcpu validation")
	}
	opts := helloOpts()
	opts.UserPassword = "secret"
	_, enc := buildHelloWith(t, opts)

	p := filepath.Join(t.TempDir(), "hello-enc.pdf")
	if err := os.WriteFile(p, enc, 0o644); err != nil {
		t.Fatal(err)
	}
	// Double-dash flags parse with both older (stdlib flag) and newer
	// (pflag) pdfcpu CLIs; pflag rejects -upw as bundled shorthands.
	out, err := exec.Command(bin, "validate", "-m", "strict", "--upw", "secret", p).CombinedOutput()
	if err != nil {
		t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
	}
}

func TestEncryptOwnerPasswordOnlyValidatesWithPDFCPU(t *testing.T) {
	bin := findPDFCPU()
	if bin == "" {
		t.Log("pdfcpu not found on PATH or in GOPATH/bin")
		t.Skip("skipping pdfcpu validation")
	}
	opts := helloOpts()
	opts.OwnerPassword = "hunter2"
	_, enc := buildHelloWith(t, opts)

	p := filepath.Join(t.TempDir(), "hello-opw.pdf")
	if err := os.WriteFile(p, enc, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(bin, "validate", "-m", "strict", "--opw", "hunter2", p).CombinedOutput()
	if err != nil {
		t.Fatalf("pdfcpu validate with owner password failed: %v\n%s", err, out)
	}
	// The user password is empty, so the file must also open with no password.
	out, err = exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput()
	if err != nil {
		t.Fatalf("pdfcpu validate without password failed (empty user password): %v\n%s", err, out)
	}
}

// TestNoPasswordsNoEncrypt pins down that encryption is fully inert when no
// password is set: no /Encrypt anywhere, and output byte-identical to the
// golden-covered buildHello document.
func TestNoPasswordsNoEncrypt(t *testing.T) {
	_, data := buildHelloWith(t, helloOpts())
	if bytes.Contains(data, []byte("/Encrypt")) {
		t.Error("unencrypted output contains /Encrypt")
	}
	if !bytes.Equal(data, buildHello(t)) {
		t.Error("output differs from buildHello twin; encryption changed the no-password path")
	}
}
