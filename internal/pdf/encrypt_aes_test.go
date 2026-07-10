package pdf

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// AES-128 encryption emits a /V 4 /R 4 handler with an AESV2 crypt filter.
func TestEncryptAESStructure(t *testing.T) {
	opts := helloOpts()
	opts.UserPassword = "secret"
	opts.EncryptAES = true
	_, enc := buildHelloWith(t, opts)
	s := string(enc)
	for _, want := range []string{"/V 4", "/R 4", "/CFM /AESV2", "/StmF /StdCF", "/StrF /StdCF", "/Filter /Standard"} {
		if !strings.Contains(s, want) {
			t.Errorf("AES-encrypted PDF missing %q", want)
		}
	}
}

// pkcs7Pad always adds 1..blockSize bytes (a full block when already aligned).
func TestPKCS7Pad(t *testing.T) {
	if got := pkcs7Pad([]byte("abc"), 16); len(got) != 16 || got[15] != 13 {
		t.Errorf("pad of 3 -> len %d last %d, want 16 and 13", len(got), got[len(got)-1])
	}
	if got := pkcs7Pad(make([]byte, 16), 16); len(got) != 32 || got[31] != 16 {
		t.Errorf("aligned input should gain a full padding block: len %d last %d", len(got), got[len(got)-1])
	}
}

// pdfcpu decrypts and strict-validates the AES file given the user password.
func TestEncryptAESValidatesWithPDFCPU(t *testing.T) {
	bin := findPDFCPU()
	if bin == "" {
		t.Skip("pdfcpu not found")
	}
	opts := helloOpts()
	opts.UserPassword = "secret"
	opts.EncryptAES = true
	_, enc := buildHelloWith(t, opts)

	p := filepath.Join(t.TempDir(), "hello-aes.pdf")
	if err := os.WriteFile(p, enc, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bin, "validate", "-m", "strict", "--upw", "secret", p).CombinedOutput(); err != nil {
		t.Fatalf("pdfcpu validate (AES) failed: %v\n%s", err, out)
	}
	// Decrypt round-trip: the decrypted file should itself validate.
	dec := filepath.Join(t.TempDir(), "hello-dec.pdf")
	if out, err := exec.Command(bin, "decrypt", "--upw", "secret", p, dec).CombinedOutput(); err != nil {
		t.Fatalf("pdfcpu decrypt (AES) failed: %v\n%s", err, out)
	}
}
