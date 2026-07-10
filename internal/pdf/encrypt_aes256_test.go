package pdf

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// AES-256 encryption emits a /V 5 /R 6 handler with an AESV3 crypt filter and
// the R6 wrapping entries.
func TestEncryptAES256Structure(t *testing.T) {
	opts := helloOpts()
	opts.UserPassword = "secret"
	opts.EncryptAES256 = true
	_, enc := buildHelloWith(t, opts)
	s := string(enc)
	for _, want := range []string{"/V 5", "/R 6", "/CFM /AESV3", "/UE", "/OE", "/Perms", "/StmF /StdCF"} {
		if !strings.Contains(s, want) {
			t.Errorf("AES-256 PDF missing %q", want)
		}
	}
}

// hash2B is stable for fixed inputs (no randomness inside the hash itself).
func TestHash2BDeterministic(t *testing.T) {
	a := hash2B([]byte("pw"), []byte("saltsalt"), nil)
	b := hash2B([]byte("pw"), []byte("saltsalt"), nil)
	if len(a) != 32 || string(a) != string(b) {
		t.Errorf("hash2B not stable/32 bytes: %d, equal=%v", len(a), string(a) == string(b))
	}
	if c := hash2B([]byte("pw2"), []byte("saltsalt"), nil); string(c) == string(a) {
		t.Error("hash2B should differ for a different password")
	}
}

// pdfcpu recomputes the R6 key from the password to decrypt — a full correctness
// oracle for the key derivation and object encryption.
func TestEncryptAES256ValidatesWithPDFCPU(t *testing.T) {
	bin := findPDFCPU()
	if bin == "" {
		t.Skip("pdfcpu not found")
	}
	opts := helloOpts()
	opts.UserPassword = "secret"
	opts.EncryptAES256 = true
	_, enc := buildHelloWith(t, opts)

	p := filepath.Join(t.TempDir(), "hello-aes256.pdf")
	if err := os.WriteFile(p, enc, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bin, "validate", "-m", "strict", "--upw", "secret", p).CombinedOutput(); err != nil {
		t.Fatalf("pdfcpu validate (AES-256) failed: %v\n%s", err, out)
	}
	dec := filepath.Join(t.TempDir(), "hello-aes256-dec.pdf")
	if out, err := exec.Command(bin, "decrypt", "--upw", "secret", p, dec).CombinedOutput(); err != nil {
		t.Fatalf("pdfcpu decrypt (AES-256) failed: %v\n%s", err, out)
	}
}
