package pdf

import (
	"bytes"
	"compress/zlib"
	"io"
	"testing"
)

// flateProgram compresses once, returns the same bytes on every call, and its
// output is byte-identical to FlateStream's — so embedding stays deterministic
// (goldens) while the compression cost is paid once per face, not per document.
func TestEmbeddedFontFlateProgramMemoized(t *testing.T) {
	ef := &EmbeddedFont{
		Name:    "Test",
		Program: bytes.Repeat([]byte("waffle-font-bytes "), 4096),
		Widths:  make([]int, 256),
	}

	a := ef.flateProgram()
	b := ef.flateProgram()
	if len(a) == 0 || &a[0] != &b[0] {
		t.Error("flateProgram should compress once and return the cached bytes")
	}

	// Byte-identical to the uncached FlateStream path.
	if fs := FlateStream(nil, ef.Program); !bytes.Equal(fs.Data, a) {
		t.Error("memoized compression must match FlateStream output exactly")
	}

	// And it round-trips back to the original program.
	zr, err := zlib.NewReader(bytes.NewReader(a))
	if err != nil {
		t.Fatalf("zlib reader: %v", err)
	}
	defer zr.Close()
	got, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("inflate: %v", err)
	}
	if !bytes.Equal(got, ef.Program) {
		t.Error("compressed program does not round-trip to the original bytes")
	}
}
