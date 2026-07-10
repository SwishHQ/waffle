package layout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/swish/feast/internal/contract"
	"github.com/swish/feast/internal/fontstore"
	"golang.org/x/image/font/gofont/goregular"
)

// A Font.register whose src is a local file path is fetched, parsed, and resolves.
func TestBuildFontStoreFromFilePath(t *testing.T) {
	p := filepath.Join(t.TempDir(), "go.ttf")
	if err := os.WriteFile(p, goregular.TTF, 0o644); err != nil {
		t.Fatal(err)
	}
	store, warns := buildFontStore([]contract.FontRegistration{
		{Family: "GoFile", Faces: []contract.FontFace{{Src: p}}},
	})
	if store == nil {
		t.Fatalf("store should be built from a file-path font; warns=%v", warns)
	}
	if _, ok := store.ResolveFace("GoFile", 400, fontstore.StyleNormal); !ok {
		t.Error("file-path registered font should resolve")
	}
}

// A missing file path degrades gracefully (warning, no crash, no store).
func TestBuildFontStoreMissingFile(t *testing.T) {
	store, warns := buildFontStore([]contract.FontRegistration{
		{Family: "Ghost", Faces: []contract.FontFace{{Src: "/no/such/font.ttf"}}},
	})
	if store != nil {
		t.Error("a missing font file should not produce a store")
	}
	if len(warns) == 0 {
		t.Error("a missing font file should emit a warning")
	}
}
