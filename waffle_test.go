package waffle

import (
	"bytes"
	"context"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const boxesDoc = `{"version":"waffle-tree/v1","document":{"props":{"title":"Boxes"},"children":[
	{"type":"PAGE","props":{"size":[240,180],"style":{"padding":20,"backgroundColor":"#eeeeee"}},"children":[
		{"type":"VIEW","props":{"style":{"flexDirection":"row","gap":10,"height":60}},"children":[
			{"type":"VIEW","props":{"style":{"flexGrow":1,"backgroundColor":"#e63946","border":"2pt solid #1d3557"}}},
			{"type":"VIEW","props":{"style":{"flexGrow":2,"backgroundColor":"#457b9d"}}}
		]},
		{"type":"VIEW","props":{"style":{"marginTop":10,"height":40,"backgroundColor":"#a8dadc","borderRadius":4}}}
	]}
]}}`

func TestRenderTreeEndToEnd(t *testing.T) {
	var buf bytes.Buffer
	info, err := RenderTree(context.Background(), []byte(boxesDoc), &buf)
	if err != nil {
		t.Fatalf("RenderTree: %v", err)
	}
	if info.PageCount != 1 {
		t.Errorf("PageCount = %d, want 1", info.PageCount)
	}
	if len(info.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", info.Warnings)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Fatalf("output is not a PDF")
	}

	if bin := findPDFCPU(); bin != "" {
		p := filepath.Join(t.TempDir(), "boxes.pdf")
		if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(bin, "validate", "-m", "strict", p).CombinedOutput(); err != nil {
			t.Fatalf("pdfcpu validate failed: %v\n%s", err, out)
		}
	} else {
		t.Log("pdfcpu not found; skipping validation")
	}
}

func TestRenderTreeCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RenderTree(ctx, []byte(boxesDoc), &bytes.Buffer{}); err == nil {
		t.Errorf("expected error for canceled context")
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

// A Document with a userPassword must produce an encrypted, still-valid PDF.
func TestRenderTreeEncrypted(t *testing.T) {
	doc := `{"version":"waffle-tree/v1","document":{"props":{"userPassword":"secret"},"children":[
		{"type":"PAGE","props":{"size":[200,200]},"children":[
			{"type":"TEXT","props":{"style":{"fontSize":14}},"children":[{"type":"TEXT_INSTANCE","value":"locked"}]}
		]}
	]}}`
	var buf bytes.Buffer
	if _, err := RenderTree(context.Background(), []byte(doc), &buf); err != nil {
		t.Fatalf("RenderTree: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("/Encrypt")) || !bytes.Contains(buf.Bytes(), []byte("/Filter /Standard")) {
		t.Fatal("expected an /Encrypt dictionary (standard security handler)")
	}
	if bin := findPDFCPU(); bin != "" {
		p := filepath.Join(t.TempDir(), "enc.pdf")
		if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		// pdfcpu (pflag) needs the long form; validates only with the password.
		if out, err := exec.Command(bin, "validate", "-m", "strict", "--upw", "secret", p).CombinedOutput(); err != nil {
			t.Fatalf("pdfcpu validate (with password) failed: %v\n%s", err, out)
		}
	}
}
